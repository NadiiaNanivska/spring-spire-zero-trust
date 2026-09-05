#!/usr/bin/env bash
# Bypass B: an attacker-supplied jar on the real classpath.
#
# The app is started off the classpath through the Spring Boot launcher instead of
# the canonical `java -jar`, with an extra jar ahead of the application jar:
#
#   java -cp /tmp/extra-evil.jar:/app/payments-service.jar \
#        org.springframework.boot.loader.launch.JarLauncher
#
# Note this is NOT the `-cp evil.jar -jar app.jar` scenario: with -jar the launcher
# ignores -cp entirely and the extra jar is never loaded, so that variant never
# demonstrated a bypass at all. Here the extra jar is genuinely on the classpath,
# and putting it FIRST guarantees the JVM opens it before it can even resolve the
# launcher class — so it is provably open by the time the workload attests.
#
# Why the per-jar selector is not enough: jvm-attestor discovers every jar the
# process holds open and emits one jvm:jar_sha256 per jar. SPIRE matches an entry
# when its selectors are a SUBSET of the workload's, so an entry pinned only on the
# application jar's hash would STILL match — the approved selector is present, the
# extra one is simply ignored. The registration entry therefore also pins
# jvm:jar_set_sha256, a digest over the whole discovered set, which any extra jar
# changes. That is what denies the SVID here.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="bypass-cp-classpath"
OUT_TEST="$OUT/$LABEL"
MANIFEST="$OUT_TEST/payments-cp-classpath.yaml"
mkdir -p "$OUT_TEST"

# Spring Boot 3.2+ launcher class (payments-service is Spring Boot 3.3.2). Overridable
# in case the launcher package changes in a future upgrade.
JAR_LAUNCHER_CLASS="${JAR_LAUNCHER_CLASS:-org.springframework.boot.loader.launch.JarLauncher}"

EVIL_JAR="/tmp/extra-evil.jar"

# jvm selectors always carry a "key=value" payload; k8s/unix selectors never use "=".
JVM_SELECTOR_RE='debug_clean=|agent_flags_clean=|maps_verified=|jar_sha256='

test_body() {
  local pod start_cmd cmd_json raw_log host_pids hp denied code
  local pinned_hash expected_set

  pinned_hash=$(get_payments_pinned_jar_hash)
  [[ -n "$pinned_hash" ]] || die "cannot read pinned payments jar hash from jvm-hashes ConfigMap"

  # Reproduce the digest wsldev pinned in the registration entry: SHA-256 over one
  # "<path>:<sha256>\n" line per jar, ordered by path. For a clean workload that is
  # exactly one line. See jvmEntrySelectors / jarSetDigest in wsldev.
  expected_set=$(printf '%s:%s\n' "$PAYMENTS_JAR" "$pinned_hash" | sha256sum | awk '{print $1}')
  log "clean jar_set_sha256=${expected_set:0:16}... (pinned jar=${pinned_hash:0:16}...)"

  log "Deploying payments launched via classpath (no -jar) with an extra jar ahead of it"

  # Build the extra jar as a REAL archive. A text file would be rejected by the JDK
  # zip reader, the JVM would drop the descriptor immediately, and the scenario would
  # silently test nothing. `jar` ships with the temurin JDK image; copying the app jar
  # is a valid-archive fallback (a second path with the same content still changes the
  # set digest).
  #
  start_cmd="mkdir -p /tmp/evil-src; echo evil-marker > /tmp/evil-src/evil.txt; (jar cf ${EVIL_JAR} -C /tmp/evil-src . || cp ${PAYMENTS_JAR} ${EVIL_JAR}); exec java -cp ${EVIL_JAR}:${PAYMENTS_JAR} ${JAR_LAUNCHER_CLASS}"
  cmd_json=$(jq -n --arg script "$start_cmd" '["sh","-c",$script]')

  write_payments_variant_manifest "$MANIFEST" --command "$cmd_json"
  apply_manifest "$MANIFEST" || return 1
  # The JVM may fail to reach Ready because java-spiffe cannot obtain an SVID; tolerate
  # a non-Ready rollout (the denial IS the expected outcome).
  wait_deployment_settled "$PAYMENTS_DEPLOY" || true
  settle_workloads

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found after classpath launch"

  collect_agent_logs "$LABEL" "$OUT" "$SUBTEST_LOG_SINCE" "$pod"
  # Read the FULL raw log: the authoritative "No identity issued" endpoints line lives
  # only here (collect-attack-logs.sh's grep filter drops it), and it carries pid= but
  # no pod-name, so we correlate via the "PID attested" line instead.
  raw_log="$OUT/$LABEL/agent-raw.log"
  [[ -s "$raw_log" ]] || { log "ASSERT FAIL: no agent raw log at $raw_log"; return 1; }

  # Positive control: the jvm plugin must actually be emitting jvm selectors somewhere
  # in this run. A globally-inactive plugin denies EVERY fresh attestation and would
  # make this test pass for the wrong reason.
  if ! grep -qE "${JVM_SELECTOR_RE}|jvm attestation" "$raw_log"; then
    log "ASSERT FAIL (inconclusive): no jvm selectors anywhere in the agent log window — the jvm plugin looks inactive. Re-run with setup (custom-jvm overlay)."
    return 1
  fi
  record_evidence_signal "jvm-plugin-active"

  host_pids=$(grep -F "pod-name:${pod}" "$raw_log" 2>/dev/null | grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u)
  [[ -n "$host_pids" ]] || { log "ASSERT FAIL: no attestation line for pod $pod in $raw_log (did it attest?)"; return 1; }

  local log_file="$OUT/$LABEL/agent-attestor.log"

  # Proof 1: discovery worked. Without this a denial could just mean the jar was never
  # found (which is what the OLD cmdline-only discovery did) rather than that the extra
  # jar was detected.
  assert_log_contains_for_pod 'jar_source=fd' "$log_file" "$pod" || return 1
  record_evidence_signal "discovery:fd"

  # Proof 2: the subset hole is real. The approved per-jar selector IS present, so an
  # entry pinned only on jar_sha256 would have matched and issued the SVID.
  assert_log_contains_for_pod "jar_sha256=${pinned_hash}" "$log_file" "$pod" || return 1
  record_evidence_signal "approved-jar-selector-still-present"

  # Proof 3: the defense. The set digest is not the clean one, because the process
  # holds a jar the deployment never approved.
  if grep -F "pod-name:${pod}" "$raw_log" | grep -qF "jar_set_sha256=${expected_set}"; then
    log "ASSERT FAIL: jar_set_sha256 still equals the clean value — the extra classpath jar was never opened by the JVM, so this run did not exercise the scenario"
    return 1
  fi
  record_evidence_signal "jar-set-digest-changed"

  # Proof 4: the Workload API refused an identity for this workload's PID(s).
  denied=1
  for hp in $host_pids; do
    if grep -E "No identity issued.*pid=${hp}.*registered=false" "$raw_log" >/dev/null 2>&1; then
      denied=0
      record_evidence_signal "no-identity-issued:pid-${hp}"
      break
    fi
  done
  [[ $denied -eq 0 ]] || { log "ASSERT FAIL: no 'No identity issued / registered=false' for pod $pod PIDs ($host_pids)"; return 1; }

  # Best-effort functional corroboration (never fatal, never loops): a single mTLS probe.
  # We deliberately do NOT call assert_mtls_fails here — it treats 000 as a probe error
  # (not denial) and would spin for its whole retry budget.
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
