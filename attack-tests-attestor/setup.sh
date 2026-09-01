#!/usr/bin/env bash
# Bootstrap cluster state for JVM attestor attack tests.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$(results_dir)}}
mkdir -p "$OUT"
export RESULTS_DIR="$OUT"

log "=== Attack test setup -> $OUT ==="
preflight

deploy_overlay custom-jvm
deploy_clean_apps
ensure_orders_service
ensure_spire_entries

log "Recording baseline: clean workloads should obtain SVID and complete mTLS"
collect_agent_logs "baseline" "$OUT" || true

BASELINE_LOG="$OUT/baseline/agent-attestor.log"
if assert_log_contains 'debug_clean=true|jvm attestation timing' "$BASELINE_LOG" 2>/dev/null; then
  log "Baseline log: JVM attestation observed"
else
  log "WARN: baseline JVM attestation lines not found yet (may appear after first Workload API call)"
fi

if assert_mtls_ok; then
  log "Baseline mTLS: PASS (orders->payments succeeded)"
  echo "BASELINE_MTLS=PASS" >"$OUT/baseline.env"
else
  log "WARN: baseline mTLS probe did not return 2xx (cluster may still be settling)"
  echo "BASELINE_MTLS=WARN" >"$OUT/baseline.env"
fi

kubectl exec -n "$K8S_NAMESPACE" spire-server-0 -c spire-server -- \
  /opt/spire/bin/spire-server entry show >"$OUT/spire-entries.txt" 2>&1 || true

log "Setup complete. Results: $OUT"
