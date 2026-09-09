# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-09-08T13:31:21Z

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
| a | attestation_avg_ms | 3.572 | 25.4 | 611.0 |
| a | agent_cpu_cores_avg | 0.003411 | 0.006061 | 77.7 |
| a | agent_memory_mb_avg | 70.2 | 33.62 | -52.1 |
| a | server_cpu_cores_avg | 0.007525 | 0.008813 | 17.1 |
| a | server_memory_mb_avg | 163.6 | 170.6 | 4.2 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0.07589 | 0.05758 | -24.1 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 2.718e+04 | — |
| a | jvm_jar_hash_us_max | — | 4.34e+04 | — |
| a | jvm_attest_total_us_avg | — | 2.76e+04 | — |
| a | jvm_attest_total_us_max | — | 4.358e+04 | — |
| b | attestation_avg_ms | — | 16.11 | — |
| b | agent_cpu_cores_avg | 0.00341 | 0.005449 | 59.8 |
| b | agent_memory_mb_avg | 43 | 34.09 | -20.7 |
| b | server_cpu_cores_avg | 0.01439 | 0.01856 | 29.0 |
| b | server_memory_mb_avg | 168.5 | 172.3 | 2.3 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.06348 | 0.06081 | -4.2 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 1.463e+04 | — |
| b | jvm_jar_hash_us_max | — | 5.306e+04 | — |
| b | jvm_attest_total_us_avg | — | 1.55e+04 | — |
| b | jvm_attest_total_us_max | — | 5.343e+04 | — |
| c | attestation_avg_ms | 1.965 | 10.65 | 442.2 |
| c | agent_cpu_cores_avg | 0.002996 | 0.003645 | 21.7 |
| c | agent_memory_mb_avg | 30.02 | 38.15 | 27.1 |
| c | server_cpu_cores_avg | 0.007689 | 0.00939 | 22.1 |
| c | server_memory_mb_avg | 169.4 | 170.4 | 0.6 |
| c | http_p95_ms_avg | 29.69 | 34.75 | 17.0 |
| c | http_p99_ms_avg | 138.4 | 126.9 | -8.2 |
| c | http_5xx_rate_avg | 1.157 | 1.327 | 14.7 |
| c | svid_issued_rate_avg | 0.07047 | 0.06972 | -1.1 |
| c | http_p99_ms_max | 1357 | 1421 | 4.7 |
| c | k6_http_p95_ms | 25.71 | 25.74 | 0.1 |
| c | k6_error_rate | 0.01245 | 0.01449 | — |
| c | jvm_jar_hash_us_avg | — | 2.656e+04 | — |
| c | jvm_jar_hash_us_max | — | 5.634e+04 | — |
| c | jvm_attest_total_us_avg | — | 2.686e+04 | — |
| c | jvm_attest_total_us_max | — | 5.669e+04 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260908-124923`
