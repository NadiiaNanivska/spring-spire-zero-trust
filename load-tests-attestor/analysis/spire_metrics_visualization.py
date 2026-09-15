#!/usr/bin/env python3
"""
Побудова часових рядів метрик SPIRE/JVM для експериментів.

Призначення:
- не виконує статистичний тест;
- готує чисті часові графіки для ілюстрації динаміки метрик;
- для кількох прогонів показує медіанний часовий ряд між незалежними прогонами;
- для агентських метрик враховує лише pod-и, що були активними наприкінці вікна;
- не використовує seaborn;
- не використовує довірчі інтервали як заміну статистичному аналізу.

Джерела:
1) готовий long_all.csv з aggregate_runs.py;
2) або один results/run-* безпосередньо.

Приклад:
    python spire_metrics_visualization.py \
        --input results/aggregate-.../long_all.csv

    python spire_metrics_visualization.py \
        --run-dir results/run-20260905-191950

Вихід:
    plots_clean/
        scenario-a/
        scenario-b/
        scenario-c/
    long_clean.csv або long_clean_single.csv
"""

from __future__ import annotations

import argparse
import os
import sys
from typing import Iterable

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import pandas as pd

from aggregate_runs import run_label
from plot_results import collect_long_rows, discover_runs


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

METRIC_LABELS = {
    "attestation_avg_ms": "Час атестації (мс)",
    "agent_cpu": "Процесорне навантаження агента SPIRE (ядра)",
    "agent_memory_mb": "Споживання оперативної пам'яті агентом SPIRE (МБ)",
    "server_cpu": "Процесорне навантаження сервера SPIRE (ядра)",
    "server_memory_mb": "Споживання оперативної пам'яті сервером SPIRE (МБ)",
    "http_req_p95_ms": "Затримка HTTP-запитів, p95 (мс)",
    "http_req_p99_ms": "Затримка HTTP-запитів, p99 (мс)",
    "http_req_rate": "Інтенсивність HTTP-запитів (запитів/с)",
    "http_5xx_rate": "Частота HTTP 5xx-помилок (помилок/с)",
    "svid_issued_rate": "Інтенсивність видачі SVID (SVID/с)",
    "jvm_gc_pause_rate": "Частота пауз GC",
    "jvm_heap_bytes": "Використання heap JVM (МБ)",
}

SCENARIO_LABELS = {
    "a": "Сценарій A",
    "b": "Сценарій B",
    "c": "Сценарій C",
}

OVERLAY_LABELS = {
    "default": "Стандартний атестатор SPIRE",
    "custom-jvm": "Плагін атестації JVM",
}

PER_SERVICE_METRICS = {
    "http_req_p95_ms",
    "http_req_p99_ms",
    "jvm_heap_bytes",
}

AGENT_METRICS = {
    "agent_cpu",
    "agent_memory_mb",
}



def metric_label(metric: str, series: str = "") -> str:
    label = METRIC_LABELS.get(metric, metric)
    if series:
        return f"{label} — {series}"
    return label


def overlay_label(overlay: str) -> str:
    return OVERLAY_LABELS.get(overlay, overlay)


def load_long_df_from_run(run_dir: str) -> pd.DataFrame:
    runs = discover_runs(run_dir)
    if not runs:
        raise ValueError(
            f"У {run_dir} не знайдено підкаталогів "
            "'<overlay>-scenario-<id>-<timestamp>'."
        )

    rows = collect_long_rows(runs)
    run_id = run_label(run_dir)
    for row in rows:
        row["run"] = run_id

    return pd.DataFrame(rows)


def validate_long_df(df: pd.DataFrame) -> pd.DataFrame:
    required = {"run", "scenario", "overlay", "metric", "elapsed_s", "value"}
    missing = required - set(df.columns)
    if missing:
        raise ValueError(
            f"У вхідних даних відсутні обов'язкові стовпці: "
            f"{', '.join(sorted(missing))}"
        )

    out = df.copy()

    out["elapsed_s"] = pd.to_numeric(out["elapsed_s"], errors="coerce")
    out["value"] = pd.to_numeric(out["value"], errors="coerce")

    out = out.dropna(subset=["elapsed_s", "value"])
    out = out[out["elapsed_s"] >= 0]

    heap_mask = out["metric"].eq("jvm_heap_bytes")
    out.loc[heap_mask, "value"] /= 1024.0 ** 2

    return out


