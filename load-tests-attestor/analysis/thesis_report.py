#!/usr/bin/env python3
"""
Статистичний звіт для розділу 4:
порівняння стандартного атестатора SPIRE та JVM-плагіна атестації.

Статистична одиниця:
    один каталог results/run-* = один незалежний експериментальний повтор.

Парність:
    default і custom-jvm з ОДНОГО run-* утворюють одну пару.

Основний опис:
    медіана, Q1, Q3, IQR, min, max.

Основний статистичний критерій:
    двобічний критерій знакових рангів Вілкоксона
    scipy.stats.wilcoxon.

Розмір ефекту:
    matched-pairs rank-biserial correlation:
        r_rb = (W_plus - W_minus) / (W_plus + W_minus)
    де знак визначається як custom-jvm - default.

Додатково:
    Holm-корекція p-значень для множинних перевірок усередині
    всього набору тестів. Початкове p зберігається окремо.

Важливо:
    часові точки всередині одного прогону НЕ є незалежними
    спостереженнями для основного статистичного тесту.
    Для Wilcoxon використовуються тільки скалярні значення
    на рівні незалежного run-*.

Приклад:
    python thesis_report.py --glob "results/run-*"

    python thesis_report.py \
        --glob "results/run-*" \
        --last 13 \
        --out-dir results/thesis-report-final

Залежності:
    pandas
    scipy
    matplotlib
"""

from __future__ import annotations

import argparse
import csv
import glob
import html
import math
import os
import statistics
import sys
from collections import defaultdict
from typing import Iterable

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
from scipy.stats import wilcoxon

from aggregate_runs import (
    PER_SERVICE_METRICS,
    SCENARIOS_CONFIG,
    discover_runs,
    run_label,
    scalar_for_metric,
    scalars_per_service,
)


EXTRA_METRICS = {
    "a": ["svid_issued_total"],
    "b": ["svid_issued_total"],
    "c": [
        "svid_issued_total",
        "success_rate_pct",
        "http_error_count",
    ],
}

METRIC_LABELS_UA = {
    "attestation_avg_ms": "Час атестації (мс)",
    "agent_cpu": "Процесорне навантаження агента SPIRE (ядра)",
    "agent_memory_mb": "Споживання оперативної пам'яті агентом SPIRE (МБ)",
    "server_cpu": "Процесорне навантаження сервера SPIRE (ядра)",
    "server_memory_mb": "Споживання оперативної пам'яті сервером SPIRE (МБ)",
    "http_req_p95_ms": "Затримка HTTP-запитів p95 (мс)",
    "http_req_p99_ms": "Затримка HTTP-запитів p99 (мс)",
    "http_5xx_rate": "Частота HTTP 5xx-помилок (помилок/с)",
    "process_cpu": "Процесорне навантаження застосунку (ядра)",
    "jvm_heap_bytes": "Використання heap JVM (МБ)",
    "jvm_gc_pause_rate": "Частота пауз GC",
    "svid_issued_total": "Кількість виданих SVID",
    "success_rate_pct": "Частка успішних перевірок (%)",
    "http_error_count": "Кількість HTTP-помилок",
}

OVERLAY_LABELS_UA = {
    "default": "Стандартний атестатор",
    "custom-jvm": "Плагін атестації JVM",
}

SCENARIO_LABELS_UA = {
    "a": "Сценарій A",
    "b": "Сценарій B",
    "c": "Сценарій C",
}


def all_metrics_for_scenario(scenario: str) -> list[str]:
    metrics = list(
        SCENARIOS_CONFIG.get(scenario, {}).get("metrics", [])
    )
    for metric in EXTRA_METRICS.get(scenario, []):
        if metric not in metrics:
            metrics.append(metric)
    return metrics


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

    rate = (
        doc.get("metrics", {})
        .get("checks", {})
        .get("values", {})
        .get("rate")
    )

    return (
        float(rate) * 100.0
        if isinstance(rate, (int, float))
        else float("nan")
    )


def http_error_count(subdir: str) -> float:
    doc = read_k6_summary(subdir)
    if not doc:
        return float("nan")

    fails = (
        doc.get("metrics", {})
        .get("http_req_failed", {})
        .get("values", {})
        .get("fails")
    )

    return (
        float(fails)
        if isinstance(fails, (int, float))
        else float("nan")
    )


