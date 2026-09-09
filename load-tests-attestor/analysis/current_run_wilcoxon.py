#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
from pathlib import Path

import numpy as np
import pandas as pd
from scipy.stats import wilcoxon


PROM_FILES = {
    "agent_cpu_cores": "agent_cpu.json",
    "agent_memory_mb": "agent_memory_mb.json",
    "server_cpu_cores": "server_cpu.json",
    "server_memory_mb": "server_memory_mb.json",
    "http_p95_ms": "http_req_p95_ms.json",
    "http_p99_ms": "http_req_p99_ms.json",
    "http_5xx_rate": "http_5xx_rate.json",
}


def read_prometheus(path):

    with path.open(
        "r",
        encoding="utf-8",
    ) as f:
        data = json.load(f)

    result = []

    for series in data.get(
        "data",
        {}
    ).get(
        "result",
        []
    ):

        labels = series.get(
            "metric",
            {}
        )

        service = (
            labels.get("service")
            or labels.get("pod")
            or labels.get("instance")
            or "aggregate"
        )

        for timestamp, value in series.get(
            "values",
            [],
        ):

            try:
                value = float(value)
            except (
                TypeError,
                ValueError,
            ):
                continue

            result.append({
                "timestamp": float(timestamp),
                "service": service,
                "value": value,
            })

    return pd.DataFrame(result)


def normalize_elapsed(df):

    if df.empty:
        return df

    df = df.copy()

    df["timestamp"] = pd.to_numeric(
        df["timestamp"]
    )

    df["elapsed_s"] = (
        df["timestamp"]
        - df["timestamp"].min()
    )

    # Prometheus data are sampled every 5 seconds.
    #
    # Round to avoid floating-point timestamp mismatch.
    df["elapsed_s"] = (
        df["elapsed_s"]
        .round()
        .astype(int)
    )

    return df


def pair_metric(
    default_path,
    custom_path,
):

    default = normalize_elapsed(
        read_prometheus(default_path)
    )

    custom = normalize_elapsed(
        read_prometheus(custom_path)
    )

    if default.empty or custom.empty:
        return pd.DataFrame()

    merged = default.merge(
        custom,
        on=[
            "elapsed_s",
            "service",
        ],
        suffixes=(
            "_default",
            "_custom",
        ),
    )

    return merged


def run_wilcoxon(
    default_values,
    custom_values,
):

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

    differences = (
        custom_values
        - default_values
    )

    non_zero = differences[
        differences != 0
    ]

    if len(non_zero) < 2:
        return {
            "n": len(non_zero),
            "W": np.nan,
            "p": np.nan,
            "method": "insufficient",
        }

    # Current-run data normally contain many observations,
    # so exact Wilcoxon is generally not applicable.
    #
    # We use the asymptotic approximation with continuity
    # correction.

    result = wilcoxon(
        default_values,
        custom_values,
        alternative="two-sided",
        zero_method="wilcox",
        method="approx",
        correction=True,
    )

    return {
        "n": len(non_zero),
        "W": result.statistic,
        "p": result.pvalue,
        "method": "approx_with_continuity_correction",
    }


def main():

    parser = argparse.ArgumentParser()

    parser.add_argument(
        "--default",
        required=True,
        type=Path,
        help="default/prometheus directory",
    )

    parser.add_argument(
        "--custom",
        required=True,
        type=Path,
        help="custom-jvm/prometheus directory",
    )

    parser.add_argument(
        "--output",
        default="current_run_wilcoxon.csv",
        type=Path,
    )

    args = parser.parse_args()

    results = []

    for metric, filename in PROM_FILES.items():

        default_file = (
            args.default
            / filename
        )

        custom_file = (
            args.custom
            / filename
        )

        if not (
            default_file.exists()
            and custom_file.exists()
        ):
            continue

        paired = pair_metric(
            default_file,
            custom_file,
        )

        if paired.empty:
            continue

        for service, group in paired.groupby(
            "service"
        ):

            test = run_wilcoxon(
                group["value_default"],
                group["value_custom"],
            )

            results.append({
                "metric": metric,
                "service": service,

                "n_paired_samples": test["n"],

                "default_median": group[
                    "value_default"
                ].median(),

                "custom_median": group[
                    "value_custom"
                ].median(),

                "delta_pct": (
                    (
                        group[
                            "value_custom"
                        ].median()
                        -
                        group[
                            "value_default"
                        ].median()
                    )
                    /
                    group[
                        "value_default"
                    ].median()
                    * 100
                )
                if group[
                    "value_default"
                ].median() != 0
                else np.nan,

                "wilcoxon_W": test["W"],
                "wilcoxon_p": test["p"],
                "method": test["method"],

                "significant_alpha_005": (
                    test["p"] < 0.05
                    if np.isfinite(test["p"])
                    else False
                ),
            })

    result = pd.DataFrame(results)

    result.to_csv(
        args.output,
        index=False,
    )

    print(
        f"Saved:\n{args.output}"
    )


if __name__ == "__main__":
    main()