def filter_active_agent_pods(df: pd.DataFrame, tail_seconds: int = 30) -> pd.DataFrame:
    """
    Для agent_cpu / agent_memory_mb залишає лише часові ряди pod-ів,
    які ще передавали дані в останні tail_seconds відносно завершення
    відповідного (run, scenario, overlay, metric) вікна.

    Інші метрики не змінюються.
    """
    agent = df[df["metric"].isin(AGENT_METRICS)].copy()
    other = df[~df["metric"].isin(AGENT_METRICS)].copy()

    if agent.empty:
        return df.copy()

    group_keys = [
        "run", "scenario", "overlay", "metric", "series"
    ]

    per_series_end = (
        agent.groupby(group_keys, dropna=False)["elapsed_s"]
        .max()
        .reset_index(name="series_end_s")
    )

    global_end = (
        agent.groupby(
            ["run", "scenario", "overlay", "metric"],
            dropna=False
        )["elapsed_s"]
        .max()
        .reset_index(name="global_end_s")
    )

    active = per_series_end.merge(
        global_end,
        on=["run", "scenario", "overlay", "metric"],
        how="inner",
    )

    active = active[
        active["series_end_s"] >= active["global_end_s"] - tail_seconds
    ]

    agent = agent.merge(
        active[group_keys].drop_duplicates(),
        on=group_keys,
        how="inner",
    )

    return pd.concat([agent, other], ignore_index=True)


def collapse_agent_pods(df: pd.DataFrame) -> pd.DataFrame:
    """
    Один ряд на (run, scenario, overlay, metric, elapsed_s):
    середнє між активними agent pod-ами.

    Це агрегація всередині одного моменту часу, а не статистична
    агрегація між незалежними експериментальними прогонами.
    """
    agent = df[df["metric"].isin(AGENT_METRICS)].copy()
    other = df[~df["metric"].isin(AGENT_METRICS)].copy()

    if agent.empty:
        return df.copy()

    keys = ["run", "scenario", "overlay", "metric", "elapsed_s"]

    agent = (
        agent.groupby(keys, as_index=False)["value"]
        .mean()
    )

    return pd.concat([agent, other], ignore_index=True)


def clean_long_df(
    df: pd.DataFrame,
    bucket_seconds: int = 5,
    active_tail_seconds: int = 30,
) -> pd.DataFrame:
    df = validate_long_df(df)
    df = filter_active_agent_pods(df, active_tail_seconds)
    df = collapse_agent_pods(df)

    if bucket_seconds <= 0:
        raise ValueError("bucket_seconds має бути > 0.")

    df["elapsed_s"] = (
        np.floor(df["elapsed_s"] / bucket_seconds) * bucket_seconds
    ).astype(int)

    keys = [
        "run", "scenario", "overlay", "metric", "series", "elapsed_s"
    ]
    df = (
        df.groupby(keys, dropna=False, as_index=False)["value"]
        .mean()
    )

    return df.sort_values(
        ["scenario", "metric", "series", "overlay", "run", "elapsed_s"]
    ).reset_index(drop=True)


def _series_values(
    df: pd.DataFrame,
    scenario: str,
    metric: str,
    series: str,
    overlay: str,
) -> pd.DataFrame:
    mask = (
        df["scenario"].eq(scenario)
        & df["metric"].eq(metric)
        & df["overlay"].eq(overlay)
    )

    if series:
        mask &= df["series"].fillna("").eq(series)
    else:
        mask &= df["series"].fillna("").eq("")

    return df.loc[mask].copy()


def median_across_runs(
    df: pd.DataFrame,
    scenario: str,
    metric: str,
    series: str,
    overlay: str,
) -> pd.DataFrame:
    """
    Для кожного elapsed_s обчислює медіану значень між незалежними
    експериментальними прогонами.

    Це лише спосіб візуалізації динаміки; ці точки не використовуються
    як незалежні спостереження для критерію Вілкоксона.
    """
    subset = _series_values(df, scenario, metric, series, overlay)

    if subset.empty:
        return pd.DataFrame(columns=["elapsed_s", "value", "n_runs"])

    grouped = (
        subset.groupby("elapsed_s")["value"]
        .agg(
            value="median",
            n_runs="count",
        )
        .reset_index()
    )

    return grouped.sort_values("elapsed_s")


def available_series(
    df: pd.DataFrame,
    scenario: str,
    metric: str,
) -> list[str]:
    subset = df[
        df["scenario"].eq(scenario)
        & df["metric"].eq(metric)
    ]

    series = (
        subset["series"]
        .fillna("")
        .astype(str)
        .drop_duplicates()
        .tolist()
    )

    return sorted(series, key=lambda x: (x != "", x))


