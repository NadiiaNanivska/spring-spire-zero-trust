# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-07-30T14:35:00Z

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
| a | attestation_avg_ms | 3.571 | 3.538 | -0.9 |
| a | agent_cpu_cores_avg | 0.003652 | 0.003702 | 1.4 |
| a | agent_memory_mb_avg | 15.16 | 32.9 | 117.1 |
| a | server_cpu_cores_avg | 0.008815 | 0.007931 | -10.0 |
| a | server_memory_mb_avg | 162.9 | 170 | 4.3 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0.03545 | 0.05758 | 62.4 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 362 | — |
| a | jvm_jar_hash_us_max | — | 529 | — |
| a | jvm_attest_total_us_avg | — | 687.7 | — |
| a | jvm_attest_total_us_max | — | 1098 | — |
| b | attestation_avg_ms | 4.672 | 14.07 | 201.1 |
| b | agent_cpu_cores_avg | 0.003626 | 0.005217 | 43.9 |
| b | agent_memory_mb_avg | 24.28 | 33.03 | 36.0 |
| b | server_cpu_cores_avg | 0.01504 | 0.01759 | 17.0 |
| b | server_memory_mb_avg | 168 | 171.4 | 2.1 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.05705 | 0.07081 | 24.1 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 1.428e+04 | — |
| b | jvm_jar_hash_us_max | — | 5.199e+04 | — |
| b | jvm_attest_total_us_avg | — | 1.461e+04 | — |
| b | jvm_attest_total_us_max | — | 5.237e+04 | — |
| c | attestation_avg_ms | 1.908 | 2.453 | 28.6 |
| c | agent_cpu_cores_avg | 0.002971 | 0.003316 | 11.6 |
| c | agent_memory_mb_avg | 29.1 | 36.39 | 25.1 |
| c | server_cpu_cores_avg | 0.008431 | 0.008178 | -3.0 |
| c | server_memory_mb_avg | 168.9 | 169.7 | 0.5 |
| c | http_p95_ms_avg | 34.25 | 30.78 | -10.1 |
| c | http_p99_ms_avg | 135.7 | 135.8 | 0.1 |
| c | http_5xx_rate_avg | 3.118 | 0 | -100.0 |
| c | svid_issued_rate_avg | 0.06146 | 0.07122 | 15.9 |
| c | http_p99_ms_max | 1400 | 1505 | 7.5 |
| c | k6_http_p95_ms | 25.64 | 25.63 | -0.1 |
| c | k6_error_rate | 0.01251 | 0.01253 | — |
| c | jvm_jar_hash_us_avg | — | 483.5 | — |
| c | jvm_jar_hash_us_max | — | 675 | — |
| c | jvm_attest_total_us_avg | — | 1075 | — |
| c | jvm_attest_total_us_max | — | 1831 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260730-135344`
