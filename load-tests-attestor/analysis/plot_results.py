#!/usr/bin/env python3
"""Generate comparison charts from a load-test run directory.

Reads results/<run>/<overlay>-scenario-<id>-<ts>/prometheus/*.json (Prometheus
query_range matrices) plus k6-summary.json, and produces:
  - <run>/plots/scenario-<id>/<metric>.png   time series, default vs custom-jvm
  - <run>/plots/aggregate-<metric>.png        bar charts across scenarios
  - <run>/long.csv                            tidy data (for Excel/other tools)

Usage: plot_results.py <run-dir>
"""
import csv
import json
import os
import re
import sys
from collections import defaultdict

def _require_matplotlib():
    try:
        import matplotlib
        matplotlib.use("Agg")
        import matplotlib.pyplot as plt
        return plt
    except ImportError:
        sys.stderr.write(
            "ERROR: matplotlib is required. Install with: pip install matplotlib\n"
        )
        sys.exit(2)

RUN_DIR_RE = re.compile(r"^(?P<overlay>default|custom-jvm)-scenario-(?P<scenario>[a-d])-")

# Prometheus metric files to plot as time series (filename without .json).
TIME_SERIES_METRICS = [
    "attestation_avg_ms",
    "agent_cpu",
    "agent_memory_mb",
    "server_cpu",
    "server_memory_mb",
    "http_req_p95_ms",
    "http_req_p99_ms",
    "http_req_rate",
    "http_5xx_rate",
    "svid_issued_rate",
    "jvm_gc_pause_rate",
    "jvm_heap_bytes",
]

# Gauge metrics summarised as a single scalar via a simple mean over the window.
# These are true gauges (CPU cores, RSS MB) that are sampled continuously over
# the full window, so an unweighted mean is the correct estimator.
AGGREGATE_MEAN_METRICS = [
    "agent_cpu",
    "agent_memory_mb",
]

# Percentile metrics carry a per-service label. Pooling services into one mean
# mixes two different latency distributions, so we emit one bar chart per
# service instead of a single blended bar.
AGGREGATE_PER_SERVICE_METRICS = [
    "http_req_p95_ms",
    "http_req_p99_ms",
]

OVERLAY_STYLE = {
    "default": {"linestyle": "--"},
    "custom-jvm": {"linestyle": "-"},
}


def parse_matrix(path):
    """Return list of (labels_dict, [(ts, value_float_or_nan), ...])."""
    try:
        with open(path, "r", encoding="utf-8") as fh:
            doc = json.load(fh)
    except (OSError, json.JSONDecodeError):
        return []
    if doc.get("status") != "success":
        return []
    series = []
    for res in doc.get("data", {}).get("result", []):
        labels = res.get("metric", {})
        points = []
        for ts, val in res.get("values", []):
            try:
                v = float(val)
            except (TypeError, ValueError):
                v = float("nan")
            points.append((float(ts), v))
        if points:
            series.append((labels, points))
    return series


def label_suffix(labels):
    for key in ("service", "pod", "application", "status", "action"):
        if key in labels:
            return labels[key]
    if labels:
        return ",".join(f"{k}={v}" for k, v in sorted(labels.items()))
    return ""


def discover_runs(run_dir):
    """Return {scenario: {overlay: subdir_path}}."""
    runs = defaultdict(dict)
    for name in sorted(os.listdir(run_dir)):
        full = os.path.join(run_dir, name)
        if not os.path.isdir(full):
            continue
        m = RUN_DIR_RE.match(name)
        if not m:
            continue
        runs[m.group("scenario")][m.group("overlay")] = full
    return runs


def nanmean(values):
    clean = [v for v in values if v == v]  # drop NaN
    return sum(clean) / len(clean) if clean else float("nan")


def counter_delta_sum(path):
    """Sum over series of (last - first). For monotonic Prometheus counters this
    is the total increment across the window."""
    total = 0.0
    found = False
    for _, points in parse_matrix(path):
        vals = [v for _, v in points if v == v]
        if vals:
            total += vals[-1] - vals[0]
            found = True
    return total if found else float("nan")


