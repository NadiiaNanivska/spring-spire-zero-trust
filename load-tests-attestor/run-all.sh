#!/usr/bin/env bash
# Master orchestrator: overlay x scenario load tests + metrics export + summary.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OVERLAYS=(default custom-jvm)
SCENARIOS=(a b c)
RUN_TS=$(date -u +%Y%m%d-%H%M%S)
RESULTS_DIR="${RESULTS_DIR:-$LIB_DIR/results/run-$RUN_TS}"

usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  --overlays LIST    Comma-separated: default,custom-jvm (default: both)
  --scenarios LIST   Comma-separated: a,b,c (default: a,b,c; d still selectable)
  --results-dir DIR  Output root (default: load-tests-attestor/results/run-<timestamp>)
  -h, --help         Show help

Environment: RPS, WARMUP_SEC, COLD_START_CYCLES, COLD_COMPUTE_CYCLES,
             POST_DEPLOY_SETTLE, WSLDEV_BIN, ORDERS_URL
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --overlays)
      IFS=',' read -ra OVERLAYS <<<"$2"
      shift 2
      ;;
    --scenarios)
      IFS=',' read -ra SCENARIOS <<<"$2"
      shift 2
      ;;
    --results-dir)
      RESULTS_DIR=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

mkdir -p "$RESULTS_DIR"
export RESULTS_DIR

trap cleanup_background EXIT

log "Results directory: $RESULTS_DIR"
preflight
ensure_workload_apps
port_forward_prometheus

for overlay in "${OVERLAYS[@]}"; do
  deploy_overlay "$overlay"

  for scenario in "${SCENARIOS[@]}"; do
    scenario_lower=$(echo "$scenario" | tr '[:upper:]' '[:lower:]')
    case "$scenario_lower" in
      a|b|c|d) ;;
      *) die "invalid scenario: $scenario (use a,b,c,d)" ;;
    esac

    label="${overlay}-scenario-${scenario_lower}-${RUN_TS}"
    subdir="$RESULTS_DIR/$label"
    mkdir -p "$subdir"
    write_run_meta "$subdir" "$overlay" "$scenario_lower"

    log "======== Run: overlay=$overlay scenario=$scenario_lower label=$label ========"
    reset_apps_replicas 1
    warmup_sleep "$WARMUP_SEC"

    "$LIB_DIR/scenario-${scenario_lower}.sh" "$subdir"

    # shellcheck disable=SC1090
    source "$subdir/window.env"
    "$LIB_DIR/metrics-export.sh" "$START" "$END" "$label" "$RESULTS_DIR"

    if [[ "$overlay" == "custom-jvm" ]]; then
      # S-B restarts spire-agent every cycle, so it collects & merges agent logs
      # per-cycle itself (the final pod's logs alone would miss earlier cycles).
      # For scenarios where the agent stays alive (A, C) collect once here.
      if [[ -f "$subdir/attestor-timing.csv" ]]; then
        log "Agent logs already collected per-cycle by scenario; skipping final collect for $label"
      else
        "$LIB_DIR/collect-attestor-logs.sh" "$START" "$END" "$label" "$RESULTS_DIR"
      fi
    fi

    log "Finished $label (Grafana window: from=$((START * 1000)) to=$((END * 1000)))"
  done
done

"$LIB_DIR/compare.sh" "$RESULTS_DIR"
log "All runs complete. See $RESULTS_DIR/summary.md"
