#!/usr/bin/env bash
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
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
  grep -E 'JVM attestor configured|enabled_modules|checker failed|FailedPrecondition|attach_socket|suspicious_|not a JVM|No identity issued|PID attested|registered=false|failed to attest|attestation failed' \
  >"$GREP" || true

if [[ -n "$POD_FILTER" ]]; then
  grep -F "pod-name:${POD_FILTER}" "$GREP" >"$POD_SCOPED" 2>/dev/null || true
  if [[ ! -s "$POD_SCOPED" ]]; then
    log "WARN: no pod-name lines for $POD_FILTER in $GREP (correlation falls back to host PID)"
  fi
fi

count=$(wc -l <"$GREP" | tr -d ' ')
log "Collected $count attestor-related log lines -> $GREP"
