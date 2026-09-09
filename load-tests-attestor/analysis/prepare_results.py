#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path

import numpy as np
import pandas as pd


PROM_FILES = {
    "agent_cpu": "agent_cpu.json",
    "agent_memory_mb": "agent_memory_mb.json",
    "server_cpu": "server_cpu.json",
    "server_memory_mb": "server_memory_mb.json",
    "http_p95_ms": "http_req_p95_ms.json",
    "http_p99_ms": "http_req_p99_ms.json",
    "http_5xx_rate": "http_5xx_rate.json",
    "svid_issued_rate": "svid_issued_rate.json",
}

ATT_KEY = "attestation_avg_ms"


def parse_run_dir(path: Path):
    """
    Expected:
      custom-jvm-scenario-a-20260905-191950
      default-scenario-c-20260730-171948
    """

    m = re.match(
        r"(?P<overlay>default|custom-jvm)-scenario-"
        r"(?P<scenario>[a-z])-"
        r"(?P<runid>\d{8}-\d{6})$",
        path.name,
    )

    if not m:
        return None

    return m.groupdict()


def read_json(path: Path):
    if not path.exists():
        return None

    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def prometheus_values(path: Path):
    """
    Returns:
        list[(service, timestamp, value)]
    """

    data = read_json(path)

    if not data:
        return []

    result = []

    for series in data.get("data", {}).get("result", []):
        labels = series.get("metric", {})
        service = (
            labels.get("service")
            or labels.get("pod")
            or labels.get("instance")
            or "aggregate"
        )

        for timestamp, value in series.get("values", []):
            try:
                value = float(value)
            except (TypeError, ValueError):
                continue

            if np.isfinite(value):
                result.append((service, timestamp, value))

    return result


def summarize(values):
    if not values:
        return {
            "median": np.nan,
            "q1": np.nan,
            "q3": np.nan,
            "iqr": np.nan,
            "min": np.nan,
            "max": np.nan,
            "n_samples": 0,
        }

    x = np.asarray(values, dtype=float)

    q1, median, q3 = np.percentile(
        x,
        [25, 50, 75],
        method="linear",
    )

    return {
        "median": median,
        "q1": q1,
        "q3": q3,
        "iqr": q3 - q1,
        "min": np.min(x),
        "max": np.max(x),
        "n_samples": len(x),
    }


def attestation_average(prom_dir: Path):
    sum_file = prom_dir / "attestation_elapsed_sum.json"
    count_file = prom_dir / "attestation_elapsed_count.json"

    sums = prometheus_values(sum_file)
    counts = prometheus_values(count_file)

    if not sums or not counts:
        return np.nan

    # Usually one aggregate series.
    # Use the change over the measurement window.
    sum_values = [x[2] for x in sums]
    count_values = [x[2] for x in counts]

    delta_sum = sum_values[-1] - sum_values[0]
    delta_count = count_values[-1] - count_values[0]

    if delta_count <= 0:
        return np.nan

    return delta_sum / delta_count


def read_k6(run_dir: Path):
    path = run_dir / "k6-summary.json"

    data = read_json(path)

    if not data:
        return {}

    metrics = data.get("metrics", {})

    duration = metrics.get("http_req_duration", {}).get("values", {})
    failed = metrics.get("http_req_failed", {}).get("values", {})
    requests = metrics.get("http_reqs", {}).get("values", {})

    return {
        "k6_http_p95_ms": duration.get("p(95)", np.nan),
        "k6_http_median_ms": duration.get("med", np.nan),
        "k6_http_min_ms": duration.get("min", np.nan),
        "k6_http_max_ms": duration.get("max", np.nan),
        "k6_error_rate": failed.get("rate", np.nan),
        "k6_throughput_req_s": requests.get("rate", np.nan),
    }


def read_plugin_timing(run_dir: Path):
    path = run_dir / "attestor-timing.csv"

    if not path.exists():
        return {}

    df = pd.read_csv(path)

    result = {}

    for column in [
        "total_us",
        "anti_debug_us",
        "anti_tamper_us",
        "jar_hash_us",
    ]:
        if column not in df:
            continue

        stats = summarize(df[column].dropna().to_numpy())

        for key, value in stats.items():
            result[f"plugin_{column}_{key}"] = value

    result["plugin_attestation_events"] = len(df)

    return result


def process_run(run_dir: Path):
    meta = parse_run_dir(run_dir)

    if meta is None:
        return []

    overlay = meta["overlay"]
    scenario = meta["scenario"]
    run_id = meta["runid"]

    prom_dir = run_dir / "prometheus"

    rows = []

    # Whole workload attestation.
    attestation = attestation_average(prom_dir)

    rows.append({
        "run_id": run_id,
        "overlay": overlay,
        "scenario": scenario,
        "metric": ATT_KEY,
        "service": "aggregate",
        "value": attestation,
        "statistic": "mean_over_attestation_events",
    })

    # Prometheus metrics.
    for metric, filename in PROM_FILES.items():
        values = prometheus_values(prom_dir / filename)

        by_service = {}

        for service, timestamp, value in values:
            by_service.setdefault(service, []).append(value)

        for service, service_values in by_service.items():
            stats = summarize(service_values)

            for statistic, value in stats.items():
                if statistic == "n_samples":
                    continue

                rows.append({
                    "run_id": run_id,
                    "overlay": overlay,
                    "scenario": scenario,
                    "metric": metric,
                    "service": service,
                    "value": value,
                    "statistic": statistic,
                })

    # k6 metrics are already summarized by k6.
    for metric, value in read_k6(run_dir).items():
        rows.append({
            "run_id": run_id,
            "overlay": overlay,
            "scenario": scenario,
            "metric": metric,
            "service": "k6",
            "value": value,
            "statistic": "run_summary",
        })

    # Plugin-only timing.
    plugin = read_plugin_timing(run_dir)

    for key, value in plugin.items():
        if key == "plugin_attestation_events":
            continue

        metric, statistic = key.rsplit("_", 1)

        rows.append({
            "run_id": run_id,
            "overlay": overlay,
            "scenario": scenario,
            "metric": metric,
            "service": "spire-agent",
            "value": value,
            "statistic": statistic,
        })

    return rows


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "results_dir",
        type=Path,
        help="Path to load-tests-attestor/results",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=Path("prepared_results.csv"),
    )

    args = parser.parse_args()

    all_rows = []

    for run_dir in sorted(args.results_dir.glob("run-*")):
        if not run_dir.is_dir():
            continue

        for child in sorted(run_dir.iterdir()):
            if not child.is_dir():
                continue

            rows = process_run(child)
            all_rows.extend(rows)

    if not all_rows:
        raise SystemExit("No experimental run directories found.")

    df = pd.DataFrame(all_rows)

    df = df.replace([np.inf, -np.inf], np.nan)
    df = df.dropna(subset=["value"])

    df.to_csv(args.output, index=False)

    print(f"Wrote {len(df)} rows to {args.output}")


if __name__ == "__main__":
    main()