#!/usr/bin/env python3

from __future__ import annotations

import argparse
from pathlib import Path

import matplotlib.pyplot as plt
import pandas as pd


def boxplots(df, output_dir):
    output_dir.mkdir(parents=True, exist_ok=True)

    metrics = [
        "attestation_avg_ms",
        "agent_cpu",
        "agent_memory_mb",
        "server_cpu",
        "server_memory_mb",
        "http_p95_ms",
        "http_p99_ms",
        "http_5xx_rate",
        "k6_http_p95_ms",
        "k6_error_rate",
    ]

    for scenario in sorted(df["scenario"].unique()):

        scenario_df = df[
            (df["scenario"] == scenario)
            & df["overlay"].isin(["default", "custom-jvm"])
        ]

        for metric in metrics:

            metric_df = scenario_df[
                scenario_df["metric"] == metric
            ]

            if metric_df.empty:
                continue

            values = []
            labels = []

            for overlay in ["default", "custom-jvm"]:

                x = metric_df[
                    metric_df["overlay"] == overlay
                ]["value"].dropna()

                if len(x) == 0:
                    continue

                values.append(x.to_numpy())
                labels.append(overlay)

            if not values:
                continue

            fig = plt.figure(figsize=(8, 5))

            plt.boxplot(
                values,
                labels=labels,
                showmeans=True,
                whis=[0, 100],
            )

            plt.title(
                f"Scenario {scenario.upper()} — {metric}"
            )

            plt.ylabel(metric)
            plt.grid(axis="y", alpha=0.25)

            fig.tight_layout()

            filename = (
                output_dir
                / f"scenario_{scenario}_{metric}.png"
            )

            fig.savefig(
                filename,
                dpi=200,
            )

            plt.close(fig)


def paired_plots(df, output_dir):
    output_dir.mkdir(parents=True, exist_ok=True)

    metrics = [
        "attestation_avg_ms",
        "agent_cpu",
        "agent_memory_mb",
        "server_cpu",
        "server_memory_mb",
        "http_p95_ms",
        "http_p99_ms",
        "http_5xx_rate",
    ]

    for scenario in sorted(df["scenario"].unique()):

        for metric in metrics:

            x = df[
                (df["scenario"] == scenario)
                & (df["metric"] == metric)
                & df["overlay"].isin(
                    ["default", "custom-jvm"]
                )
            ]

            pivot = x.pivot_table(
                index="run_id",
                columns="overlay",
                values="value",
                aggfunc="first",
            )

            if not {
                "default",
                "custom-jvm",
            }.issubset(pivot.columns):
                continue

            pivot = pivot.dropna()

            if len(pivot) < 2:
                continue

            fig = plt.figure(figsize=(8, 5))

            for _, row in pivot.iterrows():

                plt.plot(
                    ["default", "custom-jvm"],
                    [
                        row["default"],
                        row["custom-jvm"],
                    ],
                    marker="o",
                    alpha=0.6,
                )

            plt.title(
                f"Paired comparison — "
                f"scenario {scenario.upper()} — {metric}"
            )

            plt.ylabel(metric)
            plt.grid(axis="y", alpha=0.25)

            fig.tight_layout()

            filename = (
                output_dir
                / f"paired_{scenario}_{metric}.png"
            )

            fig.savefig(
                filename,
                dpi=200,
            )

            plt.close(fig)


def main():

    parser = argparse.ArgumentParser()

    parser.add_argument(
        "input",
        type=Path,
    )

    parser.add_argument(
        "--output-dir",
        type=Path,
        default=Path("analysis/plots"),
    )

    args = parser.parse_args()

    df = pd.read_csv(args.input)

    # One representative value per run.
    df = df[
        (df["statistic"].isin(["median", "run_summary"]))
        | (df["metric"] == "attestation_avg_ms")
    ].copy()

    boxplots(
        df,
        args.output_dir / "boxplots",
    )

    paired_plots(
        df,
        args.output_dir / "paired",
    )


if __name__ == "__main__":
    main()