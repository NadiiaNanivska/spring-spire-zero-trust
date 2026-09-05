# Attestor overhead comparison (default vs custom-jvm)

Generated: 2026-07-21T16:47:51Z

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
| a | attestation_avg_ms | 4.378 | 4.161 | -5.0 |
| a | agent_cpu_cores_avg | 0.002872 | 0.003095 | 7.8 |
| a | agent_memory_mb_avg | 22.06 | 27.23 | 23.5 |
| a | server_cpu_cores_avg | 0.003771 | 0.003415 | -9.4 |
| a | server_memory_mb_avg | 36.68 | 37.93 | 3.4 |
| a | http_p95_ms_avg | — | — | — |
| a | http_p99_ms_avg | — | — | — |
| a | http_5xx_rate_avg | — | — | — |
| a | svid_issued_rate_avg | 0 | 0 | — |
| a | http_p99_ms_max | — | — | — |
| a | jvm_jar_hash_us_avg | — | 1288 | — |
| a | jvm_jar_hash_us_max | — | 2318 | — |
| a | jvm_attest_total_us_avg | — | 1636 | — |
| a | jvm_attest_total_us_max | — | 2836 | — |
| b | attestation_avg_ms | — | 35.06 | — |
| b | agent_cpu_cores_avg | 0.002951 | 0.004432 | 50.2 |
| b | agent_memory_mb_avg | 22.47 | 27.39 | 21.9 |
| b | server_cpu_cores_avg | 0.003299 | 0.003732 | 13.1 |
| b | server_memory_mb_avg | 36.53 | 37.71 | 3.2 |
| b | http_p95_ms_avg | — | — | — |
| b | http_p99_ms_avg | — | — | — |
| b | http_5xx_rate_avg | — | — | — |
| b | svid_issued_rate_avg | 0.01034 | 0.009861 | -4.7 |
| b | http_p99_ms_max | — | — | — |
| b | jvm_jar_hash_us_avg | — | 4.988e+04 | — |
| b | jvm_jar_hash_us_max | — | 6.056e+04 | — |
| b | jvm_attest_total_us_avg | — | 5.028e+04 | — |
| b | jvm_attest_total_us_max | — | 6.082e+04 | — |
| c | attestation_avg_ms | 1.826 | 2.474 | 35.5 |
| c | agent_cpu_cores_avg | 0.002401 | 0.002671 | 11.2 |
| c | agent_memory_mb_avg | 24.69 | 29.95 | 21.3 |
| c | server_cpu_cores_avg | 0.003398 | 0.003436 | 1.1 |
| c | server_memory_mb_avg | 36.73 | 38.18 | 4.0 |
| c | http_p95_ms_avg | 38.19 | 32.89 | -13.9 |
| c | http_p99_ms_avg | 193 | 180.5 | -6.5 |
| c | http_5xx_rate_avg | 0 | 5.407 | — |
| c | svid_issued_rate_avg | 0 | 0 | — |
| c | http_p99_ms_max | 1697 | 1610 | -5.1 |
| c | k6_http_p95_ms | 26.01 | 25.82 | -0.7 |
| c | k6_error_rate | 0.01297 | 0.01481 | — |
| c | jvm_jar_hash_us_avg | — | 2180 | — |
| c | jvm_jar_hash_us_max | — | 4061 | — |
| c | jvm_attest_total_us_avg | — | 2352 | — |
| c | jvm_attest_total_us_max | — | 4243 | — |

Raw runs under: `/mnt/c/Users/nnani/IdeaProjects/spring-spire-zero-trust/load-tests-attestor/results/run-20260721-160830`





======================================================================
Сценарій A: Масштабування з "теплим" кешем
======================================================================

### Метрика: attestation_avg_ms
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |          18 |         4.885 |           5.37  |      5.515 |
| default    |          17 |         2.157 |           2.224 |      4.378 |

### Метрика: agent_cpu
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |          18 |         0.003 |           0.003 |      0.004 |
| default    |          18 |         0.003 |           0.003 |      0.003 |

### Метрика: agent_memory_mb
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |          36 |        27.232 |          25.422 |     32.219 |
| default    |          55 |        22.059 |          21.762 |     24.566 |

======================================================================
Сценарій B: "Холодний" старт та перерахунок цілісності
======================================================================

### Метрика: attestation_avg_ms
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |    P95 |   Максимум |
|:-----------|------------:|--------------:|----------------:|-------:|-----------:|
| custom-jvm |          58 |        32.146 |          32.11  | 62.082 |     62.082 |
| default    |          54 |         4.249 |           4.416 |  7.266 |      7.266 |

### Метрика: agent_cpu
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |          70 |         0.004 |           0.004 |      0.009 |
| default    |          67 |         0.003 |           0.003 |      0.005 |

### Метрика: agent_memory_mb
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |         164 |        27.392 |          30.004 |     32.992 |
| default    |         164 |        22.473 |          23.145 |     28.465 |

### Метрика: server_cpu
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |          59 |         0.004 |           0.004 |      0.005 |
| default    |          58 |         0.003 |           0.003 |      0.004 |

======================================================================
Сценарій C: Стабільне навантаження та "шторм" переатестації
======================================================================

### Метрика: http_req_p95_ms
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |         236 |        32.894 |          27.677 |    906.939 |
| default    |         237 |        38.195 |          27.673 |   1107.02  |

### Метрика: http_req_p99_ms
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |         236 |       180.491 |          27.911 |    1610.1  |
| default    |         237 |       193.018 |          27.909 |    1697.06 |

### Метрика: http_5xx_rate
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |          16 |         5.407 |           6.942 |     11.105 |
| default    |          15 |         0     |           0     |      0     |

### Метрика: process_cpu
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |         241 |         0.013 |           0.008 |      0.111 |
| default    |         242 |         0.012 |           0.008 |      0.148 |

### Метрика: jvm_heap_bytes
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |         241 |       291.146 |         276.91  |    592.076 |
| default    |         242 |       312.726 |         305.622 |    525.393 |

### Метрика: jvm_gc_pause_rate
| Плагін     |   К-сть (N) |   Середнє (μ) |   Медіана (P50) |   Максимум |
|:-----------|------------:|--------------:|----------------:|-----------:|
| custom-jvm |         121 |         0.104 |           0.091 |      0.218 |
| default    |         122 |         0.09  |           0.073 |      0.327 |
