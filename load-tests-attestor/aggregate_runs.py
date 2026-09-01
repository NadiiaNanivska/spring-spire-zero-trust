#!/usr/bin/env python3
"""Aggregate N independent run-all.sh results into statistically valid summaries.

Each run directory is one statistical replicate.  Per (run, scenario, overlay,
metric) a single scalar is computed with the same estimators as plot_results.py,
then descriptive statistics (mean, std, median, 95% t-CI) are computed across runs.

Outputs (under --out-dir, default results/aggregate-<timestamp>/):
  run_scalars.csv    raw per-run scalars
  summary_stats.csv  mean ± 95% CI across runs
  results_tables.html  tables for thesis (Word copy-paste)
  long_all.csv       raw time series from all runs (column ``run``)

Usage:
  python aggregate_runs.py results/run-ts1 results/run-ts2 results/run-ts3
  python aggregate_runs.py --glob 'results/run-*' --last 3
"""
from __future__ import annotations

import argparse
import csv
import glob
import math
import os
import re
import statistics
import sys
from datetime import datetime, timezone

# Reuse statistically correct estimators from plot_results.py
from plot_results import (
    attestation_weighted_ms,
    collect_long_rows,
    discover_runs,
    nanmean,
    parse_matrix,
    rate_mean_full_window,
    read_window_seconds,
    service_means,
)

# 95% two-tailed t critical values (df = n-1), df 1..30; df>30 -> 1.96
T_CRITICAL_975: dict[int, float] = {
    1: 12.706,
    2: 4.303,
    3: 3.182,
    4: 2.776,
    5: 2.571,
    6: 2.447,
    7: 2.365,
    8: 2.306,
    9: 2.262,
    10: 2.228,
    11: 2.201,
    12: 2.179,
    13: 2.160,
    14: 2.145,
    15: 2.131,
    16: 2.120,
    17: 2.110,
    18: 2.101,
    19: 2.093,
    20: 2.086,
    21: 2.080,
    22: 2.074,
    23: 2.069,
    24: 2.064,
    25: 2.060,
    26: 2.056,
    27: 2.052,
    28: 2.048,
    29: 2.045,
    30: 2.042,
}

# Metrics reported per scenario (thesis tables)
SCENARIOS_CONFIG: dict[str, dict] = {
    "a": {
        "title": 'Таблиця 4.1 — Результати для Сценарію A: Масштабування з "теплим" кешем',
        "metrics": ["attestation_avg_ms", "agent_cpu", "agent_memory_mb"],
    },
    "b": {
        "title": 'Таблиця 4.2 — Результати для Сценарію B: "Холодний" старт та перерахунок цілісності',
        "metrics": [
            "attestation_avg_ms",
            "agent_cpu",
            "agent_memory_mb",
            "server_cpu",
        ],
    },
    "c": {
        "title": 'Таблиця 4.3 — Результати для Сценарію C: Стабільне навантаження та "шторм" переатестації',
        "metrics": [
            "http_req_p95_ms",
            "http_req_p99_ms",
            "http_5xx_rate",
            "process_cpu",
            "jvm_heap_bytes",
            "jvm_gc_pause_rate",
        ],
    },
}

# Per-service metrics: one scalar row per service label (never blend services).
PER_SERVICE_METRICS = frozenset({"http_req_p95_ms", "http_req_p99_ms", "jvm_heap_bytes"})

# Agent metrics: mean over window but only from pods active in the last 30 s.
ACTIVE_POD_METRICS = frozenset({"agent_cpu", "agent_memory_mb"})

RUN_DIR_BASENAME_RE = re.compile(r"run-(\d{8}-\d{6})")


def t_critical_975(n: int) -> float:
    """Two-tailed 95% t critical value for sample size n."""
    if n < 2:
        return float("nan")
    df = n - 1
    return T_CRITICAL_975.get(df, 1.96)


def run_label(run_dir: str) -> str:
    """Stable short label for a run directory (e.g. run-20260721-160830)."""
    base = os.path.basename(os.path.abspath(run_dir))
    if RUN_DIR_BASENAME_RE.match(base):
        return base
    return base


