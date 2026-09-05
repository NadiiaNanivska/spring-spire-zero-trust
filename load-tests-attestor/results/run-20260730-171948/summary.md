# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-07-30T18:00:40Z

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
| a | attestation_avg_ms | 4.596 | 2.728 | -40.6 |
| a | agent_cpu_cores_avg | 0.003577 | 0.003264 | -8.7 |
| a | agent_memory_mb_avg | 31.02 | 33.03 | 6.5 |
| a | server_cpu_cores_avg | 0.006971 | 0.008826 | 26.6 |
| a | server_memory_mb_avg | 142.5 | 142.4 | -0.1 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0.07475 | 0.06465 | -13.5 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 271 | — |
| a | jvm_jar_hash_us_max | — | 271 | — |
| a | jvm_attest_total_us_avg | — | 453 | — |
| a | jvm_attest_total_us_max | — | 453 | — |
| b | attestation_avg_ms | 5.145 | 13.71 | 166.4 |
| b | agent_cpu_cores_avg | 0.003399 | 0.005427 | 59.6 |
| b | agent_memory_mb_avg | 26.26 | 32.67 | 24.4 |
| b | server_cpu_cores_avg | 0.01734 | 0.01777 | 2.5 |
| b | server_memory_mb_avg | 143.5 | 144.1 | 0.4 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.06364 | 0.05639 | -11.4 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 1.592e+04 | — |
| b | jvm_jar_hash_us_max | — | 6.921e+04 | — |
| b | jvm_attest_total_us_avg | — | 1.627e+04 | — |
| b | jvm_attest_total_us_max | — | 6.95e+04 | — |
| c | attestation_avg_ms | 1.765 | 3.111 | 76.3 |
| c | agent_cpu_cores_avg | 0.003029 | 0.003117 | 2.9 |
| c | agent_memory_mb_avg | 29.98 | 35.78 | 19.3 |
| c | server_cpu_cores_avg | 0.009278 | 0.008573 | -7.6 |
| c | server_memory_mb_avg | 142.8 | 143.6 | 0.5 |
| c | http_p95_ms_avg | 30.68 | 34.36 | 12.0 |
| c | http_p99_ms_avg | 132.1 | 140.3 | 6.2 |
| c | http_5xx_rate_avg | 0.6205 | 1.791 | 188.6 |
| c | svid_issued_rate_avg | 0.06506 | 0.06882 | 5.8 |
| c | http_p99_ms_max | 1402 | 1354 | -3.4 |
| c | k6_http_p95_ms | 25.98 | 25.87 | -0.4 |
| c | k6_error_rate | 0.01323 | 0.01223 | — |
| c | jvm_jar_hash_us_avg | — | 400.5 | — |
| c | jvm_jar_hash_us_max | — | 536 | — |
| c | jvm_attest_total_us_avg | — | 613.8 | — |
| c | jvm_attest_total_us_max | — | 805 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260730-171948`
