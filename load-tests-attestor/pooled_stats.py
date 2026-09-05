#!/usr/bin/env python3
"""Generate pooled-sample statistics and Mann-Whitney comparisons."""
from __future__ import annotations

import argparse
import csv
import math
import os
import sys

import pandas as pd

from aggregate_runs import PER_SERVICE_METRICS, SCENARIOS_CONFIG, run_label
from plot_results import collect_long_rows, discover_runs

OVERLAYS = ("default", "custom-jvm")


def load_long_df_from_run(run_dir: str) -> pd.DataFrame:
    runs = discover_runs(run_dir)
    rows = collect_long_rows(runs)
    run_id = run_label(run_dir)
    for row in rows:
        row["run"] = run_id
    return pd.DataFrame(rows)


def load_input_df(args: argparse.Namespace) -> tuple[pd.DataFrame, str]:
    if args.run_dir:
        base_dir = os.path.abspath(args.run_dir)
        if not os.path.isdir(base_dir):
            raise FileNotFoundError(f"not a directory: {base_dir}")
        if not discover_runs(base_dir):
            raise FileNotFoundError(
                f"no '<overlay>-scenario-<id>-*' subdirs in {base_dir}",
            )
        return load_long_df_from_run(base_dir), base_dir

    if args.input:
        input_path = os.path.abspath(args.input)
        base_dir = os.path.dirname(input_path)
    else:
        base_dir = os.path.abspath(args.aggregate_dir)
        input_path = os.path.join(base_dir, "long_all.csv")

    if not os.path.isfile(input_path):
        raise FileNotFoundError(f"input not found: {input_path}")

    return pd.read_csv(input_path), base_dir


def normalize_df(df: pd.DataFrame) -> pd.DataFrame:
    required = ["scenario", "overlay", "metric", "value"]
    missing = [c for c in required if c not in df.columns]
    if missing:
        raise ValueError(f"missing required columns: {', '.join(missing)}")

    out = df.copy()
    if "series" not in out.columns:
        out["series"] = ""
    out["series"] = out["series"].fillna("")
    out.loc[~out["metric"].isin(PER_SERVICE_METRICS), "series"] = ""

    out = out[out["overlay"].isin(OVERLAYS)]
    out["value"] = pd.to_numeric(out["value"], errors="coerce")
    out = out.dropna(subset=["value"])

    heap_mask = out["metric"] == "jvm_heap_bytes"
    out.loc[heap_mask, "value"] = out.loc[heap_mask, "value"] / (1024 * 1024)
    return out


def mann_whitney_u_pvalue(x: list[float], y: list[float]) -> tuple[float, float]:
    n1 = len(x)
    n2 = len(y)
    if n1 == 0 or n2 == 0:
        return float("nan"), float("nan")

    pooled = [(float(v), 0) for v in x] + [(float(v), 1) for v in y]
    pooled.sort(key=lambda it: it[0])

    ranks: list[float] = [0.0] * len(pooled)
    tie_counts: list[int] = []
    i = 0
    while i < len(pooled):
        j = i + 1
        while j < len(pooled) and pooled[j][0] == pooled[i][0]:
            j += 1
        avg_rank = (i + 1 + j) / 2.0
        for k in range(i, j):
            ranks[k] = avg_rank
        tie_counts.append(j - i)
        i = j

    r1 = sum(rank for rank, (_, grp) in zip(ranks, pooled) if grp == 0)
    u1 = r1 - n1 * (n1 + 1) / 2.0
    u2 = n1 * n2 - u1
    u = min(u1, u2)

    n = n1 + n2
    if n < 2:
        return u, float("nan")

    tie_term = sum(t ** 3 - t for t in tie_counts)
    var_u = (n1 * n2 / 12.0) * ((n + 1) - tie_term / (n * (n - 1)))
    if var_u <= 0:
        return u, float("nan")

    mean_u = n1 * n2 / 2.0
    z = (abs(u - mean_u) - 0.5) / math.sqrt(var_u)
    p_value = math.erfc(abs(z) / math.sqrt(2.0))
    return u, p_value


def significance_label(p_value: float) -> str:
    if p_value != p_value:
        return ""
    if p_value < 0.001:
        return "0.001"
    if p_value < 0.01:
        return "0.01"
    if p_value < 0.05:
        return "0.05"
    return "ns"


def metric_display_name(metric: str, series: str) -> str:
    return f"{metric} [{series}]" if series else metric


