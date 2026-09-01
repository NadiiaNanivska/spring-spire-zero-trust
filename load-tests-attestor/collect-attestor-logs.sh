#!/usr/bin/env bash
# Parse JVM attestor timing lines from spire-agent logs.
# Usage: collect-attestor-logs.sh <start_unix> <end_unix> <label> [output_dir]
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

START=${1:?start unix required}
END=${2:?end unix required}
LABEL=${3:?label required}
OUT_BASE=${4:-${RESULTS_DIR:-$LIB_DIR/results}}

OUT_DIR="$OUT_BASE/$LABEL"
mkdir -p "$OUT_DIR"
CSV="$OUT_DIR/attestor-timing.csv"
RAW="$OUT_DIR/attestor-timing-raw.log"

duration_sec=$((END - START + 60))
if [[ $duration_sec -lt 60 ]]; then
  duration_sec=60
fi
since="${duration_sec}s"

log "Collecting spire-agent logs (since=$since) -> $CSV"

kubectl logs -n "$K8S_NAMESPACE" daemonset/spire-agent -c spire-agent --since="$since" 2>/dev/null | tee "$RAW" | \
  grep -F 'jvm attestation timing' >"$OUT_DIR/attestor-timing-grep.log" || true

{
  echo "timestamp,pod,pid,total_us,anti_debug_us,anti_tamper_us,jar_hash_us,selectors"
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    # Example: ... level=info msg="jvm attestation timing" pid=123 total_us=450 anti_debug_us=10 ...
    pid=$(echo "$line" | sed -n 's/.*pid=\([0-9]*\).*/\1/p')
    total=$(echo "$line" | sed -n 's/.*total_us=\([0-9]*\).*/\1/p')
    anti_debug=$(echo "$line" | sed -n 's/.*anti_debug_us=\([0-9]*\).*/\1/p')
    anti_tamper=$(echo "$line" | sed -n 's/.*anti_tamper_us=\([0-9]*\).*/\1/p')
    jar_hash=$(echo "$line" | sed -n 's/.*jar_hash_us=\([0-9]*\).*/\1/p')
    selectors=$(echo "$line" | sed -n 's/.*selectors=\([0-9]*\).*/\1/p')
    ts=$(echo "$line" | sed -n 's/^\([^ ]*\).*/\1/p')
    echo "${ts},spire-agent,${pid:-},${total:-},${anti_debug:-},${anti_tamper:-},${jar_hash:-},${selectors:-}"
  done <"$OUT_DIR/attestor-timing-grep.log"
} >"$CSV"

count=$(($(wc -l <"$CSV") - 1))
log "Parsed $count attestor timing rows -> $CSV"
