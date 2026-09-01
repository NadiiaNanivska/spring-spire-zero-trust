#!/usr/bin/env bash
# S-C: Rolling restart under steady HTTP load (re-attestation storm).
# Usage: scenario-c.sh <results_subdir>
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:?results subdir required}
RATE=${RPS:-100}
DURATION=${SCENARIO_C_DURATION:-10m}
mkdir -p "$OUT"

OVERLAY_NAME=""
if [[ -f "$OUT/meta.env" ]]; then
  # shellcheck disable=SC1090
  source "$OUT/meta.env"
  OVERLAY_NAME="${OVERLAY:-}"
fi

ensure_load_target

# `wsldev app deploy` (scenario B) registers JVM selectors even on the default
# overlay. Re-apply the correct entry shape before rollout restarts under load.
ensure_spire_entries_for_overlay "${OVERLAY_NAME:-custom-jvm}"
settle_workloads_after_entries

START=$(utc_now)
log "Scenario C: steady load + rollout restart orders and payments"

run_k6_steady "$DURATION" "$RATE" "$OUT" &
K6_PID=$!

sleep 90
log "Rollout restart payments-service"
kubectl rollout restart deployment/payments-service -n "$K8S_NAMESPACE"
kubectl rollout status deployment/payments-service -n "$K8S_NAMESPACE" --timeout=600s || true

sleep 30
log "Rollout restart orders-service"
kubectl rollout restart deployment/orders-service -n "$K8S_NAMESPACE"
kubectl rollout status deployment/orders-service -n "$K8S_NAMESPACE" --timeout=600s || true

wait "$K6_PID" 2>/dev/null || true
END=$(utc_now)
record_window "$OUT" "$START" "$END"
log "Scenario C complete: window $START..$END"
