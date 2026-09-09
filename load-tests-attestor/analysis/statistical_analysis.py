#!/usr/bin/env python3

from __future__ import annotations

import argparse
from pathlib import Path

import numpy as np
import pandas as pd
from scipy.stats import wilcoxon


OVERLAYS = ["default", "custom-jvm"]

ALPHA = 0.05


def descriptive(values):
    x = np.asarray(values, dtype=float)
    x = x[np.isfinite(x)]

    if len(x) == 0:
        return {
            "n": 0,
            "median": np.nan,
            "q1": np.nan,
            "q3": np.nan,
            "iqr": np.nan,
            "min": np.nan,
            "max": np.nan,
        }

    q1, median, q3 = np.percentile(
        x,
        [25, 50, 75],
        method="linear",
    )

    return {
        "n": len(x),
        "median": median,
        "q1": q1,
        "q3": q3,
        "iqr": q3 - q1,
        "min": np.min(x),
        "max": np.max(x),
    }


def rank_biserial(differences):
    """
    Rank-biserial correlation for paired samples.

    Positive:
        custom > default

    Negative:
        custom < default
    """

    d = np.asarray(differences, dtype=float)
    d = d[np.isfinite(d)]

    d = d[d != 0]

    if len(d) == 0:
        return 0.0

    abs_d = np.abs(d)

    ranks = pd.Series(abs_d).rank(
        method="average"
    ).to_numpy()

    positive = ranks[d > 0].sum()
    negative = ranks[d < 0].sum()

    denominator = positive + negative

    if denominator == 0:
        return 0.0

    return (positive - negative) / denominator


def wilcoxon_test(default_values, custom_values):
    """
    Proper paired Wilcoxon signed-rank test.
    """

    default_values = np.asarray(
        default_values,
        dtype=float,
    )

    custom_values = np.asarray(
        custom_values,
        dtype=float,
    )

    mask = (
        np.isfinite(default_values)
        & np.isfinite(custom_values)
    )

    default_values = default_values[mask]
    custom_values = custom_values[mask]

    n = len(default_values)

    if n < 2:
        return {
            "paired_n": n,
            "wilcoxon_statistic": np.nan,
            "wilcoxon_p": np.nan,
            "effect_size": np.nan,
            "test_method": "not_available",
            "significant": False,
        }

    differences = (
        custom_values
        - default_values
    )

    if np.allclose(differences, 0):
        return {
            "paired_n": n,
            "wilcoxon_statistic": 0.0,
            "wilcoxon_p": 1.0,
            "effect_size": 0.0,
            "test_method": "all_zero",
            "significant": False,
        }

    non_zero = differences[differences != 0]

    # Exact calculation is appropriate for small samples
    # when there are no ties / zero differences.
    exact_possible = (
        len(non_zero) <= 25
        and len(np.unique(np.abs(non_zero)))
        == len(non_zero)
    )

    method = "exact" if exact_possible else "approx"

    try:
        result = wilcoxon(
            default_values,
            custom_values,
            alternative="two-sided",
            zero_method="wilcox",
            method=method,
        )
    except TypeError:
        # Compatibility with older SciPy.
        result = wilcoxon(
            default_values,
            custom_values,
            alternative="two-sided",
            zero_method="wilcox",
            mode=method,
        )

    effect = rank_biserial(differences)

    return {
        "paired_n": n,
        "wilcoxon_statistic": result.statistic,
        "wilcoxon_p": result.pvalue,
        "effect_size": effect,
        "test_method": method,
        "significant": result.pvalue < ALPHA,
    }


def pair_runs(df):
    """
    Pair runs using explicit pair_id if available.

    Expected future structure:

        pair_id | scenario | overlay
        A01     | a        | default
        A01     | a        | custom-jvm
        A02     | a        | default
        A02     | a        | custom-jvm
        ...

    If pair_id does not exist, infer it from run_id.

    IMPORTANT:
    For the thesis, explicit pair_id is strongly recommended.
    """

    df = df.copy()

    if "pair_id" in df.columns:
        return df

    # Expected run_id:
    #
    # 20260908-A01-default
    # 20260908-A01-custom
    #
    # or:
    #
    # A01-default
    # A01-custom

    df["pair_id"] = (
        df["run_id"]
        .astype(str)
        .str.replace(
            r"-(default|custom-jvm)$",
            "",
            regex=True,
        )
    )

    return df


def paired_run_analysis(df):
    df = pair_runs(df)

    results = []

    grouping = [
        "scenario",
        "metric",
        "service",
    ]

    for keys, group in df.groupby(grouping):

        scenario, metric, service = keys

        default = group[
            group["overlay"] == "default"
        ][
            ["pair_id", "value"]
        ].rename(
            columns={"value": "default_value"}
        )

        custom = group[
            group["overlay"] == "custom-jvm"
        ][
            ["pair_id", "value"]
        ].rename(
            columns={"value": "custom_value"}
        )

        paired = default.merge(
            custom,
            on="pair_id",
            how="inner",
        )

        if paired.empty:
            continue

        test = wilcoxon_test(
            paired["default_value"],
            paired["custom_value"],
        )

        default_stats = descriptive(
            paired["default_value"]
        )

        custom_stats = descriptive(
            paired["custom_value"]
        )

        default_median = default_stats["median"]
        custom_median = custom_stats["median"]

        if (
            np.isfinite(default_median)
            and default_median != 0
            and np.isfinite(custom_median)
        ):
            delta_pct = (
                (custom_median - default_median)
                / default_median
                * 100
            )
        else:
            delta_pct = np.nan

        results.append({
            "scenario": scenario,
            "metric": metric,
            "service": service,

            "default_n": default_stats["n"],
            "default_median": default_stats["median"],
            "default_q1": default_stats["q1"],
            "default_q3": default_stats["q3"],
            "default_iqr": default_stats["iqr"],
            "default_min": default_stats["min"],
            "default_max": default_stats["max"],

            "custom_n": custom_stats["n"],
            "custom_median": custom_stats["median"],
            "custom_q1": custom_stats["q1"],
            "custom_q3": custom_stats["q3"],
            "custom_iqr": custom_stats["iqr"],
            "custom_min": custom_stats["min"],
            "custom_max": custom_stats["max"],

            "delta_pct": delta_pct,

            **test,
        })

    return pd.DataFrame(results)


def main():

    parser = argparse.ArgumentParser()

    parser.add_argument(
        "input",
        type=Path,
    )

    parser.add_argument(
        "--output",
        type=Path,
        default=Path(
            "analysis/statistical_results.csv"
        ),
    )

    args = parser.parse_args()

    df = pd.read_csv(args.input)

    # Use one representative value per run.
    #
    # Prometheus:
    #     median of time series
    #
    # k6:
    #     run summary
    #
    # attestation:
    #     mean over attestation events

    df = df[
        df["statistic"].isin(
            [
                "median",
                "run_summary",
                "mean_over_attestation_events",
            ]
        )
    ].copy()

    # Plugin-only metrics cannot be compared with default,
    # because the default attestor does not produce the
    # same JVM-specific measurements.

    df = df[
        ~df["metric"].str.startswith("plugin_")
    ].copy()

    result = paired_run_analysis(df)

    args.output.parent.mkdir(
        parents=True,
        exist_ok=True,
    )

    result.to_csv(
        args.output,
        index=False,
    )

    print(
        f"Saved statistical analysis to:\n"
        f"{args.output}"
    )

    print(
        f"\nRows: {len(result)}"
    )


if __name__ == "__main__":
    main()