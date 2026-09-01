# Attestor Overhead Load Tests

Automated comparison of SPIRE **default** (k8s/unix attestors only) vs **custom-jvm** (JVM integrity plugin) on the `spire` mTLS profile.

## Prerequisites

- Kind (or other) cluster with SPIRE and Prometheus in namespace `spire`
- `kubectl`, `curl`, `jq`, `wsldev` (or `wsldev/wsldev` built from repo)
- `k6` on the host **only** for `K6_MODE=local`; the default in-cluster mode runs k6 as a Pod
- `wsldev observability prometheus-deploy` if Prometheus is not installed
- **orders Service**: scripts auto-apply [`orders-service/order-service-svc.yaml`](../orders-service/order-service-svc.yaml) if missing

`run-all.sh` bootstraps workloads automatically: if `orders-service` / `payments-service`
are missing it runs `wsldev app deploy payments orders`. When the **custom-jvm** overlay
is applied it also runs `wsldev spire register-jvm` so SPIRE entries pin the expected
`jvm:jar_sha256` hashes (the plugin only computes hashes; the server enforces them).

## Load generation modes

`kubectl port-forward` binds to a single pod and dies when that pod is replaced (scenario C rollout / scenario B scale), which shows up as `connection refused` in k6. To avoid this:

- **`K6_MODE=incluster` (default)** — k6 runs as a Pod hitting `orders-service.spire.svc.cluster.local:8080`. kube-proxy always routes to Ready endpoints, so it survives rollouts/scale with no host port-forward. Requires the `grafana/k6` image to be pullable (or preload with `kind load docker-image grafana/k6:0.49.0`).
- **`K6_MODE=local`** — k6 runs on the host against `ORDERS_URL`; the scripts start a supervised port-forward to `svc/orders-service`. Use only if you cannot run k6 in-cluster.

## Quick start

```bash
cd load-tests-attestor
chmod +x *.sh
./run-all.sh
```

For a fresh cluster or standalone scenario runs, bootstrap once:

```bash
./setup.sh
```

Partial run (e.g. only the cold-compute cycles on the custom plugin):

```bash
./run-all.sh --overlays custom-jvm --scenarios b
```

> S-B runs `wsldev app deploy payments orders` once per cycle, which does a full
> `mvn package` + `docker build` + `kind load` for **both** services. That is
> minutes per cycle — keep `COLD_COMPUTE_CYCLES` small (default 3) and expect the
> S-B leg to take a while. It requires the dev toolchain (mvn, docker, kind,
> wsldev), same as `wsldev app deploy`.

## Scenarios

| ID | Script | What it measures |
|----|--------|------------------|
| S-A | `scenario-a.sh` | Pod-restart cycles: scale apps 0→1 (×10), no HTTP load. Jar unchanged + agent kept alive ⇒ attestation hits the plugin's **warm hash cache** (cache path). |
| S-B | `scenario-b.sh` | **Cold hash-compute** cycles, no HTTP load. Each cycle runs `wsldev app deploy` (rebuild jar → update expected hashes → register SPIRE entries → restart spire-agent), clearing the plugin cache so the next attestation **recomputes** the jar SHA-256 (compute path). |
| S-C | `scenario-c.sh` | Steady HTTP load + rollout restart both apps (re-attestation storm). |

> S-A vs S-B isolate the two jar-hash paths: **warm cache** (S-A) vs **cold compute** (S-B). Compare their `jvm_jar_hash_us_*` rows (microseconds) in `summary.md` to read the cache benefit.
>
> `scenario-d.sh` (steady-state control) is retained but no longer part of the default run; select it explicitly with `--scenarios d`.

## Output

- `results/run-<timestamp>/<overlay>-scenario-<id>-<timestamp>/` — window.env, k6 logs, prometheus JSON, attestor-timing.csv (custom-jvm)
- `results/run-<timestamp>/summary.csv` and `summary.md` — default vs custom-jvm comparison