def svid_issued_total(subdir: str) -> float:
    path = os.path.join(
        subdir,
        "prometheus",
        "svid_issued_total.json",
    )

    # Використовуємо ту саму реалізацію counter delta, що й
    # aggregate_runs / plot_results.
    from plot_results import counter_delta_sum
    return counter_delta_sum(path)


def scalar_for_metric_ext(subdir: str, metric: str) -> float:
    if metric == "success_rate_pct":
        return success_rate_pct(subdir)
    if metric == "http_error_count":
        return http_error_count(subdir)
    if metric == "svid_issued_total":
        return svid_issued_total(subdir)

    return scalar_for_metric(subdir, metric)


def collect_run_scalars(run_dir: str) -> list[dict]:
    """
    Формує один скаляр на:
        (run, scenario, overlay, metric[, service]).

    Для per-service метрик служби НЕ змішуються.
    """
    run_id = run_label(run_dir)
    runs = discover_runs(run_dir)

    rows: list[dict] = []

    for scenario, overlays in sorted(runs.items()):
        if scenario not in SCENARIOS_CONFIG:
            continue

        for metric in all_metrics_for_scenario(scenario):
            for overlay, subdir in sorted(overlays.items()):
                if metric in PER_SERVICE_METRICS:
                    values = scalars_per_service(
                        subdir,
                        metric,
                    )

                    for series, value in sorted(values.items()):
                        if np.isfinite(value):
                            rows.append({
                                "run": run_id,
                                "scenario": scenario,
                                "overlay": overlay,
                                "metric": metric,
                                "series": series,
                                "value": float(value),
                            })
                else:
                    value = scalar_for_metric_ext(
                        subdir,
                        metric,
                    )

                    if np.isfinite(value):
                        rows.append({
                            "run": run_id,
                            "scenario": scenario,
                            "overlay": overlay,
                            "metric": metric,
                            "series": "",
                            "value": float(value),
                        })

    return rows


def resolve_run_dirs(
    run_dirs: list[str],
    glob_pattern: str,
    last: int,
) -> list[str]:
    paths = list(run_dirs)

    if glob_pattern:
        paths.extend(glob.glob(glob_pattern))

    unique = []
    seen = set()

    for path in paths:
        absolute = os.path.abspath(path)

        if not os.path.isdir(absolute):
            continue

        if absolute not in seen:
            seen.add(absolute)
            unique.append(absolute)

    unique.sort()

    if last > 0:
        unique = unique[-last:]

    return unique


def write_csv(
    rows: list[dict],
    fields: list[str],
    path: str,
) -> None:
    with open(path, "w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(
            fh,
            fieldnames=fields,
        )
        writer.writeheader()
        writer.writerows(rows)


def quartiles(values: Iterable[float]) -> tuple[float, float]:
    vals = [float(v) for v in values]

    if not vals:
        return float("nan"), float("nan")

    if len(vals) == 1:
        return vals[0], vals[0]

    q1, q3 = np.percentile(
        np.asarray(vals, dtype=float),
        [25, 75],
        method="linear",
    )

    return float(q1), float(q3)


def compute_descriptive_stats(
    scalar_rows: list[dict],
) -> list[dict]:
    groups: dict[tuple, list[float]] = defaultdict(list)

    for row in scalar_rows:
        key = (
            row["scenario"],
            row["overlay"],
            row["metric"],
            row["series"],
        )
        groups[key].append(float(row["value"]))

    output: list[dict] = []

    for (
        scenario,
        overlay,
        metric,
        series,
    ), values in sorted(groups.items()):

        if not values:
            continue

        q1, q3 = quartiles(values)
        median = float(np.median(values))

        output.append({
            "scenario": scenario,
            "overlay": overlay,
            "metric": metric,
            "series": series,
            "n": len(values),
            "median": median,
            "q1": q1,
            "q3": q3,
            "iqr": q3 - q1,
            "min": float(np.min(values)),
            "max": float(np.max(values)),
        })

    return output