def active_pod_mean(subdir: str, metric: str) -> float:
    """Mean over the window using only agent pods active in the last 30 s."""
    path = os.path.join(subdir, "prometheus", f"{metric}.json")
    series = parse_matrix(path)
    if not series:
        return float("nan")

    per_series: list[tuple[float, list[float]]] = []
    for _labels, points in series:
        if not points:
            continue
        t0 = points[0][0]
        max_elapsed = max(ts - t0 for ts, _ in points)
        vals = [v for _, v in points if v == v]
        if vals:
            per_series.append((max_elapsed, vals))

    if not per_series:
        return float("nan")

    global_max = max(el for el, _ in per_series)
    active_vals: list[float] = []
    for max_elapsed, vals in per_series:
        if max_elapsed >= global_max - 30:
            active_vals.extend(vals)
    return nanmean(active_vals)


def gauge_mean(subdir: str, metric: str) -> float:
    """Simple mean over all samples in the window (gauges / rates without special handling)."""
    path = os.path.join(subdir, "prometheus", f"{metric}.json")
    return nanmean([v for _, points in parse_matrix(path) for _, v in points])


def scalar_for_metric(subdir: str, metric: str) -> float:
    """Single-window scalar for a non per-service metric."""
    prom = os.path.join(subdir, "prometheus")
    if metric == "attestation_avg_ms":
        return attestation_weighted_ms(subdir)
    if metric == "http_5xx_rate":
        return rate_mean_full_window(
            os.path.join(prom, "http_5xx_rate.json"),
            read_window_seconds(subdir),
        )
    if metric in ACTIVE_POD_METRICS:
        return active_pod_mean(subdir, metric)
    return gauge_mean(subdir, metric)


def scalars_per_service(subdir: str, metric: str) -> dict[str, float]:
    """Return {service_label: scalar} for per-service metrics."""
    path = os.path.join(subdir, "prometheus", f"{metric}.json")
    means = service_means(path)
    if metric == "jvm_heap_bytes":
        means = {svc: v / (1024 * 1024) for svc, v in means.items()}
    return means


def collect_run_scalars(run_dir: str) -> list[dict]:
    """One scalar per (run, scenario, overlay, metric[, series])."""
    run_id = run_label(run_dir)
    runs = discover_runs(run_dir)
    rows: list[dict] = []

    for scenario, overlays in sorted(runs.items()):
        cfg = SCENARIOS_CONFIG.get(scenario)
        if not cfg:
            continue
        for metric in cfg["metrics"]:
            for overlay, subdir in sorted(overlays.items()):
                if metric in PER_SERVICE_METRICS:
                    for series, val in sorted(scalars_per_service(subdir, metric).items()):
                        if val == val:
                            rows.append({
                                "run": run_id,
                                "scenario": scenario,
                                "overlay": overlay,
                                "metric": metric,
                                "series": series,
                                "value": val,
                            })
                else:
                    val = scalar_for_metric(subdir, metric)
                    if val == val:
                        rows.append({
                            "run": run_id,
                            "scenario": scenario,
                            "overlay": overlay,
                            "metric": metric,
                            "series": "",
                            "value": val,
                        })
    return rows


