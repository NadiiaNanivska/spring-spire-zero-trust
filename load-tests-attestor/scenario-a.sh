#!/usr/bin/env bash
# S-A: Cold-start attestation — scale to 0 and back up (no HTTP load).
# Usage: scenario-a.sh <results_subdir>
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:?results subdir required}
CYCLES=${COLD_START_CYCLES:-10}
mkdir -p "$OUT"

START=$(utc_now)
log "Scenario A: cold-start ($CYCLES cycles)"

for ((c = 1; c <= CYCLES; c++)); do
  log "Cycle $c/$CYCLES: scale to 0"
  reset_apps_replicas 0
  sleep 5
  log "Cycle $c/$CYCLES: scale to 1"
  reset_apps_replicas 1
  echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) cycle=$c ready=1" >>"$OUT/cold-start-events.log"
done

END=$(utc_now)
record_window "$OUT" "$START" "$END"
log "Scenario A complete: window $START..$END"
