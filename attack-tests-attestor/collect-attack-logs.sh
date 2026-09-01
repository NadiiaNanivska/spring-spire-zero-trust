#!/usr/bin/env bash
# Collect spire-agent logs relevant to JVM attack/resilience tests.
# Usage: collect-attack-logs.sh <label> [output_dir] [since_duration] [pod_name]
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

LABEL=${1:?label required}
OUT_BASE=${2:-${RESULTS_DIR:-$LIB_DIR/results}}
SINCE=${3:-${SUBTEST_LOG_SINCE:-120s}}
POD_FILTER=${4:-}

OUT_DIR="$OUT_BASE/$LABEL"
mkdir -p "$OUT_DIR"

RAW="$OUT_DIR/agent-raw.log"
GREP="$OUT_DIR/agent-attestor.log"
POD_SCOPED="$OUT_DIR/agent-attestor-pod.log"

log "Collecting spire-agent logs (since=$SINCE pod=${POD_FILTER:-all}) -> $GREP"

kubectl logs -n "$K8S_NAMESPACE" daemonset/spire-agent -c spire-agent --since="$SINCE" 2>/dev/null | tee "$RAW" | \
  grep -E 'jvm attestation|checker failed|debug_clean|agent_flags_clean|jar-hash|FailedPrecondition|attach_socket|inode_consistent|suspicious_|tracer_pid|maps_verified|jar_sha256|not a JVM|No identity issued|PID attested' \
  >"$GREP" || true

if [[ -n "$POD_FILTER" ]]; then
  # Write a pod-scoped view for humans, but DO NOT overwrite the full grep with it.
  # Denial lines emitted before selectors are attached (e.g. anti-tamper "checker
  # failed" / "FailedPrecondition" for the attach-socket case) carry only pid=<host>,
  # not pod-name. Narrowing agent-attestor.log to pod-name lines would drop them, and
  # assert_log_contains_for_pod's host-PID correlation fallback (which needs those
  # lines) would then find nothing -> false FAIL even though the defense worked. Keep
  # the full attestor grep; the asserts scope themselves by pod-name and host PID.
  grep -F "pod-name:${POD_FILTER}" "$GREP" >"$POD_SCOPED" 2>/dev/null || true
  if [[ ! -s "$POD_SCOPED" ]]; then
    log "WARN: no pod-name lines for $POD_FILTER in $GREP (correlation falls back to host PID)"
  fi
fi

count=$(wc -l <"$GREP" | tr -d ' ')
log "Collected $count attestor-related log lines -> $GREP"
