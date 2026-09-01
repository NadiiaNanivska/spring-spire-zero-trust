#!/usr/bin/env python3
"""Clean time-series plots for load-test metrics (lines only, no CI bands).

Two modes:
  * Multi-run (default): reads long_all.csv (from aggregate_runs.py), filters
    inactive agent pods, collapses per-pod series, applies 5-second window
    bucketing for smooth trends, and plots clean median/mean lines across runs.
  * Single-run (--run-dir): reads one results/run-<ts> directory directly,
    filters dead pods, collapses to ONE line per overlay.

Usage:
  python spire_metrics_visualization.py [aggregate-dir]
  python spire_metrics_visualization.py --input long_all.csv --plots-dir plots_clean
  python spire_metrics_visualization.py --run-dir results/run-<ts>
"""
from __future__ import annotations

import argparse
import os
import sys

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import seaborn as sns

from aggregate_runs import run_label
from plot_results import collect_long_rows, discover_runs

sns.set_theme(style="whitegrid", context="paper", font_scale=1.2)
OVERLAY_PALETTE = {"custom-jvm": "#1f77b4", "default": "#ff7f0e"}

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


def filter_active_pods(df: pd.DataFrame) -> pd.DataFrame:
    """Drop agent pods that stopped reporting more than 30 s before window end."""
    metrics_to_filter = ["agent_cpu", "agent_memory_mb"]
    df_to_filter = df[df["metric"].isin(metrics_to_filter)]
    df_others = df[~df["metric"].isin(metrics_to_filter)]

    if df_to_filter.empty:
        return df

    group_keys = ["run", "scenario", "overlay", "metric", "series"]
    max_per_series = (
        df_to_filter.groupby(group_keys)["elapsed_s"].max().reset_index()
    )
    max_global = (
        df_to_filter.groupby(["run", "scenario", "overlay", "metric"])["elapsed_s"]
        .max()
        .reset_index()
        .rename(columns={"elapsed_s": "max_global_s"})
    )
    merged = pd.merge(
        max_per_series,
        max_global,
        on=["run", "scenario", "overlay", "metric"],
    )
    active = merged[merged["elapsed_s"] >= merged["max_global_s"] - 30]
    df_filtered = pd.merge(
        df_to_filter,
        active[group_keys],
        on=group_keys,
    )
    return pd.concat([df_filtered, df_others], ignore_index=True)


def collapse_pods_per_run(df: pd.DataFrame) -> pd.DataFrame:
    """One value per (run, scenario, overlay, metric, elapsed_s): mean over series."""
    keys = ["run", "scenario", "overlay", "metric", "elapsed_s"]
    return df.groupby(keys, as_index=False)["value"].mean()


def load_long_df_from_run(run_dir: str) -> pd.DataFrame:
    """Build a long dataframe (with a ``run`` column) from one results/run-<ts> dir."""
    runs = discover_runs(run_dir)
    rows = collect_long_rows(runs)
    run_id = run_label(run_dir)
    for row in rows:
        row["run"] = run_id
    return pd.DataFrame(rows)


