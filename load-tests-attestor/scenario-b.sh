#!/usr/bin/env bash
# S-B: Cold hash-compute cycles (no HTTP load).
#
# Mirrors S-A's cycle structure, but instead of only bouncing pods (which hit
# the plugin's warm hash cache), each cycle runs the full
#   redeploy (new jar) -> sync expected hashes -> register SPIRE entries ->
#   restart spire-agent
# pipeline via `wsldev app deploy` (+ ensure_spire_entries as a hard check).
# Restarting the agent clears the plugin's in-memory HashCache, so the next
# attestation recomputes the jar SHA-256 on the COLD path -- unlike S-A.
#
# Because each cycle restarts spire-agent (deleting the previous agent pod and
# its logs), agent logs are collected at the END OF EACH CYCLE -- before the
# next redeploy wipes them -- then merged into the top-level attestor-timing.*
# files that compare.sh reads. A single end-of-scenario collect (as for S-A/S-C)
# would only capture the last cycle.
#
# Usage: scenario-b.sh <results_subdir>
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:?results subdir required}
CYCLES=${COLD_COMPUTE_CYCLES:-3}
POST_DEPLOY_SETTLE=${POST_DEPLOY_SETTLE:-45}
mkdir -p "$OUT"

wsldev_bin="$(resolve_wsldev)"

# The jvm plugin runs only on the custom-jvm overlay, so per-cycle agent-log
# capture is meaningful only there. run-all.sh writes meta.env before invoking
# this script; a standalone run (no meta.env) defaults to collecting.
OVERLAY_NAME=""
if [[ -f "$OUT/meta.env" ]]; then
  # shellcheck disable=SC1090
  source "$OUT/meta.env"
  OVERLAY_NAME="${OVERLAY:-}"
fi
COLLECT_LOGS=1
[[ "$OVERLAY_NAME" == "default" ]] && COLLECT_LOGS=0

# merge_cycle_logs concatenates the per-cycle captures under $OUT/cycles/cycle-*
# into the flat attestor-timing.{csv,raw.log,grep.log} that compare.sh expects,
# so the summary aggregates cold-compute timing across ALL cycles.
merge_cycle_logs() {
  local out=$1
  local csv="$out/attestor-timing.csv"
  local raw="$out/attestor-timing-raw.log"
  local grep_out="$out/attestor-timing-grep.log"
  local header_written=0 d

  : >"$raw"
  : >"$grep_out"
  : >"$csv"

  for d in "$out"/cycles/cycle-*; do
    [[ -d "$d" ]] || continue
    [[ -f "$d/attestor-timing-raw.log" ]] && cat "$d/attestor-timing-raw.log" >>"$raw"
    [[ -f "$d/attestor-timing-grep.log" ]] && cat "$d/attestor-timing-grep.log" >>"$grep_out"
    if [[ -f "$d/attestor-timing.csv" ]]; then
      if [[ $header_written -eq 0 ]]; then
        head -n 1 "$d/attestor-timing.csv" >>"$csv"
        header_written=1
      fi
      tail -n +2 "$d/attestor-timing.csv" >>"$csv"
    fi
  done

  if [[ $header_written -eq 0 ]]; then
    echo "timestamp,pod,pid,total_us,anti_debug_us,anti_tamper_us,jar_hash_us,selectors" >"$csv"
  fi

  local rows
  rows=$(($(wc -l <"$csv") - 1))
  log "Merged $rows attestor timing rows from $CYCLES cycle(s) -> $csv"
}

START=$(utc_now)
log "Scenario B: cold hash-compute ($CYCLES redeploy/sync/restart cycles)"

for ((c = 1; c <= CYCLES; c++)); do
  cycle_start=$(utc_now)
  log "Cycle $c/$CYCLES: wsldev app deploy payments orders (rebuild jar + sync hashes + register entries + restart agent)"
  # `wsldev app deploy` rebuilds each jar (new inode/mtime), updates
  # jvm-hashes-configmap.yaml, registers jvm:jar_sha256 SPIRE entries, and
  # restarts spire-agent -- clearing the plugin hash cache for a cold compute.
  (cd "$REPO_ROOT" && "$wsldev_bin" app deploy payments orders)

  ensure_spire_entries_for_overlay "${OVERLAY_NAME:-custom-jvm}"

  wait_deployment_ready orders-service
  wait_deployment_ready payments-service

  # Let the workloads reconnect to the freshly restarted agent and be
  # re-attested on the cold cache (the plugin recomputes the jar hash here).
  log "Cycle $c/$CYCLES: settle ${POST_DEPLOY_SETTLE}s for cold re-attestation"
  sleep "$POST_DEPLOY_SETTLE"

  cycle_end=$(utc_now)
  echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) cycle=$c redeployed=1" >>"$OUT/cold-compute-events.log"

  # Capture this cycle's agent logs NOW: the next cycle's redeploy restarts
  # spire-agent and deletes this pod (and its logs) via rollout.
  if [[ $COLLECT_LOGS -eq 1 ]]; then
    log "Cycle $c/$CYCLES: collecting agent logs before next restart"
    "$LIB_DIR/collect-attestor-logs.sh" "$cycle_start" "$cycle_end" "cycles/cycle-$c" "$OUT" || \
      log "WARN: cycle $c agent-log collection failed"
  fi
done

END=$(utc_now)
record_window "$OUT" "$START" "$END"

if [[ $COLLECT_LOGS -eq 1 ]]; then
  merge_cycle_logs "$OUT"
fi

log "Scenario B complete: window $START..$END"
