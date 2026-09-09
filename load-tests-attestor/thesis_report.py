#!/usr/bin/env python3
"""Розширений статистичний звіт для розділу 4 (custom-jvm vs default attestor).

Реалізує кроки методології зі схеми "Обробка даних":
  - описова статистика: медіана, IQR, min-max, середнє, 95% ДІ;
  - парний аналіз default vs custom-jvm (той самий прогін = пара);
  - тест Вілкоксона (Wilcoxon signed-rank) для кожної пари метрик/сценаріїв;
  - розрахунок розміру ефекту (matched-pairs rank-biserial correlation r);
  - HTML-таблиці, готові для вставки в текст роботи (Word copy-paste);
  - візуалізація: box-plot розподілів + графік парних різниць (delta) по
    прогонах.

Бере довільну кількість прогонів `run-*` (кожен прогін = один незалежний
статистичний повтор, отриманий через run-all.sh) і будує звіт по НИХ.

Використання:
  python thesis_report.py --glob "results/run-*" --last 5
  python thesis_report.py results/run-A results/run-B results/run-C
  python thesis_report.py --glob "results/run-*"          # усі прогони
  python thesis_report.py --glob "results/run-*" --last 5 --alpha 0.05

Вихід (у --out-dir, за замовчуванням results/thesis-report-<utc-ts>/):
  run_scalars.csv          сирі скаляри по кожному прогону/сценарію/метриці
  descriptive_stats.csv    описова статистика (mean, median, IQR, CI, ...)
  paired_comparison.csv    парний аналіз + Вілкоксон + розмір ефекту
  thesis_tables.html       обидві таблиці разом, готові для вставки в текст
  plots/box-<scenario>-<metric>[-<service>].png     box-plot розподілів
  plots/delta-<scenario>-<metric>[-<service>].png   парні різниці по прогонах
"""
from __future__ import annotations

import argparse
import csv
import glob
import math
import os
import statistics
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone

for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8")
    except (AttributeError, ValueError):
        pass

from plot_results import counter_delta_sum, discover_runs, nanmean, parse_matrix
from aggregate_runs import (
    PER_SERVICE_METRICS,
    SCENARIOS_CONFIG,
    run_label,
    scalar_for_metric,
    scalars_per_service,
)

# ---------------------------------------------------------------------------
# Extra "reliability" metrics not covered by aggregate_runs.py: total issued
# SVIDs (proxy for successful attestations) and, for the HTTP-load scenario,
# k6 checks success-rate / failed-request count.
# ---------------------------------------------------------------------------
EXTRA_METRICS: dict[str, list[str]] = {
    "a": ["svid_issued_total"],
    "b": ["svid_issued_total"],
    "c": ["svid_issued_total", "success_rate_pct", "http_error_count"],
}

METRIC_LABELS_UA: dict[str, str] = {
    "attestation_avg_ms": "Час атестації (мс)",
    "agent_cpu": "CPU агента SPIRE (ядра)",
    "agent_memory_mb": "RAM агента SPIRE (МБ)",
    "server_cpu": "CPU серверу SPIRE (ядра)",
    "server_memory_mb": "RAM серверу SPIRE (МБ)",
    "http_req_p95_ms": "HTTP latency p95 (мс)",
    "http_req_p99_ms": "HTTP latency p99 (мс)",
    "http_5xx_rate": "Частка 5xx помилок (req/s)",
    "process_cpu": "CPU застосунку (ядра)",
    "jvm_heap_bytes": "JVM heap (МБ)",
    "jvm_gc_pause_rate": "Частота пауз GC",
    "svid_issued_total": "Кількість виданих SVID (успішні атестації)",
    "success_rate_pct": "Success rate (%)",
    "http_error_count": "Кількість HTTP помилок",
}


def metric_label(metric: str, series: str = "") -> str:
    base = METRIC_LABELS_UA.get(metric, metric)
    return f"{base} [{series}]" if series else base


def read_k6_summary(subdir: str) -> dict | None:
    path = os.path.join(subdir, "k6-summary.json")
    try:
        import json
        with open(path, encoding="utf-8") as fh:
            return json.load(fh)
    except (OSError, ValueError):
        return None