def rank_biserial_from_differences(
    differences: np.ndarray,
) -> tuple[float, float, float]:
    """
    Rank-biserial correlation for paired differences.

    W_plus  = sum of positive signed ranks
    W_minus = sum of negative signed ranks

    r_rb = (W_plus - W_minus) / (W_plus + W_minus)

    Returns:
        W_plus, W_minus, r_rb
    """
    d = np.asarray(differences, dtype=float)
    d = d[np.isfinite(d)]
    d = d[d != 0]

    if d.size == 0:
        return float("nan"), float("nan"), float("nan")

    abs_d = np.abs(d)

    # Average ranks for ties.
    order = np.argsort(abs_d, kind="mergesort")
    sorted_abs = abs_d[order]

    ranks = np.empty(len(d), dtype=float)

    start = 0
    while start < len(d):
        end = start + 1

        while (
            end < len(d)
            and sorted_abs[end] == sorted_abs[start]
        ):
            end += 1

        # Ranks are 1-based.
        average_rank = (
            (start + 1) + end
        ) / 2.0

        ranks[order[start:end]] = average_rank
        start = end

    w_plus = float(ranks[d > 0].sum())
    w_minus = float(ranks[d < 0].sum())

    denominator = w_plus + w_minus

    if denominator == 0:
        r_rb = float("nan")
    else:
        r_rb = (w_plus - w_minus) / denominator

    return w_plus, w_minus, r_rb


def wilcoxon_paired(
    differences: np.ndarray,
) -> dict:
    """
    Виконує двобічний Wilcoxon signed-rank test через SciPy.

    SciPy самостійно визначає коректний метод для конкретної вибірки
    (exact/approx), якщо method='auto'.
    """
    d = np.asarray(differences, dtype=float)
    d = d[np.isfinite(d)]

    n_pairs = int(len(d))
    nonzero = d[d != 0]
    n_nonzero = int(len(nonzero))

    q1, q3 = quartiles(d.tolist())

    result = {
        "n_pairs": n_pairs,
        "n_nonzero": n_nonzero,
        "median_diff": (
            float(np.median(d))
            if n_pairs
            else float("nan")
        ),
        "q1_diff": q1,
        "q3_diff": q3,
        "iqr_diff": (
            q3 - q1
            if n_pairs
            else float("nan")
        ),
        "W": float("nan"),
        "W_plus": float("nan"),
        "W_minus": float("nan"),
        "rank_biserial_r": float("nan"),
        "p": float("nan"),
        "method": "",
    }

    if n_nonzero == 0:
        result["method"] = "all differences are zero"
        return result

    w_plus, w_minus, r_rb = (
        rank_biserial_from_differences(nonzero)
    )

    result["W_plus"] = w_plus
    result["W_minus"] = w_minus
    result["rank_biserial_r"] = r_rb

    try:
        test = wilcoxon(
            nonzero,
            alternative="two-sided",
            zero_method="wilcox",
            correction=False,
            method="auto",
        )

        result["W"] = float(test.statistic)
        result["p"] = float(test.pvalue)

        # SciPy's public result object does not guarantee exposing
        # the internally selected method in every supported version.
        # The exact/approximation decision is therefore reported
        # conservatively from the input characteristics.
        has_ties = (
            len(np.unique(np.abs(nonzero)))
            < len(nonzero)
        )

        if len(nonzero) <= 25 and not has_ties:
            result["method"] = "exact"
        else:
            result["method"] = "normal approximation"

    except ValueError as exc:
        result["method"] = f"failed: {exc}"

    return result


def compute_paired_comparisons(
    scalar_rows: list[dict],
) -> list[dict]:
    """
    Формує пари тільки в межах одного run.

    Для кожної:
        scenario × metric × series

    порівнюється:
        custom-jvm - default.
    """
    indexed: dict[
        tuple,
        dict[str, dict[str, float]]
    ] = defaultdict(dict)

    for row in scalar_rows:
        key = (
            row["scenario"],
            row["metric"],
            row["series"],
        )

        indexed[key].setdefault(
            row["run"],
            {}
        )[row["overlay"]] = float(row["value"])

    output: list[dict] = []

    for (
        scenario,
        metric,
        series,
    ), per_run in sorted(indexed.items()):

        pairs = []

        for run_id, overlays in sorted(per_run.items()):
            if (
                "default" not in overlays
                or "custom-jvm" not in overlays
            ):
                continue

            pairs.append({
                "run": run_id,
                "default": overlays["default"],
                "custom": overlays["custom-jvm"],
                "difference": (
                    overlays["custom-jvm"]
                    - overlays["default"]
                ),
            })

        if not pairs:
            continue

        differences = np.asarray(
            [p["difference"] for p in pairs],
            dtype=float,
        )

        stats = wilcoxon_paired(differences)

        default_values = np.asarray(
            [p["default"] for p in pairs],
            dtype=float,
        )
        custom_values = np.asarray(
            [p["custom"] for p in pairs],
            dtype=float,
        )

        default_median = float(np.median(default_values))
        custom_median = float(np.median(custom_values))

        if default_median != 0:
            delta_pct = (
                (custom_median - default_median)
                / abs(default_median)
                * 100.0
            )
        else:
            delta_pct = float("nan")

        output.append({
            "scenario": scenario,
            "metric": metric,
            "series": series,
            "n_pairs": stats["n_pairs"],
            "n_nonzero": stats["n_nonzero"],
            "default_median": default_median,
            "custom_median": custom_median,
            "median_diff": stats["median_diff"],
            "q1_diff": stats["q1_diff"],
            "q3_diff": stats["q3_diff"],
            "iqr_diff": stats["iqr_diff"],
            "delta_pct": delta_pct,
            "W": stats["W"],
            "W_plus": stats["W_plus"],
            "W_minus": stats["W_minus"],
            "rank_biserial_r": stats["rank_biserial_r"],
            "p": stats["p"],
            "method": stats["method"],
        })

    return output