def write_run_scalars_csv(rows: list[dict], out_path: str) -> None:
    fields = ["run", "scenario", "overlay", "metric", "series", "value"]
    with open(out_path, "w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=fields)
        writer.writeheader()
        writer.writerows(rows)


def compute_summary_stats(scalar_rows: list[dict]) -> list[dict]:
    """Group scalars by (scenario, overlay, metric, series) -> descriptive stats."""
    groups: dict[tuple, list[float]] = {}
    for row in scalar_rows:
        key = (row["scenario"], row["overlay"], row["metric"], row["series"])
        groups.setdefault(key, []).append(float(row["value"]))

    summary: list[dict] = []
    for (scenario, overlay, metric, series), values in sorted(groups.items()):
        n = len(values)
        if n == 0:
            continue
        mean = statistics.mean(values)
        std = statistics.stdev(values) if n > 1 else 0.0
        median = statistics.median(values)
        t = t_critical_975(n)
        ci_half = t * std / math.sqrt(n) if n > 1 and t == t else float("nan")
        summary.append({
            "scenario": scenario,
            "overlay": overlay,
            "metric": metric,
            "series": series,
            "n": n,
            "mean": mean,
            "std": std,
            "ci95_halfwidth": ci_half,
            "ci95_low": mean - ci_half if ci_half == ci_half else float("nan"),
            "ci95_high": mean + ci_half if ci_half == ci_half else float("nan"),
            "median": median,
            "min": min(values),
            "max": max(values),
        })
    return summary


def write_summary_stats_csv(rows: list[dict], out_path: str) -> None:
    fields = [
        "scenario", "overlay", "metric", "series",
        "n", "mean", "std", "ci95_halfwidth", "ci95_low", "ci95_high",
        "median", "min", "max",
    ]
    with open(out_path, "w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=fields)
        writer.writeheader()
        for row in rows:
            writer.writerow(row)


def generate_combined_html_table(rows: list[dict], columns: list[str], title: str) -> str:
    """HTML table for thesis copy-paste (Times New Roman)."""
    colspan = len(columns)
    html = (
        "<table border='1' style='border-collapse: collapse; width: 100%; "
        "font-family: \"Times New Roman\", serif;'>\n"
    )
    html += f"  <tr>\n    <td colspan='{colspan}' "
    html += f"style='padding: 5px; font-weight: bold;'>{title}</td>\n  </tr>\n"
    html += "  <tr>\n"
    for col in columns:
        html += f"    <th style='padding: 5px; text-align: center;'>{col}</th>\n"
    html += "  </tr>\n"

    current_metric = ""
    for row in rows:
        html += "  <tr>\n"
        for col in columns:
            val = row.get(col)
            if col == "Метрика":
                if val == current_metric:
                    val_str = ""
                else:
                    val_str = f"<b>{val}</b>"
                    current_metric = val
            elif val is None or (isinstance(val, float) and val != val):
                val_str = "-"
            elif col == "N (ранів)":
                val_str = f"{int(val)}"
            elif isinstance(val, (int, float)):
                val_str = f"{val:.3f}"
            else:
                val_str = str(val)
            align = "left" if col in ("Метрика", "Плагін") else "center"
            html += f"    <td style='text-align: {align}; padding: 5px;'>{val_str}</td>\n"
        html += "  </tr>\n"
    html += "</table><br><br>\n"
    return html


def metric_display_name(metric: str, series: str) -> str:
    if series:
        return f"{metric} [{series}]"
    return metric


def build_html_tables(summary_rows: list[dict]) -> str:
    """Build scenario tables from summary_stats rows."""
    if not summary_rows:
        return "<p>No summary data.</p>\n"

    columns = [
        "Метрика", "Плагін", "N (ранів)", "Середнє (μ)",
        "±95% CI", "Медіана", "Макс",
    ]
    html_output = ""

    for sc_key, config in SCENARIOS_CONFIG.items():
        sc_data = [r for r in summary_rows if r["scenario"] == sc_key]
        if not sc_data:
            continue

        table_rows: list[dict] = []
        for metric in config["metrics"]:
            metric_data = [r for r in sc_data if r["metric"] == metric]
            for row in metric_data:
                ci = row["ci95_halfwidth"]
                table_rows.append({
                    "Метрика": metric_display_name(metric, row["series"]),
                    "Плагін": row["overlay"],
                    "N (ранів)": int(row["n"]),
                    "Середнє (μ)": row["mean"],
                    "±95% CI": ci if ci == ci else None,
                    "Медіана": row["median"],
                    "Макс": row["max"],
                })

        if table_rows:
            html_output += generate_combined_html_table(
                table_rows, columns, config["title"],
            )

    return html_output


def collect_long_all(run_dirs: list[str]) -> list[dict]:
    """Merge collect_long_rows from each run with a ``run`` column."""
    all_rows: list[dict] = []
    for run_dir in run_dirs:
        run_id = run_label(run_dir)
        runs = discover_runs(run_dir)
        for row in collect_long_rows(runs):
            row["run"] = run_id
            all_rows.append(row)
    return all_rows


def write_long_all_csv(rows: list[dict], out_path: str) -> None:
    fields = ["run", "scenario", "overlay", "metric", "series", "elapsed_s", "value"]
    with open(out_path, "w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=fields)
        writer.writeheader()
        writer.writerows(rows)


def resolve_run_dirs(args: argparse.Namespace) -> list[str]:
    dirs: list[str] = []
    if args.run_dirs:
        dirs.extend(args.run_dirs)
    if args.glob_pattern:
        matched = sorted(glob.glob(args.glob_pattern))
        dirs.extend(matched)
    dirs = [os.path.abspath(d) for d in dirs]
    # Deduplicate preserving order
    seen: set[str] = set()
    unique: list[str] = []
    for d in dirs:
        if d not in seen:
            seen.add(d)
            unique.append(d)
    if args.last and args.last > 0:
        unique = sorted(unique)[-args.last :]
    return unique


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Aggregate multiple load-test runs into statistically valid summaries.",
    )
    parser.add_argument(
        "run_dirs", nargs="*", help="Run directories (e.g. results/run-<ts>)",
    )
    parser.add_argument(
        "--glob", dest="glob_pattern", default="",
        help="Glob for run dirs (e.g. 'results/run-*')",
    )
    parser.add_argument(
        "--last", type=int, default=0,
        help="After glob, keep only the N most recent directories",
    )
    parser.add_argument(
        "--out-dir", default="",
        help="Output directory (default: results/aggregate-<utc-ts>)",
    )
    args = parser.parse_args()

    run_dirs = resolve_run_dirs(args)
    if not run_dirs:
        sys.stderr.write(
            "ERROR: no run directories. Pass paths or --glob 'results/run-*'\n",
        )
        return 1

    for d in run_dirs:
        if not os.path.isdir(d):
            sys.stderr.write(f"ERROR: not a directory: {d}\n")
            return 1
        if not discover_runs(d):
            sys.stderr.write(
                f"ERROR: no scenario subdirs in {d}\n",
            )
            return 1

    lib_dir = os.path.dirname(os.path.abspath(__file__))
    if args.out_dir:
        out_dir = os.path.abspath(args.out_dir)
    else:
        ts = datetime.now(timezone.utc).strftime("%Y%m%d-%H%M%S")
        out_dir = os.path.join(lib_dir, "results", f"aggregate-{ts}")
    os.makedirs(out_dir, exist_ok=True)

    scalar_rows: list[dict] = []
    for run_dir in run_dirs:
        scalar_rows.extend(collect_run_scalars(run_dir))

    if not scalar_rows:
        sys.stderr.write("ERROR: no scalar data collected from runs\n")
        return 1

    run_scalars_path = os.path.join(out_dir, "run_scalars.csv")
    write_run_scalars_csv(scalar_rows, run_scalars_path)

    summary_rows = compute_summary_stats(scalar_rows)
    summary_path = os.path.join(out_dir, "summary_stats.csv")
    write_summary_stats_csv(summary_rows, summary_path)

    html_path = os.path.join(out_dir, "results_tables.html")
    with open(html_path, "w", encoding="utf-8") as fh:
        fh.write(build_html_tables(summary_rows))

    long_rows = collect_long_all(run_dirs)
    long_all_path = os.path.join(out_dir, "long_all.csv")
    write_long_all_csv(long_rows, long_all_path)

    # Record which runs were aggregated
    manifest_path = os.path.join(out_dir, "runs.txt")
    with open(manifest_path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(run_dirs) + "\n")

    n_runs = len({r["run"] for r in scalar_rows})
    print(f"Runs aggregated : {n_runs} ({', '.join(run_label(d) for d in run_dirs)})")
    print(f"Output directory: {out_dir}")
    print(f"  run_scalars.csv   ({len(scalar_rows)} rows)")
    print(f"  summary_stats.csv ({len(summary_rows)} rows)")
    print(f"  results_tables.html")
    print(f"  long_all.csv      ({len(long_rows)} rows)")
    print(f"  runs.txt")
    return 0


if __name__ == "__main__":
    sys.exit(main())