def success_rate_pct(subdir: str) -> float:
    doc = read_k6_summary(subdir)
    if not doc:
        return float("nan")
    rate = doc.get("metrics", {}).get("checks", {}).get("values", {}).get("rate")
    return rate * 100.0 if isinstance(rate, (int, float)) else float("nan")


def http_error_count(subdir: str) -> float:
    doc = read_k6_summary(subdir)
    if not doc:
        return float("nan")
    fails = doc.get("metrics", {}).get("http_req_failed", {}).get("values", {}).get("fails")
    return float(fails) if isinstance(fails, (int, float)) else float("nan")


def svid_issued_total(subdir: str) -> float:
    path = os.path.join(subdir, "prometheus", "svid_issued_total.json")
    return counter_delta_sum(path)


def scalar_for_metric_ext(subdir: str, metric: str) -> float:
    if metric == "success_rate_pct":
        return success_rate_pct(subdir)
    if metric == "http_error_count":
        return http_error_count(subdir)
    if metric == "svid_issued_total":
        return svid_issued_total(subdir)
    return scalar_for_metric(subdir, metric)


def all_metrics_for_scenario(scenario: str) -> list[str]:
    base = list(SCENARIOS_CONFIG.get(scenario, {}).get("metrics", []))
    for m in EXTRA_METRICS.get(scenario, []):
        if m not in base:
            base.append(m)
    return base


# ---------------------------------------------------------------------------
# Collection
# ---------------------------------------------------------------------------

def collect_run_scalars(run_dir: str) -> list[dict]:
    run_id = run_label(run_dir)
    runs = discover_runs(run_dir)
    rows: list[dict] = []
    for scenario, overlays in sorted(runs.items()):
        if scenario not in SCENARIOS_CONFIG:
            continue
        for metric in all_metrics_for_scenario(scenario):
            for overlay, subdir in sorted(overlays.items()):
                if metric in PER_SERVICE_METRICS:
                    for series, val in sorted(scalars_per_service(subdir, metric).items()):
                        if val == val:
                            rows.append({
                                "run": run_id, "scenario": scenario, "overlay": overlay,
                                "metric": metric, "series": series, "value": val,
                            })
                else:
                    val = scalar_for_metric_ext(subdir, metric)
                    if val == val:
                        rows.append({
                            "run": run_id, "scenario": scenario, "overlay": overlay,
                            "metric": metric, "series": "", "value": val,
                        })
    return rows