def read_window_seconds(subdir):
    """Full measurement-window length in seconds from window.env, else NaN."""
    start = end = None
    try:
        with open(os.path.join(subdir, "window.env"), encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if line.startswith("START="):
                    start = float(line[len("START="):])
                elif line.startswith("END="):
                    end = float(line[len("END="):])
    except (OSError, ValueError):
        return float("nan")
    if start is not None and end is not None and end > start:
        return end - start
    return float("nan")


def attestation_weighted_ms(subdir):
    """Count-weighted mean attestation time (ms): total elapsed_sum / total
    elapsed_count over the window. This is the unbiased per-attestation average,
    unlike a simple mean of the pre-averaged 1m-rate gauge."""
    prom = os.path.join(subdir, "prometheus")
    s = counter_delta_sum(os.path.join(prom, "attestation_elapsed_sum.json"))
    c = counter_delta_sum(os.path.join(prom, "attestation_elapsed_count.json"))
    if s == s and c == c and c > 0:
        return s / c
    return float("nan")


def rate_mean_full_window(path, window_s):
    """Mean of a rate series over the FULL measurement window.

    A rate series (e.g. 5xx/s) can go stale and disappear once the underlying
    counter stops incrementing, so its samples may cover only part of the run.
    Averaging just those samples over-weights the active burst. Instead we
    integrate the rate over its own timestamps (trapezoid -> total events) and
    divide by the full window length, treating absent time as zero. Falls back
    to a simple mean when the window length is unknown."""
    series = parse_matrix(path)
    if not series:
        return float("nan")
    total_area = 0.0
    all_vals = []
    for _, points in series:
        pts = [(t, v) for t, v in points if v == v]
        all_vals.extend(v for _, v in pts)
        for i in range(1, len(pts)):
            (t0, v0), (t1, v1) = pts[i - 1], pts[i]
            total_area += (v0 + v1) / 2.0 * (t1 - t0)
    if window_s == window_s and window_s > 0:
        return total_area / window_s
    return nanmean(all_vals) if all_vals else float("nan")


def service_means(path):
    """Return {service_label: mean-over-window} for a per-service metric."""
    out = {}
    for labels, points in parse_matrix(path):
        svc = labels.get("service") or label_suffix(labels) or "series"
        out[svc] = nanmean([v for _, v in points])
    return out


def collect_long_rows(runs):
    rows = []
    for scenario in sorted(runs):
        for overlay, subdir in runs[scenario].items():
            prom_dir = os.path.join(subdir, "prometheus")
            if not os.path.isdir(prom_dir):
                continue
            for fname in sorted(os.listdir(prom_dir)):
                if not fname.endswith(".json"):
                    continue
                metric = fname[:-5]
                series = parse_matrix(os.path.join(prom_dir, fname))
                for labels, points in series:
                    t0 = points[0][0]
                    suffix = label_suffix(labels)
                    for ts, val in points:
                        rows.append({
                            "scenario": scenario,
                            "overlay": overlay,
                            "metric": metric,
                            "series": suffix,
                            "elapsed_s": int(ts - t0),
                            "value": "" if val != val else val,
                        })
    return rows


def write_long_csv(rows, out_path):
    with open(out_path, "w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(
            fh, fieldnames=["scenario", "overlay", "metric", "series", "elapsed_s", "value"]
        )
        writer.writeheader()
        writer.writerows(rows)


def plot_time_series(runs, plots_dir):
    plt = _require_matplotlib()
    for scenario in sorted(runs):
        sc_dir = os.path.join(plots_dir, f"scenario-{scenario}")
        os.makedirs(sc_dir, exist_ok=True)
        for metric in TIME_SERIES_METRICS:
            fig, ax = plt.subplots(figsize=(10, 5))
            plotted = False
            for overlay, subdir in sorted(runs[scenario].items()):
                path = os.path.join(subdir, "prometheus", f"{metric}.json")
                if not os.path.isfile(path):
                    continue
                for labels, points in parse_matrix(path):
                    t0 = points[0][0]
                    xs = [ts - t0 for ts, _ in points]
                    ys = [v for _, v in points]
                    suffix = label_suffix(labels)
                    lbl = overlay + (f" [{suffix}]" if suffix else "")
                    style = OVERLAY_STYLE.get(overlay, {})
                    ax.plot(xs, ys, label=lbl, linewidth=1.4, **style)
                    plotted = True
            if not plotted:
                plt.close(fig)
                continue
            ax.set_title(f"Scenario {scenario.upper()} - {metric}")
            ax.set_xlabel("elapsed (s)")
            ax.set_ylabel(metric)
            ax.grid(True, alpha=0.3)
            ax.legend(fontsize=8)
            fig.tight_layout()
            fig.savefig(os.path.join(sc_dir, f"{metric}.png"), dpi=120)
            plt.close(fig)


def _grouped_bar(scenarios, default_vals, custom_vals, title, ylabel, out_path):
    """Render a default-vs-custom grouped bar chart; skip if no data."""
    plt = _require_matplotlib()
    if all(v != v for v in default_vals + custom_vals):
        return
    x = range(len(scenarios))
    width = 0.38
    fig, ax = plt.subplots(figsize=(8, 5))
    ax.bar([i - width / 2 for i in x], [0 if v != v else v for v in default_vals],
           width, label="default")
    ax.bar([i + width / 2 for i in x], [0 if v != v else v for v in custom_vals],
           width, label="custom-jvm")
    ax.set_title(title)
    ax.set_xticks(list(x))
    ax.set_xticklabels([f"S-{s.upper()}" for s in scenarios])
    ax.set_ylabel(ylabel)
    ax.grid(True, axis="y", alpha=0.3)
    ax.legend()
    fig.tight_layout()
    fig.savefig(out_path, dpi=120)
    plt.close(fig)


def _scalar_for(metric, subdir):
    """Statistically appropriate single-window scalar for a metric.

    - attestation_avg_ms: count-weighted (Δsum/Δcount), the unbiased mean.
    - http_5xx_rate: mean over the FULL window (rate series can be truncated).
    - other gauges: simple mean over the window.
    """
    if not subdir:
        return float("nan")
    prom = os.path.join(subdir, "prometheus")
    if metric == "attestation_avg_ms":
        return attestation_weighted_ms(subdir)
    if metric == "http_5xx_rate":
        return rate_mean_full_window(
            os.path.join(prom, "http_5xx_rate.json"), read_window_seconds(subdir)
        )
    path = os.path.join(prom, f"{metric}.json")
    return nanmean([v for _, points in parse_matrix(path) for _, v in points])


def plot_aggregate_bars(runs, plots_dir):
    scenarios = sorted(runs)

    # attestation (count-weighted) + gauges (simple mean) + 5xx (full-window).
    scalar_specs = [
        ("attestation_avg_ms", "attestation_avg_ms (count-weighted, ms)"),
        ("http_5xx_rate", "http_5xx_rate (mean over full window, req/s)"),
    ] + [(m, f"{m} (mean over window)") for m in AGGREGATE_MEAN_METRICS]

    for metric, title in scalar_specs:
        default_vals = [_scalar_for(metric, runs[s].get("default")) for s in scenarios]
        custom_vals = [_scalar_for(metric, runs[s].get("custom-jvm")) for s in scenarios]
        _grouped_bar(
            scenarios, default_vals, custom_vals, title, metric,
            os.path.join(plots_dir, f"aggregate-{metric}.png"),
        )

    # Percentile metrics: one chart per service, never blended across services.
    for metric in AGGREGATE_PER_SERVICE_METRICS:
        services = set()
        for scenario in scenarios:
            for overlay in ("default", "custom-jvm"):
                subdir = runs[scenario].get(overlay)
                if subdir:
                    p = os.path.join(subdir, "prometheus", f"{metric}.json")
                    services.update(service_means(p).keys())
        for svc in sorted(services):
            default_vals, custom_vals = [], []
            for scenario in scenarios:
                for overlay, bucket in (("default", default_vals),
                                        ("custom-jvm", custom_vals)):
                    subdir = runs[scenario].get(overlay)
                    val = float("nan")
                    if subdir:
                        p = os.path.join(subdir, "prometheus", f"{metric}.json")
                        val = service_means(p).get(svc, float("nan"))
                    bucket.append(val)
            safe_svc = re.sub(r"[^A-Za-z0-9_.-]", "_", svc)
            _grouped_bar(
                scenarios, default_vals, custom_vals,
                f"{metric} [{svc}] (mean over window)", metric,
                os.path.join(plots_dir, f"aggregate-{metric}-{safe_svc}.png"),
            )


def main():
    if len(sys.argv) < 2:
        sys.stderr.write("Usage: plot_results.py <run-dir>\n")
        sys.exit(1)
    run_dir = os.path.abspath(sys.argv[1])
    if not os.path.isdir(run_dir):
        sys.stderr.write(f"ERROR: not a directory: {run_dir}\n")
        sys.exit(1)

    runs = discover_runs(run_dir)
    if not runs:
        sys.stderr.write(
            f"ERROR: no '<overlay>-scenario-<id>-*' subdirs found in {run_dir}\n"
        )
        sys.exit(1)

    plots_dir = os.path.join(run_dir, "plots")
    os.makedirs(plots_dir, exist_ok=True)

    rows = collect_long_rows(runs)
    long_csv = os.path.join(run_dir, "long.csv")
    write_long_csv(rows, long_csv)

    plot_time_series(runs, plots_dir)
    plot_aggregate_bars(runs, plots_dir)

    scen_summary = ", ".join(
        f"S-{s.upper()}({'+'.join(sorted(runs[s]))})" for s in sorted(runs)
    )
    print(f"Scenarios found: {scen_summary}")
    print(f"Tidy data : {long_csv}")
    print(f"Charts    : {plots_dir}")


if __name__ == "__main__":
    main()
