#!/usr/bin/env bash
# Level 3: Jar integrity — tampered jar hash no longer matches the pinned SPIRE entry.
# The plugin computes jar_sha256 from the running jar; the registration entry (with
# the pre-tamper hash from jvm-hashes-configmap.yaml) enforces the allow-list.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="level3-jar-unknown"
OUT_TEST="$OUT/$LABEL"
mkdir -p "$OUT_TEST"

test_body() {
  local log_file pod pinned_hash

  ensure_spire_entries
  pinned_hash=$(tamper_payments_jar_and_reattest)

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found after jar tamper"

  collect_agent_logs "$LABEL" "$OUT" "$SUBTEST_LOG_SINCE" "$pod"
  log_file="$OUT/$LABEL/agent-attestor.log"

  assert_jar_hash_changed_for_pod "$pod" "$log_file" "$pinned_hash"
  assert_denied_by_attestor "$pod" "$log_file" 'jar_sha256='
}

if ! run_test_wrapper "level3-jar-unknown" "PASS" "$OUT_TEST" test_body; then
  restore_spire_entries
  restore_clean_deployments
  exit 1
fi

restore_spire_entries
restore_clean_deployments
log "Level 3 jar-unknown test finished"