def plot_time_series(
    df: pd.DataFrame,
    plots_dir: str,
    min_runs_for_point: int = 1,
) -> None:
    os.makedirs(plots_dir, exist_ok=True)

    scenarios = sorted(df["scenario"].dropna().unique())

    for scenario in scenarios:
        scenario_dir = os.path.join(
            plots_dir, f"scenario-{scenario}"
        )
        os.makedirs(scenario_dir, exist_ok=True)

        metrics = [
            m for m in TIME_SERIES_METRICS
            if m in set(df.loc[df["scenario"].eq(scenario), "metric"])
        ]

        for metric in metrics:
            for series in available_series(df, scenario, metric):
                fig, ax = plt.subplots(figsize=(9, 5.2))
                plotted = False

                for overlay in ("default", "custom-jvm"):
                    curve = median_across_runs(
                        df,
                        scenario,
                        metric,
                        series,
                        overlay,
                    )

                    if curve.empty:
                        continue

                    curve = curve[
                        curve["n_runs"] >= min_runs_for_point
                    ]

                    if curve.empty:
                        continue

                    ax.plot(
                        curve["elapsed_s"],
                        curve["value"],
                        label=overlay_label(overlay),
                        linewidth=1.8,
                    )
                    plotted = True

                if not plotted:
                    plt.close(fig)
                    continue

                ax.set_title(
                    f"{SCENARIO_LABELS.get(scenario, scenario)}: "
                    f"{metric_label(metric, series)}"
                )
                ax.set_xlabel("Час від початку прогону (с)")
                ax.set_ylabel(metric_label(metric, series))
                ax.grid(True, axis="y", alpha=0.25)
                ax.legend(frameon=False)
                ax.margins(x=0.01)

                if metric in {
                    "agent_cpu",
                    "agent_memory_mb",
                    "server_cpu",
                    "server_memory_mb",
                    "http_req_rate",
                    "http_5xx_rate",
                    "svid_issued_rate",
                    "jvm_gc_pause_rate",
                    "jvm_heap_bytes",
                }:
                    ax.set_ylim(bottom=0)

                fig.tight_layout()

                suffix = (
                    f"-{series}" if series else ""
                )
                safe_suffix = "".join(
                    c if c.isalnum() or c in "-_" else "_"
                    for c in suffix
                )

                path = os.path.join(
                    scenario_dir,
                    f"{metric}{safe_suffix}.png",
                )

                fig.savefig(
                    path,
                    dpi=300,
                    bbox_inches="tight",
                )
                plt.close(fig)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Побудова чистих часових рядів метрик SPIRE/JVM."
    )
    parser.add_argument(
        "aggregate_dir",
        nargs="?",
        default=".",
        help="Каталог з long_all.csv.",
    )
    parser.add_argument(
        "--input",
        default="",
        help="Шлях до long_all.csv. Має пріоритет над aggregate_dir.",
    )
    parser.add_argument(
        "--run-dir",
        default="",
        help="Окремий results/run-* для побудови графіків одного прогону.",
    )
    parser.add_argument(
        "--plots-dir",
        default="",
        help="Каталог для PNG-графіків.",
    )
    parser.add_argument(
        "--bucket-seconds",
        type=int,
        default=5,
        help="Ширина часового бакета в секундах (default: 5).",
    )
    parser.add_argument(
        "--active-tail-seconds",
        type=int,
        default=30,
        help="Допустиме запізнення останнього sample agent pod-а (default: 30).",
    )
    parser.add_argument(
        "--min-runs-for-point",
        type=int,
        default=1,
        help="Мінімальна кількість прогонів для точки медіанного ряду.",
    )

    args = parser.parse_args()

    try:
        if args.run_dir:
            run_dir = os.path.abspath(args.run_dir)
            if not os.path.isdir(run_dir):
                raise ValueError(f"Каталог не існує: {run_dir}")

            df_raw = load_long_df_from_run(run_dir)
            base_dir = run_dir
            default_plots = os.path.join(
                base_dir, "plots_clean_single"
            )
            output_csv = os.path.join(
                base_dir, "long_clean_single.csv"
            )
        else:
            if args.input:
                input_path = os.path.abspath(args.input)
                base_dir = os.path.dirname(input_path)
            else:
                base_dir = os.path.abspath(args.aggregate_dir)
                input_path = os.path.join(base_dir, "long_all.csv")

            if not os.path.isfile(input_path):
                raise ValueError(f"Файл не знайдено: {input_path}")

            df_raw = pd.read_csv(input_path)
            default_plots = os.path.join(
                base_dir, "plots_clean"
            )
            output_csv = os.path.join(
                base_dir, "long_clean.csv"
            )

        if df_raw.empty:
            raise ValueError("Вхідний набір даних порожній.")

        df_clean = clean_long_df(
            df_raw,
            bucket_seconds=args.bucket_seconds,
            active_tail_seconds=args.active_tail_seconds,
        )

        df_clean.to_csv(output_csv, index=False)

        plots_dir = (
            os.path.abspath(args.plots_dir)
            if args.plots_dir
            else default_plots
        )

        plot_time_series(
            df_clean,
            plots_dir,
            min_runs_for_point=args.min_runs_for_point,
        )

        print(f"Готово.")
        print(f"Очищені часові ряди: {output_csv}")
        print(f"Графіки: {plots_dir}")
        return 0

    except (OSError, ValueError, KeyError) as exc:
        sys.stderr.write(f"ERROR: {exc}\n")
        return 1


if __name__ == "__main__":
    sys.exit(main())
