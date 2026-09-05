# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-07-29T18:47:47Z

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
| a | attestation_avg_ms | 4.546 | 4.464 | -1.8 |
| a | agent_cpu_cores_avg | 0.003486 | 0.00303 | -13.1 |
| a | agent_memory_mb_avg | 14.99 | 30.86 | 105.8 |
| a | server_cpu_cores_avg | 0.007317 | 0.008238 | 12.6 |
| a | server_memory_mb_avg | 160.8 | 170.8 | 6.2 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0.07751 | 0.07273 | -6.2 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 358.7 | — |
| a | jvm_jar_hash_us_max | — | 515 | — |
| a | jvm_attest_total_us_avg | — | 908.3 | — |
| a | jvm_attest_total_us_max | — | 1607 | — |
| b | attestation_avg_ms | — | 15.37 | — |
| b | agent_cpu_cores_avg | 0.003276 | 0.004811 | 46.9 |
| b | agent_memory_mb_avg | 25.09 | 32.65 | 30.1 |
| b | server_cpu_cores_avg | 0.01544 | 0.01696 | 9.8 |
| b | server_memory_mb_avg | 166.8 | 170.7 | 2.4 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.06426 | 0.05742 | -10.7 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 1.643e+04 | — |
| b | jvm_jar_hash_us_max | — | 6.039e+04 | — |
| b | jvm_attest_total_us_avg | — | 1.703e+04 | — |
| b | jvm_attest_total_us_max | — | 6.078e+04 | — |
| c | attestation_avg_ms | 1.563 | 2.644 | 69.2 |
| c | agent_cpu_cores_avg | 0.002893 | 0.003019 | 4.4 |
| c | agent_memory_mb_avg | 29.9 | 35.16 | 17.6 |
| c | server_cpu_cores_avg | 0.009007 | 0.009406 | 4.4 |
| c | server_memory_mb_avg | 169.4 | 170.2 | 0.5 |
| c | http_p95_ms_avg | 29.85 | 33.04 | 10.7 |
| c | http_p99_ms_avg | 133.3 | 144.6 | 8.4 |
| c | http_5xx_rate_avg | 4.47 | 4.398 | -1.6 |
| c | svid_issued_rate_avg | 0.06957 | 0.05394 | -22.5 |
| c | http_p99_ms_max | 1393 | 1355 | -2.7 |
| c | k6_http_p95_ms | 25.72 | 25.79 | 0.3 |
| c | k6_error_rate | 0.01155 | 0.0135 | — |
| c | jvm_jar_hash_us_avg | — | 1193 | — |
| c | jvm_jar_hash_us_max | — | 3710 | — |
| c | jvm_attest_total_us_avg | — | 1465 | — |
| c | jvm_attest_total_us_max | — | 4077 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260729-180700`