## Charts (standalone, not part of run-all)

`plot-results.sh` turns the raw Prometheus JSON into charts + tidy CSV. Requires `python3` + `matplotlib` (`python3 -m pip install matplotlib`).

```bash
./plot-results.sh                       # newest results/run-* dir
./plot-results.sh results/run-<ts>      # a specific run
```

Produces, under `results/run-<ts>/`:

- `plots/scenario-<id>/<metric>.png` — time series, default vs custom-jvm overlaid (elapsed seconds on x-axis)
- `plots/aggregate-<metric>.png` — bar charts per scenario (mean over window)
- `long.csv` — tidy `scenario,overlay,metric,series,elapsed_s,value` for Excel/pandas

The run dir should contain both `default-scenario-*` and `custom-jvm-scenario-*` subdirs for side-by-side charts; missing metrics/overlays are skipped.

## Multi-run aggregation (thesis tables + CI plots)

For statistically valid comparison across **N independent** `run-all.sh` executions, use `aggregate-multi.sh`. Each run directory is one replicate; 5-second Prometheus samples are **not** treated as independent observations.

```bash
# Run the full suite 3 times (recommended minimum), then aggregate:
./run-all.sh
./run-all.sh
./run-all.sh

./aggregate-multi.sh --last 3
# or explicitly:
./aggregate-multi.sh results/run-<ts1> results/run-<ts2> results/run-<ts3>
```

Requires `python3` + `pandas` + `matplotlib` + `seaborn`.

Produces under `results/aggregate-<utc-ts>/`:

| File | Purpose |
|------|---------|
| `run_scalars.csv` | One scalar per (run, scenario, overlay, metric) — raw replicates |
| `summary_stats.csv` | Mean, std, median, min, max, **95% t-CI** across runs (run-level) |
| `results_tables.html` | Thesis tables, run-level t-CI (copy into Word) |
| `pooled_stats.csv` | **Pooled** per (scenario, overlay, metric, series): n, median (p50), min, max, **Mann-Whitney U + p-value** |
| `results_tables_pooled.html` | Thesis tables, pooled: Метрика │ Плагін │ Медіана (p50) │ Мінімальне │ Максимальне │ p-value (Mann-Whitney) |
| `long_all.csv` | Raw time series from all runs (column `run`) |
| `plots_clean/` | Time-series charts with **run-to-run** 95% t-CI bands |
| `plots_clean_box/` | **Box plots** ("ящик з вусами") of pooled samples, default vs custom-jvm, outliers shown |

### Two statistical views (both produced)

`aggregate-multi.sh` emits **two independent analyses** of the same runs:

1. **Run-level t-CI** (`summary_stats.csv`, `results_tables.html`, `plots_clean/`) — each
   run is collapsed to one scalar; N = number of runs; comparison via mean ± 95% t-CI.
   Reliable only at N ≥ 5; a t-CI on N = 3 is very wide.
2. **Pooled samples + Mann-Whitney** (`pooled_stats.csv`, `results_tables_pooled.html`,
   `plots_clean_box/`) — see below. Preferred for small numbers of runs.

### Pooled samples + Mann-Whitney U (preferred for small N)

Averaging inside a run hides the tail. Example: a run of 1000 attestations where 990
take 2 ms and 10 hit a GC pause at 200 ms averages to ~3.9 ms — the chart looks clean,
but those 10 slow requests are exactly what would time out in production. The mean
"smoothed the problem away".

Instead, **pool every raw Prometheus scrape sample from every run** into one array per
`(scenario, overlay, metric, service)`. 7 runs × ~1000 samples ⇒ N = 7000, not N = 7.
A run with more samples automatically contributes proportionally more points, so pooling
is self-weighting. On the pooled array we report:

- **median (p50), min, max** — the honest distribution, tails included;
- **Mann-Whitney U** (rank-based, distribution-free) comparing `default` vs `custom-jvm`.
  It assumes neither normality nor N = 3 replicates the way a t-CI does. Implemented in
  `pooled_stats.py` with a tie-corrected normal approximation + continuity correction
  (no scipy).
