# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-09-08T14:15:15Z

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
| a | attestation_avg_ms | 3.489 | 14.61 | 318.7 |
| a | agent_cpu_cores_avg | 0.003453 | 0.005192 | 50.4 |
| a | agent_memory_mb_avg | 31.52 | 33.83 | 7.3 |
| a | server_cpu_cores_avg | 0.007268 | 0.006928 | -4.7 |
| a | server_memory_mb_avg | 170.2 | 171.5 | 0.8 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0.0488 | 0.06109 | 25.2 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 1.412e+04 | — |
| a | jvm_jar_hash_us_max | — | 4.393e+04 | — |
| a | jvm_attest_total_us_avg | — | 1.44e+04 | — |
| a | jvm_attest_total_us_max | — | 4.412e+04 | — |
| b | attestation_avg_ms | — | 14.5 | — |
| b | agent_cpu_cores_avg | 0.003611 | 0.005582 | 54.6 |
| b | agent_memory_mb_avg | 27.62 | 34.7 | 25.6 |
| b | server_cpu_cores_avg | 0.01604 | 0.01624 | 1.2 |
| b | server_memory_mb_avg | 171.7 | 172.5 | 0.5 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.07868 | 0.05997 | -23.8 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 1.555e+04 | — |
| b | jvm_jar_hash_us_max | — | 5.23e+04 | — |
| b | jvm_attest_total_us_avg | — | 1.594e+04 | — |
| b | jvm_attest_total_us_max | — | 5.249e+04 | — |
| c | attestation_avg_ms | 2.385 | 9.648 | 304.5 |
| c | agent_cpu_cores_avg | 0.00303 | 0.003414 | 12.7 |
| c | agent_memory_mb_avg | 29.97 | 37.37 | 24.7 |
| c | server_cpu_cores_avg | 0.009013 | 0.008651 | -4.0 |
| c | server_memory_mb_avg | 172.1 | 171.7 | -0.2 |
| c | http_p95_ms_avg | 30.75 | 30.16 | -1.9 |
| c | http_p99_ms_avg | 137.6 | 130.4 | -5.3 |
| c | http_5xx_rate_avg | 0 | 0 | — |
| c | svid_issued_rate_avg | 0.06867 | 0.05995 | -12.7 |
| c | http_p99_ms_max | 1497 | 1309 | -12.5 |
| c | k6_http_p95_ms | 25.9 | 25.74 | -0.6 |
| c | k6_error_rate | 0.0122 | 0.01272 | — |
| c | jvm_jar_hash_us_avg | — | 2.361e+04 | — |
| c | jvm_jar_hash_us_max | — | 4.853e+04 | — |
| c | jvm_attest_total_us_avg | — | 2.386e+04 | — |
| c | jvm_attest_total_us_max | — | 4.875e+04 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260908-133356`