def holm_adjust(p_values: list[float]) -> list[float]:
    """
    Holm–Bonferroni adjustment.

    NaN values remain NaN.
    """
    p = np.asarray(p_values, dtype=float)
    adjusted = np.full_like(p, np.nan)

    finite_indices = np.flatnonzero(np.isfinite(p))

    if len(finite_indices) == 0:
        return adjusted.tolist()

    order = finite_indices[
        np.argsort(p[finite_indices], kind="mergesort")
    ]

    m = len(order)
    running_max = 0.0

    for rank, idx in enumerate(order):
        candidate = (m - rank) * p[idx]
        running_max = max(running_max, candidate)
        adjusted[idx] = min(running_max, 1.0)

    return adjusted.tolist()


def add_multiple_testing_correction(
    rows: list[dict],
) -> list[dict]:
    """
    Коригує всі скалярні Wilcoxon p-значення одним сімейством тестів.

    Це консервативний варіант для звіту. Сирі p залишаються у стовпці p.
    """
    p_values = [row["p"] for row in rows]
    adjusted = holm_adjust(p_values)

    out = []

    for row, p_adj in zip(rows, adjusted):
        row = dict(row)
        row["p_holm"] = p_adj
        row["significant_raw"] = (
            bool(np.isfinite(row["p"]))
            and row["p"] < 0.05
        )
        row["significant_holm"] = (
            bool(np.isfinite(p_adj))
            and p_adj < 0.05
        )
        out.append(row)

    return out


def _fmt(value, digits: int = 3) -> str:
    if value is None:
        return "—"

    try:
        value = float(value)
    except (TypeError, ValueError):
        return html.escape(str(value))

    if not np.isfinite(value):
        return "—"

    return f"{value:.{digits}f}"


def _fmt_p(value) -> str:
    if value is None:
        return "—"

    try:
        value = float(value)
    except (TypeError, ValueError):
        return "—"

    if not np.isfinite(value):
        return "—"

    if value < 0.001:
        return "<0.001"

    return f"{value:.4f}"


def _metric_label(metric: str, series: str) -> str:
    base = METRIC_LABELS_UA.get(metric, metric)
    return f"{base} [{series}]" if series else base


