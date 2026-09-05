# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-07-21T15:28:45Z

## How to read

- `delta %` = (custom-jvm − default) / default × 100. Negative = custom-jvm lower.
- For latency / CPU / memory / error metrics, **lower is better**; a positive delta is plugin overhead.
- `attestation_avg_ms` = mean of the **whole** workload attestation (k8s + unix + jvm) in ms,
  from raw `_sum`/`_count`. The k8s attestor (kubelet call) dominates, so small deltas here are noise.
- `jvm_*` rows = **plugin-only** cost in ms (from agent logs); shown on the custom-jvm side only.
  These are the cleanest measure of what the plugin itself costs.
- Prometheus rows are **means over the measurement window** (except `http_p99_ms_max`).
- default and custom-jvm are separate runs, so treat small (<~30%) deltas as run-to-run noise.

| scenario | metric | default | custom-jvm | delta % |
|----------|--------|---------|------------|---------|
| a | attestation_avg_ms | 6.391 | 6.454 | 1.0 |
| a | agent_cpu_cores_avg | 0.003422 | 0.004725 | 38.1 |
| a | agent_memory_mb_avg | 52.88 | 88.64 | 67.6 |
| a | server_cpu_cores_avg | 0.003601 | 0.004395 | 22.0 |
| a | server_memory_mb_avg | 50.33 | 44.61 | -11.4 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0 | 0 | — |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_ms_avg | — | 0 | — |
| a | jvm_jar_hash_ms_max | — | 0 | — |
| a | jvm_attest_total_ms_avg | — | 0.46 | — |
| a | jvm_attest_total_ms_max | — | 1 | — |
| b | attestation_avg_ms | 10.73 | 8.027 | -25.2 |
| b | agent_cpu_cores_avg | 0.003356 | 0.002663 | -20.6 |
| b | agent_memory_mb_avg | 30.15 | 31.64 | 4.9 |
| b | server_cpu_cores_avg | 0.004246 | 0.003512 | -17.3 |
| b | server_memory_mb_avg | 56.71 | 58.71 | 3.5 |
| b | http_p95_ms_avg | 27.64 | 25.28 | -8.6 |
| b | http_p99_ms_avg | 29.92 | 26.21 | -12.4 |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0 | 0 | — |
| b | http_p99_ms_max | 61.1 | 41.59 | -31.9 |
| b | k6_http_p95_ms | 26.38 | 24.25 | -8.1 |
| b | k6_error_rate | 0 | 0 | — |
| b | jvm_jar_hash_ms_avg | — | 0.73 | — |
| b | jvm_jar_hash_ms_max | — | 8 | — |
| b | jvm_attest_total_ms_avg | — | 1.55 | — |
| b | jvm_attest_total_ms_max | — | 9 | — |
| c | attestation_avg_ms | 2.725 | 2.238 | -17.9 |
| c | agent_cpu_cores_avg | 0.003047 | 0.002383 | -21.8 |
| c | agent_memory_mb_avg | 28.71 | 33.05 | 15.1 |
| c | server_cpu_cores_avg | 0.003934 | 0.003202 | -18.6 |
| c | server_memory_mb_avg | 57.29 | 58.47 | 2.1 |
| c | http_p95_ms_avg | 26.79 | 25.29 | -5.6 |
| c | http_p99_ms_avg | 135.2 | 95.92 | -29.1 |
| c | http_5xx_rate_avg | 4.727 | 4.962 | 5.0 |
| c | svid_issued_rate_avg | 0 | 0 | — |
| c | http_p99_ms_max | 1549 | 1190 | -23.2 |
| c | k6_http_p95_ms | 27.83 | 25.13 | -9.7 |
| c | k6_error_rate | 0.01773 | 0.01479 | — |
| c | jvm_jar_hash_ms_avg | — | 0 | — |
| c | jvm_jar_hash_ms_max | — | 0 | — |
| c | jvm_attest_total_ms_avg | — | 0 | — |
| c | jvm_attest_total_ms_max | — | 0 | — |
| d | attestation_avg_ms | 0.687 | 1.059 | 54.1 |
| d | agent_cpu_cores_avg | 0.00274 | 0.002417 | -11.8 |
| d | agent_memory_mb_avg | 27.02 | 31.68 | 17.3 |
| d | server_cpu_cores_avg | 0.003643 | 0.003263 | -10.4 |
| d | server_memory_mb_avg | 57.19 | 59.04 | 3.2 |
| d | http_p95_ms_avg | 25.54 | 24.89 | -2.5 |
| d | http_p99_ms_avg | 26.52 | 25.12 | -5.3 |
| d | http_5xx_rate_avg | — | — | — |
| d | svid_issued_rate_avg | 0.003306 | 0.003306 | 0.0 |
| d | http_p99_ms_max | 27.92 | 27.91 | -0.0 |
| d | k6_http_p95_ms | 25.48 | 23.77 | -6.7 |
| d | k6_error_rate | 0 | 0 | — |
| d | jvm_jar_hash_ms_avg | — | — | — |
| d | jvm_jar_hash_ms_max | — | — | — |
| d | jvm_attest_total_ms_avg | — | — | — |
| d | jvm_attest_total_ms_max | — | — | — |

Raw runs under: `results/run-20260721-132745`
