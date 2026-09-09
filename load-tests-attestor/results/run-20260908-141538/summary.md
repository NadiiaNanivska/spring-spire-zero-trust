# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-09-08T14:57:19Z

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
| a | attestation_avg_ms | 3.903 | 18.47 | 373.2 |
| a | agent_cpu_cores_avg | 0.004625 | 0.005237 | 13.2 |
| a | agent_memory_mb_avg | 34.23 | 34.66 | 1.2 |
| a | server_cpu_cores_avg | 0.01324 | 0.01123 | -15.2 |
| a | server_memory_mb_avg | 171.2 | 173.6 | 1.4 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0.05035 | 0.06942 | 37.9 |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 2.035e+04 | — |
| a | jvm_jar_hash_us_max | — | 4.3e+04 | — |
| a | jvm_attest_total_us_avg | — | 2.062e+04 | — |
| a | jvm_attest_total_us_max | — | 4.321e+04 | — |
| b | attestation_avg_ms | 5.028 | 14.95 | 197.3 |
| b | agent_cpu_cores_avg | 0.003419 | 0.005263 | 54.0 |
| b | agent_memory_mb_avg | 25.98 | 32.1 | 23.6 |
| b | server_cpu_cores_avg | 0.01637 | 0.01587 | -3.0 |
| b | server_memory_mb_avg | 173.2 | 173.2 | 0.0 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.06429 | 0.06212 | -3.4 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 1.652e+04 | — |
| b | jvm_jar_hash_us_max | — | 6.207e+04 | — |
| b | jvm_attest_total_us_avg | — | 1.689e+04 | — |
| b | jvm_attest_total_us_max | — | 6.256e+04 | — |
| c | attestation_avg_ms | 1.761 | 10.09 | 472.8 |
| c | agent_cpu_cores_avg | 0.003385 | 0.003352 | -1.0 |
| c | agent_memory_mb_avg | 31.14 | 36.06 | 15.8 |
| c | server_cpu_cores_avg | 0.008769 | 0.007779 | -11.3 |
| c | server_memory_mb_avg | 172 | 171.6 | -0.2 |
| c | http_p95_ms_avg | 31.2 | 35.16 | 12.7 |
| c | http_p99_ms_avg | 137.2 | 141.6 | 3.2 |
| c | http_5xx_rate_avg | 0 | 1.591 | — |
| c | svid_issued_rate_avg | 0.07002 | 0.0598 | -14.6 |
| c | http_p99_ms_max | 1416 | 1414 | -0.1 |
| c | k6_http_p95_ms | 25.81 | 25.85 | 0.2 |
| c | k6_error_rate | 0.01454 | 0.01423 | — |
| c | jvm_jar_hash_us_avg | — | 2.493e+04 | — |
| c | jvm_jar_hash_us_max | — | 4.894e+04 | — |
| c | jvm_attest_total_us_avg | — | 2.528e+04 | — |
| c | jvm_attest_total_us_max | — | 4.932e+04 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260908-141538`
