# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-09-10T15:57:37Z

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
| a | attestation_avg_ms | 6.293 | 31.81 | 405.5 |
| a | agent_cpu_cores_avg | 0.00561 | 0.008966 | 59.8 |
| a | agent_memory_mb_avg | 32.12 | 35.01 | 9.0 |
| a | server_cpu_cores_avg | 0.01305 | 0.01633 | 25.2 |
| a | server_memory_mb_avg | 173 | 170.6 | -1.4 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | 0 | — | — |
| a | svid_issued_rate_avg | 0.06263 | 0.06116 | -2.3 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 3.378e+04 | — |
| a | jvm_jar_hash_us_max | — | 7.098e+04 | — |
| a | jvm_attest_total_us_avg | — | 3.443e+04 | — |
| a | jvm_attest_total_us_max | — | 7.142e+04 | — |
| b | attestation_avg_ms | 7.649 | 13.84 | 80.9 |
| b | agent_cpu_cores_avg | 0.005567 | 0.008512 | 52.9 |
| b | agent_memory_mb_avg | 27.54 | 35.88 | 30.3 |
| b | server_cpu_cores_avg | 0.02386 | 0.02371 | -0.6 |
| b | server_memory_mb_avg | 172.1 | 171 | -0.6 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.06234 | 0.07127 | 14.3 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 2.292e+04 | — |
| b | jvm_jar_hash_us_max | — | 9.307e+04 | — |
| b | jvm_attest_total_us_avg | — | 2.353e+04 | — |
| b | jvm_attest_total_us_max | — | 9.349e+04 | — |
| c | attestation_avg_ms | 2.392 | 16.99 | 610.2 |
| c | agent_cpu_cores_avg | 0.004402 | 0.005309 | 20.6 |
| c | agent_memory_mb_avg | 31.18 | 38.38 | 23.1 |
| c | server_cpu_cores_avg | 0.0129 | 0.01391 | 7.8 |
| c | server_memory_mb_avg | 171.3 | 172.2 | 0.6 |
| c | http_p95_ms_avg | 67.34 | 59.06 | -12.3 |
| c | http_p99_ms_avg | 264.5 | 258.1 | -2.4 |
| c | http_5xx_rate_avg | 1.286 | 1.347 | 4.8 |
| c | svid_issued_rate_avg | 0.06358 | 0.06311 | -0.7 |
| c | http_p99_ms_max | 2589 | 2584 | -0.2 |
| c | k6_http_p95_ms | 29.24 | 29.43 | 0.6 |
| c | k6_error_rate | 0.02524 | 0.02812 | — |
| c | jvm_jar_hash_us_avg | — | 4.298e+04 | — |
| c | jvm_jar_hash_us_max | — | 8.444e+04 | — |
| c | jvm_attest_total_us_avg | — | 4.399e+04 | — |
| c | jvm_attest_total_us_max | — | 8.552e+04 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260910-151006`
