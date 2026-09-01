#!/usr/bin/env bash
# Export Prometheus query_range snapshots for a test window.
# Usage: metrics-export.sh <start_unix> <end_unix> <label> [output_dir]
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

START=${1:?start unix required}
END=${2:?end unix required}
LABEL=${3:?label required}
OUT_BASE=${4:-${RESULTS_DIR:-$LIB_DIR/results}}

OUT_DIR="$OUT_BASE/$LABEL/prometheus"
mkdir -p "$OUT_DIR"

STEP="${METRICS_STEP:-5s}"
PROM="${PROMETHEUS_URL:-http://127.0.0.1:9090}"

# Each entry: output_filename|PromQL
QUERIES=(
  "attestation_elapsed_sum.json|spire_agent_workload_api_workload_attestation_elapsed_time_sum"
  "attestation_elapsed_count.json|spire_agent_workload_api_workload_attestation_elapsed_time_count"
  "attestation_elapsed_bucket.json|spire_agent_workload_api_workload_attestation_elapsed_time_bucket"
  "attestation_avg_ms.json|sum(rate(spire_agent_workload_api_workload_attestation_elapsed_time_sum[1m])) / sum(rate(spire_agent_workload_api_workload_attestation_elapsed_time_count[1m]))"
  "svid_issued_total.json|spire_server_rpc_svid_v1_svid_batch_new_x509svid"
  "svid_issued_rate.json|sum(rate(spire_server_rpc_svid_v1_svid_batch_new_x509svid[1m]))"
  "agent_cpu.json|sum by(pod) (rate(container_cpu_usage_seconds_total{container=\"spire-agent\"}[1m]))"
  "agent_memory_mb.json|sum by(pod) (container_memory_usage_bytes{container=\"spire-agent\"}) / 1024 / 1024"
  "server_cpu.json|sum by(pod) (rate(container_cpu_usage_seconds_total{container=\"spire-server\"}[1m]))"
  "server_memory_mb.json|sum by(pod) (container_memory_usage_bytes{container=\"spire-server\"}) / 1024 / 1024"
  "http_req_rate.json|sum by(service, status) (rate(http_server_requests_seconds_count{uri!~\"/actuator.*\"}[1m]))"
  "http_req_p95_ms.json|histogram_quantile(0.95, sum by(le, service) (rate(http_server_requests_seconds_bucket{uri!~\"/actuator.*\"}[1m]))) * 1000"
  "http_req_p99_ms.json|histogram_quantile(0.99, sum by(le, service) (rate(http_server_requests_seconds_bucket{uri!~\"/actuator.*\"}[1m]))) * 1000"
  "http_5xx_rate.json|sum by(service) (rate(http_server_requests_seconds_count{status=~\"5..\",uri!~\"/actuator.*\"}[1m]))"
  "http_401_403_rate.json|sum by(service) (rate(http_server_requests_seconds_count{status=~\"401|403\",uri!~\"/actuator.*\"}[1m]))"
  "jvm_heap_bytes.json|sum by(service) (jvm_memory_used_bytes{area=\"heap\"})"
  "process_cpu.json|process_cpu_usage"
  "jvm_gc_pause_rate.json|sum by(application) (rate(jvm_gc_pause_seconds_count[1m]))"
  "process_uptime.json|process_uptime_seconds"
)

log "Exporting Prometheus metrics for label=$LABEL window=$START..$END -> $OUT_DIR"

for entry in "${QUERIES[@]}"; do
  file=${entry%%|*}
  query=${entry#*|}
  dest="$OUT_DIR/$file"
  if curl -sfG "${PROM}/api/v1/query_range" \
    --data-urlencode "query=$query" \
    --data-urlencode "start=$START" \
    --data-urlencode "end=$END" \
    --data-urlencode "step=$STEP" \
    -o "$dest"; then
    status=$(jq -r '.status // "unknown"' "$dest" 2>/dev/null || echo "invalid-json")
    if [[ "$status" != "success" ]]; then
      log "WARN: query $file returned status=$status"
    fi
  else
    log "WARN: failed to fetch $file"
  fi
done

log "Prometheus export complete: $OUT_DIR"
