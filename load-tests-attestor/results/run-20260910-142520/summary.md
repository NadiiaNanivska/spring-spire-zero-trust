# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-09-10T15:10:01Z

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
| a | attestation_avg_ms | 4.123 | 17.64 | 327.7 |
| a | agent_cpu_cores_avg | 0.003415 | 0.007012 | 105.3 |
| a | agent_memory_mb_avg | 71.08 | 32.97 | -53.6 |
| a | server_cpu_cores_avg | 0.008819 | 0.01152 | 30.6 |
| a | server_memory_mb_avg | 162.4 | 169.5 | 4.4 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0.04502 | 0.05522 | 22.6 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 1.479e+04 | — |
| a | jvm_jar_hash_us_max | — | 6.945e+04 | — |
| a | jvm_attest_total_us_avg | — | 1.534e+04 | — |
| a | jvm_attest_total_us_max | — | 7.056e+04 | — |
| b | attestation_avg_ms | 4.639 | 13.38 | 188.4 |
| b | agent_cpu_cores_avg | 0.003626 | 0.008119 | 123.9 |
| b | agent_memory_mb_avg | 45.4 | 35.78 | -21.2 |
| b | server_cpu_cores_avg | 0.01374 | 0.02309 | 68.0 |
| b | server_memory_mb_avg | 167.1 | 172.6 | 3.3 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.0593 | 0.07013 | 18.3 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 2.126e+04 | — |
| b | jvm_jar_hash_us_max | — | 7.989e+04 | — |
| b | jvm_attest_total_us_avg | — | 2.21e+04 | — |
| b | jvm_attest_total_us_max | — | 8.086e+04 | — |
| c | attestation_avg_ms | 1.955 | 16.91 | 764.8 |
| c | agent_cpu_cores_avg | 0.003069 | 0.004733 | 54.2 |
| c | agent_memory_mb_avg | 30.81 | 37.05 | 20.3 |
| c | server_cpu_cores_avg | 0.009222 | 0.01208 | 31.0 |
| c | server_memory_mb_avg | 169.2 | 170.6 | 0.8 |
| c | http_p95_ms_avg | 30.57 | 49.14 | 60.7 |
| c | http_p99_ms_avg | 132.9 | 220 | 65.5 |
| c | http_5xx_rate_avg | 0.9326 | 1.281 | 37.4 |
| c | svid_issued_rate_avg | 0.06867 | 0.05897 | -14.1 |
| c | http_p99_ms_max | 1534 | 2431 | 58.5 |
| c | k6_http_p95_ms | 26.04 | 29.15 | 11.9 |
| c | k6_error_rate | 0.01378 | 0.02897 | — |
| c | jvm_jar_hash_us_avg | — | 4.336e+04 | — |
| c | jvm_jar_hash_us_max | — | 8.612e+04 | — |
| c | jvm_attest_total_us_avg | — | 4.452e+04 | — |
| c | jvm_attest_total_us_max | — | 8.913e+04 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260910-142520`
