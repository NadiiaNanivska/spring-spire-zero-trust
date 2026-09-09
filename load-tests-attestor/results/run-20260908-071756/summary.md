# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-09-08T07:58:47Z

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
| a | attestation_avg_ms | 3.967 | 13.67 | 244.6 |
| a | agent_cpu_cores_avg | 0.00344 | 0.003542 | 3.0 |
| a | agent_memory_mb_avg | 69.34 | 34.57 | -50.1 |
| a | server_cpu_cores_avg | 0.008148 | 0.01119 | 37.4 |
| a | server_memory_mb_avg | 162 | 170.9 | 5.5 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0.06411 | 0.04545 | -29.1 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 3.758e+04 | — |
| a | jvm_jar_hash_us_max | — | 3.758e+04 | — |
| a | jvm_attest_total_us_avg | — | 3.785e+04 | — |
| a | jvm_attest_total_us_max | — | 3.785e+04 | — |
| b | attestation_avg_ms | 5.286 | 14.34 | 171.2 |
| b | agent_cpu_cores_avg | 0.003595 | 0.00546 | 51.9 |
| b | agent_memory_mb_avg | 44.29 | 33.82 | -23.6 |
| b | server_cpu_cores_avg | 0.01632 | 0.01733 | 6.2 |
| b | server_memory_mb_avg | 167.4 | 171.6 | 2.5 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.07398 | 0.04912 | -33.6 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 1.624e+04 | — |
| b | jvm_jar_hash_us_max | — | 5.378e+04 | — |
| b | jvm_attest_total_us_avg | — | 1.68e+04 | — |
| b | jvm_attest_total_us_max | — | 5.41e+04 | — |
| c | attestation_avg_ms | 1.791 | 8.574 | 378.7 |
| c | agent_cpu_cores_avg | 0.003038 | 0.003305 | 8.8 |
| c | agent_memory_mb_avg | 30.55 | 37.01 | 21.1 |
| c | server_cpu_cores_avg | 0.008993 | 0.009816 | 9.1 |
| c | server_memory_mb_avg | 170.3 | 172.1 | 1.1 |
| c | http_p95_ms_avg | 26.78 | 35.62 | 33.0 |
| c | http_p99_ms_avg | 108.1 | 141.9 | 31.3 |
| c | http_5xx_rate_avg | 4.105 | 1.754 | -57.3 |
| c | svid_issued_rate_avg | 0.06687 | 0.06807 | 1.8 |
| c | http_p99_ms_max | 1083 | 1551 | 43.2 |
| c | k6_http_p95_ms | 25.93 | 25.76 | -0.7 |
| c | k6_error_rate | 0.01389 | 0.01277 | — |
| c | jvm_jar_hash_us_avg | — | 2.17e+04 | — |
| c | jvm_jar_hash_us_max | — | 4.336e+04 | — |
| c | jvm_attest_total_us_avg | — | 2.201e+04 | — |
| c | jvm_attest_total_us_max | — | 4.364e+04 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260908-071756`