def fmt_num(value: float) -> str:
    if value != value:
        return ""
    return f"{value:.6g}"


def fmt_min_max(min_value: float, max_value: float) -> str:
    return f"{fmt_num(min_value)}-{fmt_num(max_value)}"


def compute_pooled_stats(df: pd.DataFrame) -> tuple[list[dict], list[dict]]:
    grouped_values: dict[tuple[str, str, str, str], list[float]] = {}
    for key, group in df.groupby(["scenario", "metric", "series", "overlay"], sort=True):
        grouped_values[key] = group["value"].astype(float).tolist()

    pooled_rows: list[dict] = []
    for (scenario, metric, series, overlay), values in sorted(grouped_values.items()):
        s = pd.Series(values, dtype=float)
        p25 = float(s.quantile(0.25))
        median = float(s.quantile(0.50))
        p75 = float(s.quantile(0.75))
        pooled_rows.append({
            "scenario": scenario,
            "metric": metric,
            "series": series,
            "overlay": overlay,
            "n": len(values),
            "median": median,
            "p25": p25,
            "p75": p75,
            "iqr": p75 - p25,
            "min": float(min(values)),
            "max": float(max(values)),
        })

    pooled_map = {
        (r["scenario"], r["metric"], r["series"], r["overlay"]): r
        for r in pooled_rows
    }

    base_keys = sorted({(s, m, sr) for (s, m, sr, _ov) in pooled_map.keys()})
    comparison_rows: list[dict] = []
    for scenario, metric, series in base_keys:
        default_stats = pooled_map.get((scenario, metric, series, "default"))
        custom_stats = pooled_map.get((scenario, metric, series, "custom-jvm"))
        default_vals = grouped_values.get((scenario, metric, series, "default"), [])
        custom_vals = grouped_values.get((scenario, metric, series, "custom-jvm"), [])

        _u_value, p_value = mann_whitney_u_pvalue(default_vals, custom_vals)

        default_median = default_stats["median"] if default_stats else float("nan")
        custom_median = custom_stats["median"] if custom_stats else float("nan")
        delta_pct = float("nan")
        if default_median == default_median and custom_median == custom_median and default_median != 0:
            delta_pct = (custom_median - default_median) / default_median * 100.0

        comparison_rows.append({
            "scenario": scenario,
            "metric_name": metric,
            "series_name": series,
            "n_default": int(default_stats["n"]) if default_stats else 0,
            "median_default": default_median,
            "iqr_default": default_stats["iqr"] if default_stats else float("nan"),
            "min_max_default": fmt_min_max(default_stats["min"], default_stats["max"]) if default_stats else "",
            "n_custom": int(custom_stats["n"]) if custom_stats else 0,
            "median_custom": custom_median,
            "iqr_custom": custom_stats["iqr"] if custom_stats else float("nan"),
            "min_max_custom": fmt_min_max(custom_stats["min"], custom_stats["max"]) if custom_stats else "",
            "delta_median_pct": delta_pct,
            "p_value": p_value,
            "significance": significance_label(p_value),
        })

    return pooled_rows, comparison_rows


