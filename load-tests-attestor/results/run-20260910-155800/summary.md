# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-09-10T17:12:13Z

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
| a | attestation_avg_ms | 7.153 | 51.58 | 621.1 |
| a | agent_cpu_cores_avg | 0.005928 | 0.008267 | 39.5 |
| a | agent_memory_mb_avg | 35.14 | 34.82 | -0.9 |
| a | server_cpu_cores_avg | 0.01527 | 0.01485 | -2.7 |
| a | server_memory_mb_avg | 171.5 | 171.6 | 0.1 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | 0 | — | — |
| a | svid_issued_rate_avg | 0.06458 | 0.04848 | -24.9 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 6.784e+04 | — |
| a | jvm_jar_hash_us_max | — | 7.587e+04 | — |
| a | jvm_attest_total_us_avg | — | 6.947e+04 | — |
| a | jvm_attest_total_us_max | — | 7.625e+04 | — |
| b | attestation_avg_ms | 3.978 | — | — |
| b | agent_cpu_cores_avg | 0.004264 | 0.008405 | 97.1 |
| b | agent_memory_mb_avg | 30.02 | 35.81 | 19.3 |
| b | server_cpu_cores_avg | 0.01375 | 0.02356 | 71.4 |
| b | server_memory_mb_avg | 171.6 | 172.1 | 0.3 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.06527 | 0.06667 | 2.1 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 2.417e+04 | — |
| b | jvm_jar_hash_us_max | — | 1.027e+05 | — |
| b | jvm_attest_total_us_avg | — | 2.491e+04 | — |
| b | jvm_attest_total_us_max | — | 1.031e+05 | — |
| c | attestation_avg_ms | 1.822 | 17.34 | 851.5 |
| c | agent_cpu_cores_avg | 0.003153 | 0.005098 | 61.7 |
| c | agent_memory_mb_avg | 31.22 | 38.16 | 22.2 |
| c | server_cpu_cores_avg | 0.009664 | 0.01342 | 38.9 |
| c | server_memory_mb_avg | 172 | 172.4 | 0.2 |
| c | http_p95_ms_avg | 34.75 | 54.73 | 57.5 |
| c | http_p99_ms_avg | 136.1 | 265.4 | 95.0 |
| c | http_5xx_rate_avg | 3.69 | 1.398 | -62.1 |
| c | svid_issued_rate_avg | 0.06635 | 0.06431 | -3.1 |
| c | http_p99_ms_max | 1562 | 2684 | 71.8 |
| c | k6_http_p95_ms | 25.89 | 29.48 | 13.9 |
| c | k6_error_rate | 0.0136 | 0.02851 | — |
| c | jvm_jar_hash_us_avg | — | 3.157e+04 | — |
| c | jvm_jar_hash_us_max | — | 9.207e+04 | — |
| c | jvm_attest_total_us_avg | — | 3.219e+04 | — |
| c | jvm_attest_total_us_max | — | 9.261e+04 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260910-155800`
