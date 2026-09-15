#!/usr/bin/env bash
# Put the extra jar first on the classpath so it is open before attestation.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="bypass-cp-classpath"
OUT_TEST="$OUT/$LABEL"
MANIFEST="$OUT_TEST/payments-cp-classpath.yaml"
mkdir -p "$OUT_TEST"

JAR_LAUNCHER_CLASS="${JAR_LAUNCHER_CLASS:-org.springframework.boot.loader.launch.JarLauncher}"

EVIL_JAR="/tmp/extra-evil.jar"

JVM_SELECTOR_RE='debug_clean=|agent_flags_clean=|maps_verified=|jar_sha256='

test_body() {
  local pod start_cmd cmd_json raw_log host_pids hp denied code
  local pinned_hash expected_set

  pinned_hash=$(get_payments_pinned_jar_hash)
  [[ -n "$pinned_hash" ]] || die "cannot read pinned payments jar hash from jvm-hashes ConfigMap"

  expected_set=$(printf '%s:%s\n' "$PAYMENTS_JAR" "$pinned_hash" | sha256sum | awk '{print $1}')
  log "clean jar_set_sha256=${expected_set:0:16}... (pinned jar=${pinned_hash:0:16}...)"

  log "Deploying payments launched via classpath (no -jar) with an extra jar ahead of it"

  # Use a valid archive; the JVM immediately closes invalid jars.
  start_cmd="mkdir -p /tmp/evil-src; echo evil-marker > /tmp/evil-src/evil.txt; (jar cf ${EVIL_JAR} -C /tmp/evil-src . || cp ${PAYMENTS_JAR} ${EVIL_JAR}); exec java -cp ${EVIL_JAR}:${PAYMENTS_JAR} ${JAR_LAUNCHER_CLASS}"
  cmd_json=$(jq -n --arg script "$start_cmd" '["sh","-c",$script]')

  write_payments_variant_manifest "$MANIFEST" --command "$cmd_json"
  apply_manifest "$MANIFEST" || return 1
  wait_deployment_settled "$PAYMENTS_DEPLOY" || true
  settle_workloads

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found after classpath launch"

  collect_agent_logs "$LABEL" "$OUT" "$SUBTEST_LOG_SINCE" "$pod"
  raw_log="$OUT/$LABEL/agent-raw.log"
  [[ -s "$raw_log" ]] || { log "ASSERT FAIL: no agent raw log at $raw_log"; return 1; }

  if ! grep -qE "${JVM_SELECTOR_RE}|jvm attestation" "$raw_log"; then
    log "ASSERT FAIL (inconclusive): no jvm selectors anywhere in the agent log window — the jvm plugin looks inactive. Re-run with setup (custom-jvm overlay)."
    return 1
  fi
  record_evidence_signal "jvm-plugin-active"

  host_pids=$(grep -F "pod-name:${pod}" "$raw_log" 2>/dev/null | grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u)
  [[ -n "$host_pids" ]] || { log "ASSERT FAIL: no attestation line for pod $pod in $raw_log (did it attest?)"; return 1; }

  local log_file="$OUT/$LABEL/agent-attestor.log"

  assert_log_contains_for_pod 'jar_source=fd' "$log_file" "$pod" || return 1
  record_evidence_signal "discovery:fd"

  assert_log_contains_for_pod "jar_sha256=${pinned_hash}" "$log_file" "$pod" || return 1
  record_evidence_signal "approved-jar-selector-still-present"

  if grep -F "pod-name:${pod}" "$raw_log" | grep -qF "jar_set_sha256=${expected_set}"; then
    log "ASSERT FAIL: jar_set_sha256 still equals the clean value — the extra classpath jar was never opened by the JVM, so this run did not exercise the scenario"
    return 1
  fi
  record_evidence_signal "jar-set-digest-changed"

  denied=1
  for hp in $host_pids; do
    if grep -E "No identity issued.*pid=${hp}.*registered=false" "$raw_log" >/dev/null 2>&1; then
      denied=0
      record_evidence_signal "no-identity-issued:pid-${hp}"
      break
    fi
  done
  [[ $denied -eq 0 ]] || { log "ASSERT FAIL: no 'No identity issued / registered=false' for pod $pod PIDs ($host_pids)"; return 1; }

  code=$(orders_create_from_pod)
  record_evidence_signal "orders->payments:http-${code}"
  log "mTLS probe (informational): HTTP $code"

  return 0
}

if ! run_test_wrapper "bypass-cp-classpath" "PASS" "$OUT_TEST" test_body; then
  restore_clean_deployments
  exit 1
fi

restore_clean_deployments
log "Bypass classpath-launch test finished — extra jar changed the set digest, SVID denied"