def plot_clean_time_series(df: pd.DataFrame, plots_dir: str) -> None:
    """Plot clean trend lines with 5-second bucketing, no CI bands."""
    df = df.copy()
    # Бакетування по 5 секунд для згладжування посекундного інфраструктурного шуму
    df["elapsed_s"] = (df["elapsed_s"] // 5) * 5

    for scenario in sorted(df["scenario"].unique()):
        sc_dir = os.path.join(plots_dir, f"scenario-{scenario}")
        os.makedirs(sc_dir, exist_ok=True)
        df_scen = df[df["scenario"] == scenario]

        for metric in TIME_SERIES_METRICS:
            df_metric = df_scen[df_scen["metric"] == metric]
            if df_metric.empty:
                continue

            fig, ax = plt.subplots(figsize=(10, 5))
            plotted = False

            for overlay in sorted(df_metric["overlay"].unique()):
                df_ov = df_metric[df_metric["overlay"] == overlay]
                
                # Рахуємо медіану або середнє по бакетах для всіх ранів разом
                stats = (
                    df_ov.groupby("elapsed_s")["value"]
                    .agg(mean="mean", median="median")
                    .reset_index()
                    .sort_values("elapsed_s")
                )
                
                color = OVERLAY_PALETTE.get(overlay, None)
                
                # Малюємо лише чисту суцільну лінію тренду (використовуємо медіану або mean)
                ax.plot(
                    stats["elapsed_s"],
                    stats["median"],
                    label=overlay,
                    color=color,
                    linewidth=2.0,
                    marker="o",
                    markersize=3,
                )
                plotted = True

            if not plotted:
                plt.close(fig)
                continue

            ax.set_title(f"Scenario {scenario.upper()} - {metric}", pad=15)
            ax.set_xlabel("Elapsed (s)")
            
            ylabel = metric
            if metric == "jvm_heap_bytes":
                ylabel = "jvm_heap_mb"
            ax.set_ylabel(ylabel)
            
            ax.set_ylim(bottom=0)
            ax.legend(title="Плагін", loc="upper right", framealpha=0.9)
            
            fig.tight_layout()
            fig.savefig(os.path.join(sc_dir, f"{metric}.png"), dpi=150)
            plt.close(fig)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Clean time-series plots (clean lines only, no CI bands).",
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
        help="Single results/run-<ts> dir: clean plot, 1 line per overlay",
    )
    parser.add_argument(
        "--plots-dir",
        default="",
        help="Output plots directory (default: <base>/plots_clean[_single])",
    )
    args = parser.parse_args()

    single_run = bool(args.run_dir)

    if single_run:
        base_dir = os.path.abspath(args.run_dir)
        if not os.path.isdir(base_dir):
            sys.stderr.write(f"ERROR: not a directory: {base_dir}\n")
            return 1
        if not discover_runs(base_dir):
            sys.stderr.write(
                f"ERROR: no '<overlay>-scenario-<id>-*' subdirs in {base_dir}\n",
            )
            return 1
        print(f"Loading single run {base_dir}...")
        df_raw = load_long_df_from_run(base_dir)
        default_plots = os.path.join(base_dir, "plots_clean_single")
    else:
        if args.input:
            input_path = os.path.abspath(args.input)
            base_dir = os.path.dirname(input_path)
        else:
            base_dir = os.path.abspath(args.aggregate_dir)
            input_path = os.path.join(base_dir, "long_all.csv")
        if not os.path.isfile(input_path):
            sys.stderr.write(f"ERROR: input not found: {input_path}\n")
            return 1
        print(f"Loading {input_path}...")
        df_raw = pd.read_csv(input_path)
        default_plots = os.path.join(base_dir, "plots_clean")

    plots_dir = args.plots_dir or default_plots

    if df_raw.empty or "run" not in df_raw.columns:
        sys.stderr.write("ERROR: no usable data (missing 'run' column)\n")
        return 1

    df_raw = df_raw.dropna(subset=["value"])
    df_raw["value"] = pd.to_numeric(df_raw["value"], errors="coerce")
    df_raw = df_raw.dropna(subset=["value"])

    heap_mask = df_raw["metric"] == "jvm_heap_bytes"
    df_raw.loc[heap_mask, "value"] /= 1024 * 1024

    print("Filtering inactive agent pods...")
    df_clean = filter_active_pods(df_raw)

    print("Collapsing per-pod series to one line per overlay...")
    df_collapsed = collapse_pods_per_run(df_clean)

    clean_name = "long_clean_single.csv" if single_run else "long_clean.csv"
    clean_csv = os.path.join(base_dir, clean_name)
    df_collapsed.to_csv(clean_csv, index=False)

    os.makedirs(plots_dir, exist_ok=True)
    print(f"Generating clean time-series plots (no bands) -> {plots_dir}...")
    
    plot_clean_time_series(df_collapsed, plots_dir)

    print(f"Done. Cleaned CSV: {clean_csv}")
    print(f"Plots: {plots_dir}")
    return 0


if __name__ == "__main__":
    sys.exit(main())