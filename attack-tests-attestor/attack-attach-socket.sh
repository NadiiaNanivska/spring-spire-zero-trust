#!/usr/bin/env bash
# Level 2: Anti-tamper — JVM Attach API socket (block_on_attach_socket=true).
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="level2-attach-socket"
OUT_TEST="$OUT/$LABEL"
mkdir -p "$OUT_TEST"

test_body() {
  local log_file pod pid

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found"
  pid=$(java_pid "$pod" "$PAYMENTS_DEPLOY")
  [[ -n "$pid" ]] || die "could not locate java pid inside $pod"

  create_attach_socket "$pod" "$pid" "$PAYMENTS_DEPLOY"
  restart_spire_agent

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found after agent restart"

  collect_agent_logs "$LABEL" "$OUT" "$SUBTEST_LOG_SINCE" "$pod"
  log_file="$OUT/$LABEL/agent-attestor.log"

  # Attestor-log denial is the deterministic proof here — NOT orders->payments mTLS. This
  # is a live-process injection into an already-SVID'd pod: SPIRE does not revoke the
  # cached SVID and orders reuses its keep-alive connection to payments, so mTLS stays 200
  # long past SVID expiry (that flakiness produced the prior false FAIL). We assert that
  # the agent refused re-attestation for THIS pod.
  assert_denied_by_attestor_log_only "$pod" "$log_file" 'Attach API socket|FailedPrecondition|checker failed|anti-tamper|attach_socket_exposed'
}

if ! run_test_wrapper "level2-attach-socket" "PASS" "$OUT_TEST" test_body; then
  restore_clean_deployments
  exit 1
fi

restore_clean_deployments
log "Level 2 attach-socket test finished"
