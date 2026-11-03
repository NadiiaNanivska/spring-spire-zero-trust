#!/usr/bin/env bash
# Provenance module check (spire-jvm-attestor/internal/checkers/provenance.go).
#
# The module verifies an Ed25519 signature stored in <jar>.sig over the jar's SHA-256 digest.
# Only a valid signature yields jvm:signed_by=... and jvm:subject_match=true; the entries
# registered by `wsldev spire register-jvm` pin both, so a payments JVM whose signature is
# corrupted, missing or symlinked must not receive an SVID. The jar itself is NOT modified,
# so jar_sha256 still matches and only the provenance module can be responsible for the denial.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="level3-provenance"
OUT_TEST="$OUT/$LABEL"
mkdir -p "$OUT_TEST"

PAYMENTS_SIG="${PAYMENTS_JAR}.sig"
SIG_MODES=(corrupt missing symlink)

# Apply one signature tamper inside the running payments pod (jar bytes stay untouched).
tamper_signature() {
  local pod=$1
  local mode=$2
  case "$mode" in
    corrupt)
      # Right size (64 bytes) so the module reaches ed25519.Verify and rejects it there.
      kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$PAYMENTS_DEPLOY" -- \
        sh -c "head -c 64 /dev/zero > '$PAYMENTS_SIG'"
      ;;
    missing)
      kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$PAYMENTS_DEPLOY" -- \
        sh -c "rm -f '$PAYMENTS_SIG'"
      ;;
    symlink)
      # Valid signature bytes, but reached through a symlink: the module opens with O_NOFOLLOW.
      kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$PAYMENTS_DEPLOY" -- \
        sh -c "cp '$PAYMENTS_SIG' /tmp/payments.jar.sig && rm -f '$PAYMENTS_SIG' && ln -s /tmp/payments.jar.sig '$PAYMENTS_SIG'"
      ;;
    *) die "unknown signature tamper mode: $mode" ;;
  esac
}

check_mode() {
  local mode=$1
  local pod label log_file

  # Fresh pod => pristine, validly signed jar for every mode.
  delete_payments_pod_and_wait
  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found for provenance mode $mode"

  kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$PAYMENTS_DEPLOY" -- \
    sh -c "[ -s '$PAYMENTS_SIG' ]" ||
    die "no signature at $PAYMENTS_SIG in $pod (jar not signed; re-run: wsldev app deploy payments with WSLDEV_JVM_SIGNING_KEY set)"

  log "[provenance:$mode] tampering $PAYMENTS_SIG in $pod"
  tamper_signature "$pod" "$mode"
  restart_spire_agent

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  label="$LABEL-$mode"
  collect_agent_logs "$label" "$OUT" "$SUBTEST_LOG_SINCE" "$pod"
  log_file="$OUT/$label/agent-attestor.log"

  # 1. The module ran and found no trusted signature (signed_by empty in its timing log).
  assert_log_contains_for_pod 'provenance stage timing.*signed_by=(""| )' "$log_file" "$pod" || return 1
  # 2. Therefore the selectors the registration entry pins were not emitted.
  assert_log_not_contains_for_pod 'subject_match=true' "$log_file" "$pod" || return 1
  # 3. And SPIRE refused to issue an identity (live pod: use logs, pooled mTLS can outlive the SVID).
  assert_denied_by_attestor_log_only "$pod" "$log_file" 'No identity issued|registered=false' || return 1

  record_evidence_signal "provenance-${mode}-rejected"
}

test_body() {
  local mode

  ensure_spire_entries

  for mode in "${SIG_MODES[@]}"; do
    check_mode "$mode" || return 1
  done
  return 0
}

if ! run_test_wrapper "level3-provenance" "PASS" "$OUT_TEST" test_body; then
  restore_clean_deployments
  exit 1
fi

restore_clean_deployments
log "Level 3 provenance test finished"
