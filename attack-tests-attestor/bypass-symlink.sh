#!/usr/bin/env bash
# Bypass B: Symlink jar path — plugin must hash the real file, not a decoy.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="bypass-symlink"
OUT_TEST="$OUT/$LABEL"
mkdir -p "$OUT_TEST"

test_body() {
  local pod log_file pinned_hash
  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found"
  pinned_hash=$(get_payments_pinned_jar_hash)

  log "Creating symlink chain: payments-service.jar -> real.jar"
  kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$PAYMENTS_DEPLOY" -- sh -c "
    cd /app
    cp payments-service.jar real.jar
    echo DECOY_PAYLOAD > decoy.jar
    rm -f payments-service.jar
    ln -s real.jar payments-service.jar
  "

  restart_spire_agent

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found after agent restart"

  collect_agent_logs "$LABEL" "$OUT" "$SUBTEST_LOG_SINCE" "$pod"
  log_file="$OUT/$LABEL/agent-attestor.log"

  assert_log_contains_for_pod 'maps_verified=true' "$log_file" "$pod"
  if [[ -n "$pinned_hash" ]]; then
    assert_log_contains_for_pod "jar_sha256=${pinned_hash}" "$log_file" "$pod"
  else
    assert_log_contains_for_pod 'jar_sha256=' "$log_file" "$pod"
  fi
  assert_mtls_ok_after_agent_restart
}

if ! run_test_wrapper "bypass-symlink" "PASS" "$OUT_TEST" test_body; then
  restore_clean_deployments
  exit 1
fi

restore_clean_deployments
log "Bypass symlink test finished"
