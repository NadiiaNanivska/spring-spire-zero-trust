# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-07-29T17:42:31Z

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


| scenario | metric                  | default  | custom-jvm | delta % |
| -------- | ----------------------- | -------- | ---------- | ------- |
| a        | attestation_avg_ms      | 6.745    | 6          | -11.0   |
| a        | agent_cpu_cores_avg     | 0.005763 | 0.005538   | -3.9    |
| a        | agent_memory_mb_avg     | 32.73    | 32.56      | -0.5    |
| a        | server_cpu_cores_avg    | 0.01183  | 0.01394    | 17.9    |
| a        | server_memory_mb_avg    | 172.2    | 173        | 0.5     |
| a        | http_p95_ms_avg         | —        | —          | —       |
| a        | http_p99_ms_avg         | —        | —          | —       |
| a        | http_5xx_rate_avg       | —        | —          | —       |
| a        | svid_issued_rate_avg    | 0.07507  | 0.0721     | -4.0    |
| a        | http_p99_ms_max         | —        | —          | —       |
| a        | jvm_jar_hash_us_avg     | —        | 620.6      | —       |
| a        | jvm_jar_hash_us_max     | —        | 1173       | —       |
| a        | jvm_attest_total_us_avg | —        | 1279       | —       |
| a        | jvm_attest_total_us_max | —        | 2342       | —       |
| b        | attestation_avg_ms      | 6.722    | 14.44      | 114.8   |
| b        | agent_cpu_cores_avg     | 0.005512 | 0.005256   | -4.6    |
| b        | agent_memory_mb_avg     | 25.92    | 33.91      | 30.8    |
| b        | server_cpu_cores_avg    | 0.02088  | 0.01833    | -12.2   |
| b        | server_memory_mb_avg    | 173.1    | 173.8      | 0.4     |
| b        | http_p95_ms_avg         | —        | —          | —       |
| b        | http_p99_ms_avg         | —        | —          | —       |
| b        | http_5xx_rate_avg       | —        | —          | —       |
| b        | svid_issued_rate_avg    | 0.06962  | 0.08056    | 15.7    |
| b        | http_p99_ms_max         | —        | —          | —       |
| b        | jvm_jar_hash_us_avg     | —        | 1.514e+04  | —       |
| b        | jvm_jar_hash_us_max     | —        | 4.59e+04   | —       |
| b        | jvm_attest_total_us_avg | —        | 1.575e+04  | —       |
| b        | jvm_attest_total_us_max | —        | 4.636e+04  | —       |
| c        | attestation_avg_ms      | 3.327    | 2.881      | -13.4   |
| c        | agent_cpu_cores_avg     | 0.00441  | 0.003232   | -26.7   |
| c        | agent_memory_mb_avg     | 30.73    | 36.42      | 18.5    |
| c        | server_cpu_cores_avg    | 0.01281  | 0.008771   | -31.5   |
| c        | server_memory_mb_avg    | 172.3    | 172.9      | 0.3     |
| c        | http_p95_ms_avg         | 54.49    | 29.55      | -45.8   |
| c        | http_p99_ms_avg         | 256.4    | 135.7      | -47.0   |
| c        | http_5xx_rate_avg       | 1.339    | 6.024      | 349.9   |
| c        | svid_issued_rate_avg    | 0.06837  | 0.06717    | -1.8    |
| c        | http_p99_ms_max         | 2635     | 1303       | -50.6   |
| c        | k6_http_p95_ms          | 28.96    | 26.14      | -9.7    |
| c        | k6_error_rate           | 0.02777  | 0.01489    | —       |
| c        | jvm_jar_hash_us_avg     | —        | 503.2      | —       |
| c        | jvm_jar_hash_us_max     | —        | 589        | —       |
| c        | jvm_attest_total_us_avg | —        | 823        | —       |
| c        | jvm_attest_total_us_max | —        | 891        | —       |


Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260729-165811`