def build_html(pooled_rows: list[dict], comparison_rows: list[dict]) -> str:
    if not pooled_rows:
        return "<p>Немає даних для pooled-статистики.</p>\n"

    pooled_map = {
        (r["scenario"], r["metric"], r["series"], r["overlay"]): r
        for r in pooled_rows
    }
    cmp_map = {
        (r["scenario"], r["metric_name"], r["series_name"]): r
        for r in comparison_rows
    }

    html = ""
    columns = [
        "Метрика",
        "Плагін",
        "N (точок)",
        "Медіана (p50)",
        "IQR (p25-p75)",
        "Мінімальне-Максимальне",
        "Δmedian %",
        "p-value (Mann-Whitney)",
        "Рівень значущості",
    ]

    for scenario, config in SCENARIOS_CONFIG.items():
        keys = {
            (row["metric"], row["series"])
            for row in pooled_rows
            if row["scenario"] == scenario
        }
        if not keys:
            continue

        metric_order = {m: i for i, m in enumerate(config.get("metrics", []))}
        ordered_keys = sorted(keys, key=lambda it: (metric_order.get(it[0], 999), it[0], it[1]))

        title = f"{config['title']} — об'єднані сирі виміри (Mann-Whitney U)"
        html += (
            "<table border='1' style='border-collapse: collapse; width: 100%; "
            "font-family: \"Times New Roman\", serif;'>\n"
        )
        html += (
            f"  <tr><td colspan='{len(columns)}' style='padding: 5px; font-weight: bold;'>{title}</td></tr>\n"
        )
        html += "  <tr>\n"
        for col in columns:
            html += f"    <th style='padding: 5px; text-align: center;'>{col}</th>\n"
        html += "  </tr>\n"

        previous_metric = None
        for metric, series in ordered_keys:
            cmp_row = cmp_map.get((scenario, metric, series), {})
            metric_name = metric_display_name(metric, series)
            for overlay in OVERLAYS:
                stats = pooled_map.get((scenario, metric, series, overlay))
                if not stats:
                    continue

                metric_cell = f"<b>{metric_name}</b>" if metric_name != previous_metric else ""
                previous_metric = metric_name

                iqr_str = f"{fmt_num(stats['p25'])}-{fmt_num(stats['p75'])}"
                row_cells = [
                    metric_cell,
                    overlay,
                    str(int(stats["n"])),
                    fmt_num(stats["median"]),
                    iqr_str,
                    fmt_min_max(stats["min"], stats["max"]),
                    fmt_num(cmp_row.get("delta_median_pct", float("nan"))) if overlay == "default" else "",
                    fmt_num(cmp_row.get("p_value", float("nan"))) if overlay == "default" else "",
                    cmp_row.get("significance", "") if overlay == "default" else "",
                ]

                html += "  <tr>\n"
                for idx, cell in enumerate(row_cells):
                    align = "left" if idx in (0, 1) else "center"
                    html += f"    <td style='text-align: {align}; padding: 5px;'>{cell}</td>\n"
                html += "  </tr>\n"

        html += "</table><br><br>\n"

    if not html:
        return "<p>Немає даних для pooled-таблиць.</p>\n"
    return html


def write_csv(path: str, rows: list[dict], fields: list[str]) -> None:
    with open(path, "w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=fields)
        writer.writeheader()
        for row in rows:
            writer.writerow(row)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Generate pooled statistics and Mann-Whitney tables.",
    )
    parser.add_argument(
        "aggregate_dir",
        nargs="?",
        default=".",
        help="Directory containing long_all.csv (default: cwd)",
    )
    parser.add_argument(
        "--input",
        default="",
        help="Path to long_all.csv (overrides aggregate_dir/long_all.csv)",
    )
    parser.add_argument(
        "--run-dir",
        default="",
        help="Single results/run-<ts> dir",
    )
    parser.add_argument(
        "--out-dir",
        default="",
        help="Output directory (default: base input directory)",
    )
    args = parser.parse_args()

    try:
        df_raw, base_dir = load_input_df(args)
        df = normalize_df(df_raw)
    except (FileNotFoundError, ValueError) as exc:
        sys.stderr.write(f"ERROR: {exc}\n")
        return 1

    if df.empty:
        sys.stderr.write("ERROR: no usable pooled samples\n")
        return 1

    out_dir = os.path.abspath(args.out_dir) if args.out_dir else base_dir
    os.makedirs(out_dir, exist_ok=True)

    pooled_rows, comparison_rows = compute_pooled_stats(df)

    pooled_csv = os.path.join(out_dir, "pooled_stats.csv")
    write_csv(
        pooled_csv,
        pooled_rows,
        [
            "scenario",
            "metric",
            "series",
            "overlay",
            "n",
            "median",
            "p25",
            "p75",
            "iqr",
            "min",
            "max",
        ],
    )

    pooled_cmp_csv = os.path.join(out_dir, "pooled_stats_comparison.csv")
    write_csv(
        pooled_cmp_csv,
        comparison_rows,
        [
            "scenario",
            "metric_name",
            "series_name",
            "n_default",
            "median_default",
            "iqr_default",
            "min_max_default",
            "n_custom",
            "median_custom",
            "iqr_custom",
            "min_max_custom",
            "delta_median_pct",
            "p_value",
            "significance",
        ],
    )

    html_path = os.path.join(out_dir, "results_tables_pooled.html")
    with open(html_path, "w", encoding="utf-8") as fh:
        fh.write(build_html(pooled_rows, comparison_rows))

    print(f"Output directory: {out_dir}")
    print(f"  pooled_stats.csv            ({len(pooled_rows)} rows)")
    print(f"  pooled_stats_comparison.csv ({len(comparison_rows)} rows)")
    print("  results_tables_pooled.html")
    return 0


if __name__ == "__main__":
    sys.exit(main())
