# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-09-05T20:09:13Z

## How to read

- `delta %` = (custom-jvm − default) / default × 100. Negative = custom-jvm lower.
- For latency / CPU / memory / error metrics, **lower is better**; a positive delta is plugin overhead.
- `attestation_avg_ms` = mean of the **whole** workload attestation (k8s + unix + jvm) in ms,
  from raw `_sum`/`_count`. The k8s attestor (kubelet call) dominates, so small deltas here are noise.
- `jvm_*_us` rows = **plugin-only** cost in **microseconds** (from agent logs); custom-jvm side only.
  These are the cleanest measure of what the plugin itself costs. Compare S-A (warm cache)
  vs S-B (cold compute) `jvm_jar_hash_us_*` to read the hash-cache benefit.
- Prometheus rows are **means over the measurement window** (except `http_p99_ms_max`).
- default and custom-jvm are separate runs, so treat small (<~30%) deltas as run-to-run noise.

| scenario | metric | default | custom-jvm | delta % |
|----------|--------|---------|------------|---------|
| a | attestation_avg_ms | 4.596 | 34.71 | 655.3 |
| a | agent_cpu_cores_avg | 0.003577 | 0.007847 | 119.4 |
| a | agent_memory_mb_avg | 31.02 | 20.58 | -33.7 |
| a | server_cpu_cores_avg | 0.006971 | 0.01328 | 90.6 |
| a | server_memory_mb_avg | 142.5 | 163.3 | 14.6 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0.07475 | 0.06087 | -18.6 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 4.154e+04 | — |
| a | jvm_jar_hash_us_max | — | 6.241e+04 | — |
| a | jvm_attest_total_us_avg | — | 4.198e+04 | — |
| a | jvm_attest_total_us_max | — | 6.29e+04 | — |
| b | attestation_avg_ms | 5.145 | 11.15 | 116.7 |
| b | agent_cpu_cores_avg | 0.003399 | 0.007705 | 126.7 |
| b | agent_memory_mb_avg | 26.26 | 35.18 | 33.9 |
| b | server_cpu_cores_avg | 0.01734 | 0.01982 | 14.3 |
| b | server_memory_mb_avg | 143.5 | 168.4 | 17.4 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.06364 | 0.05097 | -19.9 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 2.087e+04 | — |
| b | jvm_jar_hash_us_max | — | 8.719e+04 | — |
| b | jvm_attest_total_us_avg | — | 2.133e+04 | — |
| b | jvm_attest_total_us_max | — | 8.802e+04 | — |
| c | attestation_avg_ms | 1.765 | 16.84 | 854.2 |
| c | agent_cpu_cores_avg | 0.003029 | 0.00512 | 69.1 |
| c | agent_memory_mb_avg | 29.98 | 38.42 | 28.1 |
| c | server_cpu_cores_avg | 0.009278 | 0.01436 | 54.8 |
| c | server_memory_mb_avg | 142.8 | 171 | 19.8 |
| c | http_p95_ms_avg | 30.68 | 39.8 | 29.7 |
| c | http_p99_ms_avg | 132.1 | 226.7 | 71.5 |
| c | http_5xx_rate_avg | 0.6205 | 1.472 | 137.2 |
| c | svid_issued_rate_avg | 0.06506 | 0.07032 | 8.1 |
| c | http_p99_ms_max | 1402 | 2366 | 68.7 |
| c | k6_http_p95_ms | 25.98 | 30.32 | 16.7 |
| c | k6_error_rate | 0.01323 | 0.03071 | — |
| c | jvm_jar_hash_us_avg | — | 4.436e+04 | — |
| c | jvm_jar_hash_us_max | — | 9.194e+04 | — |
| c | jvm_attest_total_us_avg | — | 4.516e+04 | — |
| c | jvm_attest_total_us_max | — | 9.278e+04 | — |

Raw runs under: `results/run-20260905-191950`