- a **box plot** per metric that draws the median, IQR, whiskers, and **every** tail
  sample as an outlier flier — so GC micro-delays are visible, not averaged out.

Run standalone (needs only `long_all.csv`; the table step is pure stdlib):

```bash
python3 pooled_stats.py results/aggregate-<ts>            # tables + CSV
python3 spire_metrics_visualization.py results/aggregate-<ts>   # t-CI plots + box plots
```

> Note: 5 s scrape samples are autocorrelated (not strictly i.i.d.), so the Mann-Whitney
> p-values are indicative of distributional difference, not a formal per-request test.
> Per-service latency metrics are kept per service and never blended.

### Statistical design

- **Unit of replication** = one `run-all.sh` execution (not a pod, not a 5 s scrape).
- **N = 3** is a minimum; confidence intervals will be wide (t\_{0.975, df=2} = 4.303). Prefer N ≥ 5 when possible.
- **Estimators** (same as `plot_results.py`):
  - `attestation_avg_ms` — count-weighted Δsum/Δcount (not a simple mean of the rate gauge).
  - `http_5xx_rate` — trapezoid integral over the **full** measurement window.
  - `agent_cpu`, `agent_memory_mb` — mean over window, active pods only (last 30 s).
  - `http_req_p95_ms`, `http_req_p99_ms`, `jvm_heap_bytes` — reported **per service** (never blended).
- **95% CI** = mean ± t(0.975, df=N−1) × std / √N (two-tailed, no scipy).
- Time-series CI bands show variability **across runs** at each elapsed second, after collapsing pods to one value per run.

### Single-run clean plots (peak/spike figures)

`plot_results.py` draws **one line per pod** (a SPIRE agent DaemonSet + pod restarts = many noisy lines). For thesis figures that show spikes from a single representative run, use the clean single-run mode instead — it filters dead pods and collapses to **one line per overlay** while preserving peaks (no CI band, since it is one run):

```bash
python3 spire_metrics_visualization.py --run-dir results/run-<ts>
```

Output: `results/run-<ts>/plots_clean_single/scenario-<id>/<metric>.png`.

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RPS` | 100 | k6 arrival rate (S-C) |
| `WARMUP_SEC` | 60 | Warmup before each scenario |
| `COLD_START_CYCLES` | 10 | Scenario A (warm-cache) cycles |
| `COLD_COMPUTE_CYCLES` | 3 | Scenario B redeploy/sync/restart cycles |
| `POST_DEPLOY_SETTLE` | 45 | Seconds to wait for cold re-attestation after each S-B cycle |
| `SCENARIO_C_DURATION` | 10m | k6 duration for C |
| `MAX_REPLICAS` | 12 | Scenario D scale target (control, off by default) |
| `SCENARIO_D_DURATION` | 10m | k6 duration for D (control, off by default) |
| `WSLDEV_BIN` | auto | Path to wsldev binary |
| `K6_MODE` | incluster | `incluster` (k6 as Pod) or `local` (host k6 + port-forward) |
| `ORDERS_INCLUSTER_URL` | http://orders-service.spire.svc.cluster.local:8080 | Target for in-cluster k6 |
| `K6_IMAGE` | grafana/k6:0.49.0 | k6 image for in-cluster mode |
| `ORDERS_URL` | http://127.0.0.1:8080 | Orders API for `K6_MODE=local` |
| `JVM_ENTRY_SETTLE_SEC` | 30 | Wait after SPIRE entry changes before load / rollout (scenario C) |

> **PERMISSION_DENIED / no identity issued** in scenario C usually means SPIRE entries
> do not match the active overlay: the default agent emits only `k8s`/`unix` selectors,
> but `wsldev app deploy` registers JVM selectors. Scripts now re-apply the correct
> entry set before each scenario C (and after each S-B cycle on the default overlay).