def build_html(
    descriptive_rows: list[dict],
    paired_rows: list[dict],
    alpha: float = 0.05,
) -> str:
    parts = [
        "<!doctype html>",
        "<html lang='uk'>",
        "<head>",
        "<meta charset='utf-8'>",
        "<title>Статистичний звіт</title>",
        "<style>",
        "body{font-family:'Times New Roman',serif;line-height:1.35;}",
        "table{border-collapse:collapse;width:100%;margin:12px 0 28px;}",
        "th,td{border:1px solid #777;padding:5px;text-align:center;}",
        "th{font-weight:bold;}",
        "td:first-child{text-align:left;}",
        "</style>",
        "</head>",
        "<body>",
        "<h1>Статистичний звіт для розділу 4</h1>",
        "<p>"
        "Основна статистична одиниця — незалежний експериментальний "
        "прогін. Стандартний атестатор і JVM-плагін утворюють пару "
        "в межах одного прогону."
        "</p>",
        "<h2>Описова статистика</h2>",
    ]

    for scenario in ("a", "b", "c"):
        rows = [
            row for row in descriptive_rows
            if row["scenario"] == scenario
        ]

        if not rows:
            continue

        parts.append(
            f"<h3>{SCENARIO_LABELS_UA.get(scenario, scenario)}</h3>"
        )

        parts.append("<table>")
        parts.append(
            "<tr>"
            "<th>Метрика</th>"
            "<th>Атестатор</th>"
            "<th>N</th>"
            "<th>Медіана</th>"
            "<th>Q1</th>"
            "<th>Q3</th>"
            "<th>IQR</th>"
            "<th>Min</th>"
            "<th>Max</th>"
            "</tr>"
        )

        for row in sorted(
            rows,
            key=lambda r: (
                r["metric"],
                r["series"],
                r["overlay"],
            ),
        ):
            parts.append(
                "<tr>"
                f"<td>{html.escape(_metric_label(row['metric'], row['series']))}</td>"
                f"<td>{html.escape(OVERLAY_LABELS_UA.get(row['overlay'], row['overlay']))}</td>"
                f"<td>{row['n']}</td>"
                f"<td>{_fmt(row['median'])}</td>"
                f"<td>{_fmt(row['q1'])}</td>"
                f"<td>{_fmt(row['q3'])}</td>"
                f"<td>{_fmt(row['iqr'])}</td>"
                f"<td>{_fmt(row['min'])}</td>"
                f"<td>{_fmt(row['max'])}</td>"
                "</tr>"
            )

        parts.append("</table>")

    parts.extend([
        "<h2>Парне порівняння</h2>",
        "<p>"
        "Критерій знакових рангів Вілкоксона: двобічна перевірка; "
        f"рівень значущості α = {alpha:g}. "
        "Напрямок різниці визначено як custom-jvm − default."
        "</p>",
    ])

    for scenario in ("a", "b", "c"):
        rows = [
            row for row in paired_rows
            if row["scenario"] == scenario
        ]

        if not rows:
            continue

        parts.append(
            f"<h3>{SCENARIO_LABELS_UA.get(scenario, scenario)}</h3>"
        )

        parts.append("<table>")
        parts.append(
            "<tr>"
            "<th>Метрика</th>"
            "<th>N пар</th>"
            "<th>Медіана default</th>"
            "<th>Медіана custom-jvm</th>"
            "<th>Δ медіан</th>"
            "<th>Δ, %</th>"
            "<th>W</th>"
            "<th>p</th>"
            "<th>p<sub>Holm</sub></th>"
            "<th>r<sub>rb</sub></th>"
            "<th>Рішення</th>"
            "</tr>"
        )

        for row in sorted(
            rows,
            key=lambda r: (
                r["metric"],
                r["series"],
            ),
        ):
            if row["significant_holm"]:
                decision = "статистично значуща різниця"
            else:
                decision = "статистично значущої різниці не виявлено"

            parts.append(
                "<tr>"
                f"<td>{html.escape(_metric_label(row['metric'], row['series']))}</td>"
                f"<td>{row['n_pairs']}</td>"
                f"<td>{_fmt(row['default_median'])}</td>"
                f"<td>{_fmt(row['custom_median'])}</td>"
                f"<td>{_fmt(row['median_diff'])}</td>"
                f"<td>{_fmt(row['delta_pct'], 1)}%</td>"
                f"<td>{_fmt(row['W'], 1)}</td>"
                f"<td>{_fmt_p(row['p'])}</td>"
                f"<td>{_fmt_p(row['p_holm'])}</td>"
                f"<td>{_fmt(row['rank_biserial_r'], 3)}</td>"
                f"<td>{decision}</td>"
                "</tr>"
            )

        parts.append("</table>")

    parts.extend([
        "<h2>Примітка щодо статистичної інтерпретації</h2>",
        "<p>"
        "Часові точки всередині одного експериментального прогону "
        "не трактуються як незалежні спостереження. Вони можуть бути "
        "автокорельованими, тому для основного висновку використовуються "
        "лише значення, агреговані на рівні незалежних прогонів."
        "</p>",
        "</body>",
        "</html>",
    ])

    return "\n".join(parts)


