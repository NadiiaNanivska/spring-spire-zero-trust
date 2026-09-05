# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-07-30T16:17:20Z

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
| a | attestation_avg_ms | 4.745 | 4.531 | -4.5 |
| a | agent_cpu_cores_avg | 0.003696 | 0.004366 | 18.1 |
| a | agent_memory_mb_avg | 15.68 | 29.88 | 90.6 |
| a | server_cpu_cores_avg | 0.007267 | 0.006977 | -4.0 |
| a | server_memory_mb_avg | 170.4 | 171 | 0.3 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0.07182 | 0.04 | -44.3 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 1.636e+04 | — |
| a | jvm_jar_hash_us_max | — | 4.935e+04 | — |
| a | jvm_attest_total_us_avg | — | 1.665e+04 | — |
| a | jvm_attest_total_us_max | — | 4.973e+04 | — |
| b | attestation_avg_ms | 4.353 | 14.56 | 234.5 |
| b | agent_cpu_cores_avg | 0.003334 | 0.005059 | 51.8 |
| b | agent_memory_mb_avg | 23.79 | 32.28 | 35.7 |
| b | server_cpu_cores_avg | 0.01758 | 0.01741 | -0.9 |
| b | server_memory_mb_avg | 170.6 | 172.1 | 0.8 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.07013 | 0.0789 | 12.5 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 1.537e+04 | — |
| b | jvm_jar_hash_us_max | — | 4.809e+04 | — |
| b | jvm_attest_total_us_avg | — | 1.592e+04 | — |
| b | jvm_attest_total_us_max | — | 4.837e+04 | — |
| c | attestation_avg_ms | 1.921 | 2.498 | 30.0 |
| c | agent_cpu_cores_avg | 0.003037 | 0.003238 | 6.6 |
| c | agent_memory_mb_avg | 30.1 | 35.39 | 17.6 |
| c | server_cpu_cores_avg | 0.008088 | 0.008812 | 9.0 |
| c | server_memory_mb_avg | 171 | 171.7 | 0.4 |
| c | http_p95_ms_avg | 33.31 | 30.69 | -7.9 |
| c | http_p99_ms_avg | 131.5 | 134.5 | 2.3 |
| c | http_5xx_rate_avg | 0.9671 | 0 | -100.0 |
| c | svid_issued_rate_avg | 0.06582 | 0.06191 | -5.9 |
| c | http_p99_ms_max | 1334 | 1407 | 5.5 |
| c | k6_http_p95_ms | 25.7 | 25.8 | 0.4 |
| c | k6_error_rate | 0.0131 | 0.01368 | — |
| c | jvm_jar_hash_us_avg | — | 874 | — |
| c | jvm_jar_hash_us_max | — | 1804 | — |
| c | jvm_attest_total_us_avg | — | 3415 | — |
| c | jvm_attest_total_us_max | — | 8718 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260730-145507`
