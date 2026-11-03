#!/usr/bin/env bash
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

  # Assert re-attestation denial; an existing SVID and pooled connection can still serve mTLS.
  # The plugin never hard-fails on the attach socket: it emits jvm:attach_socket=true and
  # the entry (pinned attach_socket=false) stops matching, so denial shows up in SPIRE's
  # own "No identity issued ... registered=false" line, not in a plugin error.
  assert_denied_by_attestor_log_only "$pod" "$log_file"
}

if ! run_test_wrapper "level2-attach-socket" "PASS" "$OUT_TEST" test_body; then
  restore_clean_deployments
  exit 1
fi

restore_clean_deployments
log "Level 2 attach-socket test finished"