def plot_boxplots(
    descriptive_rows: list[dict],
    scalar_rows: list[dict],
    plots_dir: str,
) -> None:
    """
    Boxplot + усі окремі run-значення.

    Boxplot не використовується як статистичний тест.
    Вуса — стандартні 1.5 IQR; крайні значення не обрізаються з даних.
    """
    os.makedirs(plots_dir, exist_ok=True)

    groups: dict[
        tuple,
        dict[str, list[float]]
    ] = defaultdict(lambda: defaultdict(list))

    for row in scalar_rows:
        key = (
            row["scenario"],
            row["metric"],
            row["series"],
        )
        groups[key][row["overlay"]].append(
            float(row["value"])
        )

    for (
        scenario,
        metric,
        series,
    ), by_overlay in sorted(groups.items()):

        overlays = [
            overlay
            for overlay in ("default", "custom-jvm")
            if by_overlay.get(overlay)
        ]

        if not overlays:
            continue

        data = [
            by_overlay[overlay]
            for overlay in overlays
        ]

        fig, ax = plt.subplots(figsize=(6.5, 5))

        ax.boxplot(
            data,
            tick_labels=[
                OVERLAY_LABELS_UA.get(o, o)
                for o in overlays
            ],
            showmeans=False,
            whis=1.5,
        )

        # Окремі run-значення показуємо, щоб не приховувати малий N.
        for x, values in enumerate(data, start=1):
            jitter = np.linspace(
                -0.055,
                0.055,
                num=len(values),
            ) if len(values) > 1 else np.array([0.0])

            ax.scatter(
                np.full(len(values), x) + jitter,
                values,
                s=18,
                alpha=0.7,
                zorder=3,
            )

        ax.set_title(
            f"{SCENARIO_LABELS_UA.get(scenario, scenario)}: "
            f"{_metric_label(metric, series)}"
        )
        ax.set_ylabel(
            METRIC_LABELS_UA.get(metric, metric)
        )
        ax.grid(True, axis="y", alpha=0.25)

        fig.tight_layout()

        suffix = f"-{series}" if series else ""
        safe_suffix = "".join(
            c if c.isalnum() or c in "-_" else "_"
            for c in suffix
        )

        fig.savefig(
            os.path.join(
                plots_dir,
                f"box-{scenario}-{metric}{safe_suffix}.png",
            ),
            dpi=300,
            bbox_inches="tight",
        )
        plt.close(fig)


