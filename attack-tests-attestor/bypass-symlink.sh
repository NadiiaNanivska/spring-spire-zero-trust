#!/usr/bin/env bash
# Bypass B: redirect the jar pathname to an attacker-controlled decoy.
#
# The JVM is already running with the real fat-jar open, so the attacker cannot
# reach the bytes it loaded. Instead they go after the NAME the attestor might
# resolve:
#
#   rm payments-service.jar                # unlink the name; the inode lives on
#   ln -s decoy.jar payments-service.jar   # the name now points at the decoy
#
# Unlinking does not touch the bytes: the JVM holds the inode open, and the kernel
# keeps resolving its descriptor to it (readlink reports the original path with a
# " (deleted)" marker, which the plugin strips). Note the attacker must unlink
# rather than rename — a rename would change the path the kernel reports for the
# descriptor, which is part of the pinned jar-set digest and would itself be caught.
#
# An attestor that resolves the pathname would open
# /proc/<pid>/root/app/payments-service.jar, follow the symlink and publish the
# DECOY's hash. That breaks integrity in both directions: an untampered workload
# loses its identity, and — with the roles reversed — a decoy holding the approved
# bytes would launder a tampered process.
#
# jvm-attestor discovers the fat-jar through /proc/<pid>/fd and reads it through
# that descriptor, which the kernel binds to the inode rather than to the name. The
# published hash must therefore still be the pinned one and mTLS must survive.
#
# NOTE: this asserts the fd-based discovery path. Run it against a plugin build that
# emits jvm:jar_source / jvm:hash_via_kernel_handle; an older path-resolving build
# is genuinely vulnerable here and will (correctly) fail.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="bypass-symlink"
OUT_TEST="$OUT/$LABEL"
mkdir -p "$OUT_TEST"

DECOY_JAR="/app/decoy.jar"

payments_exec() {
  local pod=$1
  shift
  kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$PAYMENTS_DEPLOY" -- sh -c "$*"
}

test_body() {
  local pod log_file pinned_hash decoy_hash link_target expected_set

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found"

  pinned_hash=$(get_payments_pinned_jar_hash)
  [[ -n "$pinned_hash" ]] || die "cannot read pinned payments jar hash from jvm-hashes ConfigMap"

  # The digest wsldev pinned in the registration entry: SHA-256 over one
  # "<path>:<sha256>\n" line per jar, ordered by path (one line for a clean workload).
  expected_set=$(printf '%s:%s\n' "$PAYMENTS_JAR" "$pinned_hash" | sha256sum | awk '{print $1}')

  log "Redirecting $PAYMENTS_JAR to an attacker-controlled decoy"
  payments_exec "$pod" "
    set -e
    cd /app
    echo DECOY_PAYLOAD > $(basename "$DECOY_JAR")
    rm -f $(basename "$PAYMENTS_JAR")
    ln -s $(basename "$DECOY_JAR") $(basename "$PAYMENTS_JAR")
  "

  # Negative control. Without it the test is vacuous: an `ln -s` that silently did
  # nothing would leave the original file at the original path, and every assertion
  # below would pass for the wrong reason. This is exactly how the previous version
  # of this test could report PASS while never exercising the decoy at all.
  link_target=$(payments_exec "$pod" "readlink $PAYMENTS_JAR 2>/dev/null || true" | tr -d '\r')
  if [[ "$link_target" != "$(basename "$DECOY_JAR")" ]]; then
    log "ASSERT FAIL (inconclusive): $PAYMENTS_JAR does not point at the decoy (readlink='$link_target')"
    return 1
  fi
  record_evidence_signal "decoy-symlink-in-place"

  decoy_hash=$(payments_exec "$pod" "sha256sum $DECOY_JAR" | awk '{print $1}' | tr -d '\r')
  if [[ -z "$decoy_hash" ]]; then
    log "ASSERT FAIL: cannot compute decoy hash in pod $pod"
    return 1
  fi
  if [[ "$decoy_hash" == "$pinned_hash" ]]; then
    log "ASSERT FAIL (inconclusive): decoy hash equals the pinned hash — the test cannot distinguish them"
    return 1
  fi
  log "pinned=${pinned_hash:0:16}... decoy=${decoy_hash:0:16}..."

  restart_spire_agent

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found after agent restart"

  collect_agent_logs "$LABEL" "$OUT" "$SUBTEST_LOG_SINCE" "$pod"
  log_file="$OUT/$LABEL/agent-attestor.log"

  # The claim: the hash follows the descriptor the JVM holds, not the pathname.
  assert_log_contains_for_pod "jar_sha256=${pinned_hash}" "$log_file" "$pod" || return 1
  assert_log_not_contains_for_pod "jar_sha256=${decoy_hash}" "$log_file" "$pod" || return 1
  record_evidence_signal "hash-follows-fd-not-symlink:${pinned_hash:0:16}"

  # And the digest the entry is actually pinned on is untouched, so the workload
  # still matches: neither the decoy nor the unlinked name perturbed the jar set.
  assert_log_contains_for_pod "jar_set_sha256=${expected_set}" "$log_file" "$pod" || return 1
  record_evidence_signal "jar-set-digest-unchanged"

  # And the mechanism behind it, so a PASS cannot come from a coincidence (e.g. the
  # decoy never being reachable in the first place).
  assert_log_contains_for_pod 'jar_source=fd' "$log_file" "$pod" || return 1
  assert_log_contains_for_pod 'hash_via_kernel_handle=true' "$log_file" "$pod" || return 1
  assert_log_contains_for_pod 'maps_verified=true' "$log_file" "$pod" || return 1
  record_evidence_signal "discovery:fd+kernel-handle"

  assert_mtls_ok_after_agent_restart || return 1
  return 0
}

if ! run_test_wrapper "bypass-symlink" "PASS" "$OUT_TEST" test_body; then
  restore_clean_deployments
  exit 1
fi

restore_clean_deployments
log "Bypass symlink test finished — hash followed the open descriptor, decoy ignored"