def write_csv(rows: list[dict], fields: list[str], out_path: str) -> None:
    with open(out_path, "w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=fields)
        writer.writeheader()
        writer.writerows(rows)


# ---------------------------------------------------------------------------
# Descriptive statistics (mean, std, 95% CI, median, IQR, min, max)
# ---------------------------------------------------------------------------

T_CRITICAL_975: dict[int, float] = {
    1: 12.706, 2: 4.303, 3: 3.182, 4: 2.776, 5: 2.571, 6: 2.447, 7: 2.365,
    8: 2.306, 9: 2.262, 10: 2.228, 11: 2.201, 12: 2.179, 13: 2.160, 14: 2.145,
    15: 2.131, 16: 2.120, 17: 2.110, 18: 2.101, 19: 2.093, 20: 2.086,
    21: 2.080, 22: 2.074, 23: 2.069, 24: 2.064, 25: 2.060, 26: 2.056,
    27: 2.052, 28: 2.048, 29: 2.045, 30: 2.042,
}


def t_critical_975(n: int) -> float:
    if n < 2:
        return float("nan")
    return T_CRITICAL_975.get(n - 1, 1.96)


def q1_q3(values: list[float]) -> tuple[float, float]:
    if len(values) < 2:
        v = values[0] if values else float("nan")
        return v, v
    try:
        qs = statistics.quantiles(values, n=4, method="inclusive")
        return qs[0], qs[2]
    except statistics.StatisticsError:
        v = values[0]
        return v, v


def compute_descriptive_stats(scalar_rows: list[dict]) -> list[dict]:
    groups: dict[tuple, list[float]] = defaultdict(list)
    for row in scalar_rows:
        key = (row["scenario"], row["overlay"], row["metric"], row["series"])
        groups[key].append(float(row["value"]))

    out: list[dict] = []
    for (scenario, overlay, metric, series), values in sorted(groups.items()):
        n = len(values)
        mean = statistics.mean(values)
        std = statistics.stdev(values) if n > 1 else 0.0
        median = statistics.median(values)
        q1, q3 = q1_q3(values)
        t = t_critical_975(n)
        ci_half = t * std / math.sqrt(n) if n > 1 and t == t else float("nan")
        out.append({
            "scenario": scenario, "overlay": overlay, "metric": metric, "series": series,
            "n": n, "mean": mean, "std": std,
            "ci95_halfwidth": ci_half,
            "median": median, "q1": q1, "q3": q3, "iqr": q3 - q1,
            "min": min(values), "max": max(values),
        })
    return out


# ---------------------------------------------------------------------------
# Paired analysis: Wilcoxon signed-rank test + effect size
# ---------------------------------------------------------------------------

def norm_cdf(x: float) -> float:
    return 0.5 * (1.0 + math.erf(x / math.sqrt(2.0)))


def wilcoxon_signed_rank(diffs: list[float]) -> dict:
    """Paired Wilcoxon signed-rank test, normal approximation with continuity
    and tie correction (matches the classic large-sample approximation used
    by scipy.stats.wilcoxon). ``diffs`` = custom_jvm - default per matched run.

    Returns dict with n_pairs, n_nonzero, W, z, p, r (matched-pairs
    rank-biserial correlation = effect size), median_diff, iqr_diff.
    """
    n_pairs = len(diffs)
    median_diff = statistics.median(diffs) if diffs else float("nan")
    q1, q3 = q1_q3(diffs) if diffs else (float("nan"), float("nan"))

    nonzero = [d for d in diffs if d != 0]
    n = len(nonzero)
    result = {
        "n_pairs": n_pairs, "n_nonzero": n,
        "median_diff": median_diff, "iqr_diff": q3 - q1,
        "W": float("nan"), "z": float("nan"), "p": float("nan"), "r": float("nan"),
    }
    if n == 0:
        return result

    order = sorted(range(n), key=lambda i: abs(nonzero[i]))
    rank_of = [0.0] * n
    idx = 0
    while idx < n:
        j = idx
        while j + 1 < n and abs(nonzero[order[j + 1]]) == abs(nonzero[order[idx]]):
            j += 1
        avg_rank = (idx + 1 + j + 1) / 2.0
        for k in range(idx, j + 1):
            rank_of[order[k]] = avg_rank
        idx = j + 1

    w_pos = sum(rank_of[i] for i in range(n) if nonzero[i] > 0)
    w_neg = sum(rank_of[i] for i in range(n) if nonzero[i] < 0)
    W = min(w_pos, w_neg)
    mean_W = n * (n + 1) / 4.0

    tie_sizes = Counter(rank_of).values()
    tie_term = sum(t ** 3 - t for t in tie_sizes)
    var_W = n * (n + 1) * (2 * n + 1) / 24.0 - tie_term / 48.0
    sd_W = math.sqrt(var_W) if var_W > 0 else 0.0

    if sd_W == 0:
        z = 0.0
    else:
        correction = 0.5 if W < mean_W else (-0.5 if W > mean_W else 0.0)
        z = (W - mean_W + correction) / sd_W
    p = 2.0 * (1.0 - norm_cdf(abs(z)))
    p = min(max(p, 0.0), 1.0)
    r = z / math.sqrt(n)

    result.update(W=W, z=z, p=p, r=r)
    return result


def compute_paired_comparisons(scalar_rows: list[dict]) -> list[dict]:
    """One row per (scenario, metric, series): Wilcoxon(custom_jvm - default)
    across runs that have BOTH overlays for that metric/series."""
    by_key: dict[tuple, dict[str, dict[str, float]]] = defaultdict(dict)
    for row in scalar_rows:
        key = (row["scenario"], row["metric"], row["series"])
        by_key[key].setdefault(row["run"], {})[row["overlay"]] = float(row["value"])

    out: list[dict] = []
    for (scenario, metric, series), per_run in sorted(by_key.items()):
        diffs = []
        for run_id, overlays in sorted(per_run.items()):
            if "default" in overlays and "custom-jvm" in overlays:
                diffs.append(overlays["custom-jvm"] - overlays["default"])
        if not diffs:
            continue
        stats = wilcoxon_signed_rank(diffs)
        out.append({"scenario": scenario, "metric": metric, "series": series, **stats})
    return out


# ---------------------------------------------------------------------------
# HTML tables
# ---------------------------------------------------------------------------

def _fmt(val, digits: int = 3) -> str:
    if val is None or (isinstance(val, float) and val != val):
        return "-"
    if isinstance(val, (int, float)):
        return f"{val:.{digits}f}"
    return str(val)


def _table_open(title: str, colspan: int) -> str:
    html = (
        "<table border='1' style=\"border-collapse: collapse; width: 100%; "
        "font-family: 'Times New Roman', serif;\">\n"
    )
    html += f"  <tr>\n    <td colspan='{colspan}' style='padding: 5px; font-weight: bold;'>{title}</td>\n  </tr>\n"
    return html


def build_descriptive_html(desc_rows: list[dict]) -> str:
    columns = ["Метрика", "Плагін", "N", "Середнє", "±95% CI", "Медіана", "IQR (Q1-Q3)", "Min", "Max"]
    html_out = ""
    for sc_key, cfg in SCENARIOS_CONFIG.items():
        sc_rows = [r for r in desc_rows if r["scenario"] == sc_key]
        if not sc_rows:
            continue
        html = _table_open(cfg["title"] + " — описова статистика", len(columns))
        html += "  <tr>\n" + "".join(f"    <th style='padding:5px;text-align:center;'>{c}</th>\n" for c in columns) + "  </tr>\n"
        current_metric = ""
        for metric in all_metrics_for_scenario(sc_key):
            for row in sorted([r for r in sc_rows if r["metric"] == metric], key=lambda r: (r["series"], r["overlay"])):
                label = metric_label(metric, row["series"])
                show_label = label if label != current_metric else ""
                current_metric = label
                iqr_str = f"{_fmt(row['q1'])} – {_fmt(row['q3'])}"
                html += "  <tr>\n"
                html += f"    <td style='padding:5px;'>{f'<b>{show_label}</b>' if show_label else ''}</td>\n"
                html += f"    <td style='padding:5px;'>{row['overlay']}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{row['n']}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{_fmt(row['mean'])}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{_fmt(row['ci95_halfwidth'])}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{_fmt(row['median'])}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{iqr_str}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{_fmt(row['min'])}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{_fmt(row['max'])}</td>\n"
                html += "  </tr>\n"
        html += "</table><br><br>\n"
        html_out += html
    return html_out


def build_paired_html(paired_rows: list[dict], alpha: float) -> str:
    columns = ["Метрика", "N пар", "Медіана Δ (custom - default)", "IQR Δ", "W", "Z", "p-value", "r (ефект)", "Висновок"]
    html_out = ""
    for sc_key, cfg in SCENARIOS_CONFIG.items():
        sc_rows = [r for r in paired_rows if r["scenario"] == sc_key]
        if not sc_rows:
            continue
        html = _table_open(cfg["title"] + " — парний аналіз (тест Вілкоксона, custom-jvm vs default)", len(columns))
        html += "  <tr>\n" + "".join(f"    <th style='padding:5px;text-align:center;'>{c}</th>\n" for c in columns) + "  </tr>\n"
        for metric in all_metrics_for_scenario(sc_key):
            for row in sorted([r for r in sc_rows if r["metric"] == metric], key=lambda r: r["series"]):
                label = metric_label(metric, row["series"])
                p = row["p"]
                if p != p:
                    conclusion = "недостатньо даних"
                elif p < alpha:
                    direction = "custom-jvm вищий" if row["median_diff"] > 0 else "custom-jvm нижчий"
                    conclusion = f"significant (p&lt;{alpha}), {direction}"
                else:
                    conclusion = "не значуще"
                iqr_str = f"{_fmt(row['iqr_diff'])}"
                html += "  <tr>\n"
                html += f"    <td style='padding:5px;'>{label}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{row['n_pairs']}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{_fmt(row['median_diff'])}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{iqr_str}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{_fmt(row['W'], 1)}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{_fmt(row['z'])}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{_fmt(row['p'], 4)}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{_fmt(row['r'])}</td>\n"
                html += f"    <td style='padding:5px;text-align:center;'>{conclusion}</td>\n"
                html += "  </tr>\n"
        html += "</table><br><br>\n"
        html_out += html
    return html_out


# ---------------------------------------------------------------------------
# Visualizations
# ---------------------------------------------------------------------------

def _require_matplotlib():
    try:
        import matplotlib
        matplotlib.use("Agg")
        import matplotlib.pyplot as plt
        return plt
    except ImportError:
        sys.stderr.write("ERROR: matplotlib is required. Install with: pip install matplotlib\n")
        sys.exit(2)


def plot_boxplots(scalar_rows: list[dict], plots_dir: str) -> None:
    plt = _require_matplotlib()
    groups: dict[tuple, dict[str, list[float]]] = defaultdict(lambda: defaultdict(list))
    for row in scalar_rows:
        key = (row["scenario"], row["metric"], row["series"])
        groups[key][row["overlay"]].append(float(row["value"]))

    for (scenario, metric, series), by_overlay in groups.items():
        overlays = [o for o in ("default", "custom-jvm") if by_overlay.get(o)]
        if not overlays:
            continue
        data = [by_overlay[o] for o in overlays]
        fig, ax = plt.subplots(figsize=(5, 5))
        ax.boxplot(data, tick_labels=overlays, showmeans=True)
        title = metric_label(metric, series)
        ax.set_title(f"S-{scenario.upper()}: {title}")
        ax.set_ylabel(title)
        ax.grid(True, axis="y", alpha=0.3)
        fig.tight_layout()
        suffix = f"-{series}" if series else ""
        safe_suffix = "".join(c if c.isalnum() or c in "-_" else "_" for c in suffix)
        fig.savefig(os.path.join(plots_dir, f"box-{scenario}-{metric}{safe_suffix}.png"), dpi=120)
        plt.close(fig)


def plot_deltas(scalar_rows: list[dict], plots_dir: str) -> None:
    plt = _require_matplotlib()
    by_key: dict[tuple, dict[str, dict[str, float]]] = defaultdict(dict)
    for row in scalar_rows:
        key = (row["scenario"], row["metric"], row["series"])
        by_key[key].setdefault(row["run"], {})[row["overlay"]] = float(row["value"])

    for (scenario, metric, series), per_run in by_key.items():
        runs = sorted(per_run)
        diffs = []
        xs = []
        for i, run_id in enumerate(runs, start=1):
            overlays = per_run[run_id]
            if "default" in overlays and "custom-jvm" in overlays:
                diffs.append(overlays["custom-jvm"] - overlays["default"])
                xs.append(i)
        if len(diffs) < 2:
            continue
        median = statistics.median(diffs)
        fig, ax = plt.subplots(figsize=(6, 4))
        colors = ["tab:red" if d > 0 else "tab:blue" for d in diffs]
        ax.scatter(xs, diffs, c=colors, zorder=3)
        ax.axhline(0, color="gray", linewidth=1, linestyle="--")
        ax.axhline(median, color="black", linewidth=1.2, label=f"медіана Δ = {median:.3f}")
        title = metric_label(metric, series)
        ax.set_title(f"S-{scenario.upper()}: парна різниця (custom-jvm - default), {title}")
        ax.set_xlabel("прогін (run) #")
        ax.set_ylabel(f"Δ {title}")
        ax.set_xticks(xs)
        ax.grid(True, alpha=0.3)
        ax.legend(fontsize=8)
        fig.tight_layout()
        suffix = f"-{series}" if series else ""
        safe_suffix = "".join(c if c.isalnum() or c in "-_" else "_" for c in suffix)
        fig.savefig(os.path.join(plots_dir, f"delta-{scenario}-{metric}{safe_suffix}.png"), dpi=120)
        plt.close(fig)


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def resolve_run_dirs(run_dirs: list[str], glob_pattern: str, last: int) -> list[str]:
    dirs: list[str] = list(run_dirs)
    if glob_pattern:
        dirs.extend(sorted(glob.glob(glob_pattern)))
    dirs = [os.path.abspath(d) for d in dirs]
    seen: set[str] = set()
    unique: list[str] = []
    for d in dirs:
        if d not in seen:
            seen.add(d)
            unique.append(d)
    if last and last > 0:
        unique = sorted(unique)[-last:]
    return unique


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("run_dirs", nargs="*", help="Директорії прогонів (results/run-<ts>)")
    parser.add_argument("--glob", dest="glob_pattern", default="", help="Glob для прогонів, напр. 'results/run-*'")
    parser.add_argument("--last", type=int, default=0, help="Взяти лише N останніх (за назвою) прогонів після --glob")
    parser.add_argument("--out-dir", default="", help="Куди зберегти звіт (default: results/thesis-report-<ts>)")
    parser.add_argument("--alpha", type=float, default=0.05, help="Рівень значущості для тесту Вілкоксона (default 0.05)")
    args = parser.parse_args()

    run_dirs = resolve_run_dirs(args.run_dirs, args.glob_pattern, args.last)
    if not run_dirs:
        sys.stderr.write("ERROR: жодного прогону не передано. Вкажіть шляхи або --glob 'results/run-*' [--last N]\n")
        return 1
    for d in run_dirs:
        if not os.path.isdir(d) or not discover_runs(d):
            sys.stderr.write(f"ERROR: не знайдено сценаріїв у {d}\n")
            return 1

    lib_dir = os.path.dirname(os.path.abspath(__file__))
    if args.out_dir:
        out_dir = os.path.abspath(args.out_dir)
    else:
        ts = datetime.now(timezone.utc).strftime("%Y%m%d-%H%M%S")
        out_dir = os.path.join(lib_dir, "results", f"thesis-report-{ts}")
    plots_dir = os.path.join(out_dir, "plots")
    os.makedirs(plots_dir, exist_ok=True)

    scalar_rows: list[dict] = []
    for run_dir in run_dirs:
        scalar_rows.extend(collect_run_scalars(run_dir))
    if not scalar_rows:
        sys.stderr.write("ERROR: не вдалось зібрати жодного скаляра з переданих прогонів\n")
        return 1

    write_csv(scalar_rows, ["run", "scenario", "overlay", "metric", "series", "value"],
              os.path.join(out_dir, "run_scalars.csv"))

    desc_rows = compute_descriptive_stats(scalar_rows)
    write_csv(desc_rows,
              ["scenario", "overlay", "metric", "series", "n", "mean", "std",
               "ci95_halfwidth", "median", "q1", "q3", "iqr", "min", "max"],
              os.path.join(out_dir, "descriptive_stats.csv"))

    paired_rows = compute_paired_comparisons(scalar_rows)
    write_csv(paired_rows,
              ["scenario", "metric", "series", "n_pairs", "n_nonzero",
               "median_diff", "iqr_diff", "W", "z", "p", "r"],
              os.path.join(out_dir, "paired_comparison.csv"))

    html = "<h2>Описова статистика (mean, 95% CI, медіана, IQR, min-max)</h2>\n"
    html += build_descriptive_html(desc_rows)
    html += "<h2>Парний аналіз: тест Вілкоксона + розмір ефекту (custom-jvm vs default)</h2>\n"
    html += build_paired_html(paired_rows, args.alpha)
    with open(os.path.join(out_dir, "thesis_tables.html"), "w", encoding="utf-8") as fh:
        fh.write(html)

    plot_boxplots(scalar_rows, plots_dir)
    plot_deltas(scalar_rows, plots_dir)

    run_ids = sorted({r["run"] for r in scalar_rows})
    print(f"Прогонів агреговано : {len(run_ids)} ({', '.join(run_ids)})")
    print(f"Вихідна директорія  : {out_dir}")
    print("  run_scalars.csv")
    print(f"  descriptive_stats.csv ({len(desc_rows)} рядків)")
    print(f"  paired_comparison.csv ({len(paired_rows)} рядків)")
    print("  thesis_tables.html   <- вставити в текст роботи")
    print(f"  plots/               <- box-*.png, delta-*.png")
    return 0


if __name__ == "__main__":
    sys.exit(main())