def plot_paired_differences(
    paired_rows: list[dict],
    scalar_rows: list[dict],
    plots_dir: str,
) -> None:
    """
    Парні різниці custom-jvm - default по кожному незалежному run.

    Цей графік відображає саме пари, на яких виконується Wilcoxon.
    """
    os.makedirs(plots_dir, exist_ok=True)

    indexed: dict[
        tuple,
        dict[str, dict[str, float]]
    ] = defaultdict(dict)

    for row in scalar_rows:
        key = (
            row["scenario"],
            row["metric"],
            row["series"],
        )
        indexed[key].setdefault(
            row["run"],
            {}
        )[row["overlay"]] = float(row["value"])

    for key, per_run in sorted(indexed.items()):
        scenario, metric, series = key

        pairs = []

        for run_id in sorted(per_run):
            values = per_run[run_id]

            if (
                "default" not in values
                or "custom-jvm" not in values
            ):
                continue

            pairs.append(
                (
                    run_id,
                    values["custom-jvm"]
                    - values["default"],
                )
            )

        if len(pairs) < 2:
            continue

        labels = [
            run_id.replace("run-", "")
            for run_id, _ in pairs
        ]
        differences = np.asarray(
            [difference for _, difference in pairs]
        )

        fig, ax = plt.subplots(figsize=(8, 4.8))

        x = np.arange(len(differences))
        ax.axhline(
            0,
            linewidth=1,
            linestyle="--",
        )

        ax.scatter(
            x,
            differences,
            s=28,
            zorder=3,
        )

        median = float(np.median(differences))
        ax.axhline(
            median,
            linewidth=1.2,
            label=f"медіана Δ = {median:.3f}",
        )

        ax.set_xticks(x)
        ax.set_xticklabels(
            labels,
            rotation=45,
            ha="right",
        )

        ax.set_title(
            f"{SCENARIO_LABELS_UA.get(scenario, scenario)}: "
            f"парні різниці, "
            f"{_metric_label(metric, series)}"
        )
        ax.set_xlabel("Експериментальний прогін")
        ax.set_ylabel(
            "Δ = custom-jvm − default"
        )
        ax.grid(True, axis="y", alpha=0.25)
        ax.legend(frameon=False)

        fig.tight_layout()

        suffix = f"-{series}" if series else ""
        safe_suffix = "".join(
            c if c.isalnum() or c in "-_" else "_"
            for c in suffix
        )

        fig.savefig(
            os.path.join(
                plots_dir,
                f"delta-{scenario}-{metric}{safe_suffix}.png",
            ),
            dpi=300,
            bbox_inches="tight",
        )
        plt.close(fig)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Статистичний звіт для розділу 4."
    )

    parser.add_argument(
        "run_dirs",
        nargs="*",
        help="Явно задані results/run-*.",
    )
    parser.add_argument(
        "--glob",
        default="",
        help="Glob, наприклад results/run-*.",
    )
    parser.add_argument(
        "--last",
        type=int,
        default=0,
        help="Взяти N останніх прогонів після сортування.",
    )
    parser.add_argument(
        "--out-dir",
        default="",
        help="Каталог вихідного звіту.",
    )
    parser.add_argument(
        "--alpha",
        type=float,
        default=0.05,
        help="Рівень значущості (default: 0.05).",
    )

    args = parser.parse_args()

    if not (0 < args.alpha < 1):
        parser.error("alpha має бути між 0 і 1.")

    run_dirs = resolve_run_dirs(
        args.run_dirs,
        args.glob,
        args.last,
    )

    if not run_dirs:
        sys.stderr.write(
            "ERROR: не знайдено жодного results/run-*.\n"
        )
        return 1

    try:
        scalar_rows: list[dict] = []

        for run_dir in run_dirs:
            print(f"Обробка: {run_dir}")
            scalar_rows.extend(
                collect_run_scalars(run_dir)
            )

        if not scalar_rows:
            raise ValueError(
                "Не вдалося отримати жодного скалярного показника."
            )

        descriptive_rows = compute_descriptive_stats(
            scalar_rows
        )

        paired_rows = compute_paired_comparisons(
            scalar_rows
        )

        paired_rows = add_multiple_testing_correction(
            paired_rows
        )

        if args.out_dir:
            out_dir = os.path.abspath(args.out_dir)
        else:
            from datetime import datetime, timezone
            timestamp = datetime.now(
                timezone.utc
            ).strftime("%Y%m%d-%H%M%S")
            out_dir = os.path.join(
                os.path.dirname(run_dirs[0]),
                f"thesis-report-{timestamp}",
            )

        os.makedirs(out_dir, exist_ok=True)

        write_csv(
            scalar_rows,
            [
                "run",
                "scenario",
                "overlay",
                "metric",
                "series",
                "value",
            ],
            os.path.join(
                out_dir,
                "run_scalars.csv",
            ),
        )

        write_csv(
            descriptive_rows,
            [
                "scenario",
                "overlay",
                "metric",
                "series",
                "n",
                "median",
                "q1",
                "q3",
                "iqr",
                "min",
                "max",
            ],
            os.path.join(
                out_dir,
                "descriptive_stats.csv",
            ),
        )

        write_csv(
            paired_rows,
            [
                "scenario",
                "metric",
                "series",
                "n_pairs",
                "n_nonzero",
                "default_median",
                "custom_median",
                "median_diff",
                "q1_diff",
                "q3_diff",
                "iqr_diff",
                "delta_pct",
                "W",
                "W_plus",
                "W_minus",
                "rank_biserial_r",
                "p",
                "p_holm",
                "significant_raw",
                "significant_holm",
                "method",
            ],
            os.path.join(
                out_dir,
                "paired_comparison.csv",
            ),
        )

        html_report = build_html(
            descriptive_rows,
            paired_rows,
            alpha=args.alpha,
        )

        with open(
            os.path.join(out_dir, "thesis_tables.html"),
            "w",
            encoding="utf-8",
        ) as fh:
            fh.write(html_report)

        plots_dir = os.path.join(
            out_dir,
            "plots",
        )

        plot_boxplots(
            descriptive_rows,
            scalar_rows,
            plots_dir,
        )

        plot_paired_differences(
            paired_rows,
            scalar_rows,
            plots_dir,
        )

        print()
        print("Готово.")
        print(f"Прогонів: {len(run_dirs)}")
        print(f"Скалярних спостережень: {len(scalar_rows)}")
        print(f"Парних тестів: {len(paired_rows)}")
        print(f"Звіт: {out_dir}")

        return 0

    except (OSError, ValueError, KeyError) as exc:
        sys.stderr.write(
            f"ERROR: {exc}\n"
        )
        return 1


if __name__ == "__main__":
    sys.exit(main())
