#!/usr/bin/env bash
# Resilience D: Large jar / cold hash compute latency (DoS resistance).
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="dos-large-jar"
OUT_TEST="$OUT/$LABEL"
mkdir -p "$OUT_TEST"

test_body() {
  local pod log_file max_us agent_phase pinned_hash
  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found"

  pinned_hash=$(get_payments_pinned_jar_hash)
  [[ -n "$pinned_hash" ]] || die "cannot read pinned payments jar hash"

  log "Appending ${LARGE_JAR_MB}MB to jar to stress SHA-256 compute"
  kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$PAYMENTS_DEPLOY" -- \
    sh -c "dd if=/dev/zero bs=1M count=${LARGE_JAR_MB} >> '$PAYMENTS_JAR' 2>/dev/null"

  restart_spire_agent

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found after agent restart"

  collect_agent_logs "$LABEL" "$OUT" "$SUBTEST_LOG_SINCE" "$pod"
  log_file="$OUT/$LABEL/agent-attestor.log"

  agent_phase=$(kubectl get pods -n "$K8S_NAMESPACE" -l app=spire-agent \
    -o jsonpath='{.items[0].status.phase}' 2>/dev/null)
  [[ "$agent_phase" == "Running" ]] || {
    log "ASSERT FAIL: spire-agent not Running (phase=$agent_phase)"
    return 1
  }
  record_evidence_signal "agent-running"

  assert_log_contains_for_pod 'jvm attestation timing|jar_hash_us=' "$log_file" "$pod"

  max_us=$(grep -o 'jar_hash_us=[0-9]*' "$log_file" 2>/dev/null | sed 's/jar_hash_us=//' | sort -n | tail -1 || echo "0")
  log "Max jar_hash_us observed: ${max_us:-0} (threshold $DOS_LATENCY_THRESHOLD_US)"

  if [[ "${max_us:-0}" -gt 0 ]] && [[ "${max_us:-0}" -lt "$DOS_LATENCY_THRESHOLD_US" ]]; then
    record_evidence_signal "jar-hash-latency-ok:${max_us}us"
    log "Latency within threshold"
  else
    log "WARN: jar_hash_us ${max_us:-0} outside threshold $DOS_LATENCY_THRESHOLD_US (agent still healthy)"
    record_evidence_signal "jar-hash-latency-high:${max_us:-0}us"
  fi

  assert_jar_hash_changed_for_pod "$pod" "$log_file" "$pinned_hash"
  record_evidence_signal "dos-survived"
  return 0
}

if ! run_test_wrapper "dos-large-jar" "PASS" "$OUT_TEST" test_body; then
  restore_clean_deployments
  exit 1
fi

restore_clean_deployments
log "DoS large-jar test finished"
