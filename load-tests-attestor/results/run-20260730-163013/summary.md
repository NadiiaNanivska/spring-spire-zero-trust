# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-07-30T17:11:16Z

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
| a | attestation_avg_ms | 3.962 | 4.591 | 15.9 |
| a | agent_cpu_cores_avg | 0.003378 | 0.003436 | 1.7 |
| a | agent_memory_mb_avg | 32.93 | 33.64 | 2.1 |
| a | server_cpu_cores_avg | 0.00829 | 0.009036 | 9.0 |
| a | server_memory_mb_avg | 115.6 | 139.4 | 20.6 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0.07455 | 0.05868 | -21.3 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 364 | — |
| a | jvm_jar_hash_us_max | — | 761 | — |
| a | jvm_attest_total_us_avg | — | 715.5 | — |
| a | jvm_attest_total_us_max | — | 1192 | — |
| b | attestation_avg_ms | 4.677 | 22.07 | 371.9 |
| b | agent_cpu_cores_avg | 0.003169 | 0.00532 | 67.9 |
| b | agent_memory_mb_avg | 29.23 | 35.17 | 20.3 |
| b | server_cpu_cores_avg | 0.01573 | 0.01666 | 5.9 |
| b | server_memory_mb_avg | 128 | 142.7 | 11.5 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.06006 | 0.06301 | 4.9 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 1.327e+04 | — |
| b | jvm_jar_hash_us_max | — | 5.736e+04 | — |
| b | jvm_attest_total_us_avg | — | 1.364e+04 | — |
| b | jvm_attest_total_us_max | — | 5.864e+04 | — |
| c | attestation_avg_ms | 1.532 | 2.251 | 46.9 |
| c | agent_cpu_cores_avg | 0.002913 | 0.003087 | 6.0 |
| c | agent_memory_mb_avg | 28.97 | 36.66 | 26.5 |
| c | server_cpu_cores_avg | 0.008469 | 0.009886 | 16.7 |
| c | server_memory_mb_avg | 140.7 | 143 | 1.6 |
| c | http_p95_ms_avg | 30.11 | 30.79 | 2.3 |
| c | http_p99_ms_avg | 132.5 | 144 | 8.7 |
| c | http_5xx_rate_avg | 5.257 | 4.453 | -15.3 |
| c | svid_issued_rate_avg | 0.06627 | 0.06356 | -4.1 |
| c | http_p99_ms_max | 1360 | 1472 | 8.3 |
| c | k6_http_p95_ms | 26 | 25.9 | -0.4 |
| c | k6_error_rate | 0.0135 | 0.01297 | — |
| c | jvm_jar_hash_us_avg | — | 925.2 | — |
| c | jvm_jar_hash_us_max | — | 2034 | — |
| c | jvm_attest_total_us_avg | — | 1376 | — |
| c | jvm_attest_total_us_max | — | 3221 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260730-163013`
