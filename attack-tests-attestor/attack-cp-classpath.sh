#!/usr/bin/env bash
# Bypass B (fail-safe): classpath launch without -jar.
#
# The attacker puts an extra jar on the REAL classpath and starts the app via the
# Spring Boot launcher on the classpath instead of the canonical `java -jar`:
#
#   java -cp /app/payments-service.jar:/tmp/extra-evil.jar \
#        org.springframework.boot.loader.launch.JarLauncher
#
# Unlike `-cp ... -jar app.jar` (where the JVM IGNORES -cp), here the extra jar is
# genuinely on the classpath and CAN be loaded. But the jvm-attestor discovers the
# application jar only via /proc/<pid>/maps (Spring Boot fat-jars are not memory-
# mapped) or the `-jar` cmdline argument. With no `-jar`, discovery finds nothing and
# the jar-hash checker returns ErrNotJVM, so the plugin emits NO jvm:* selectors.
#
# The payments registration entry requires jvm:debug_clean=true, jvm:agent_flags_clean=true,
# jvm:maps_verified=true and jvm:jar_sha256=<hash> (see wsldev jvmEntrySelectors). With
# zero jvm selectors that entry never matches, so the SVID is denied. Net result: a
# classpath-injection launch does NOT silently bypass integrity — it costs the workload
# its identity and ejects it from the mTLS mesh (fail-safe).
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

# jvm selectors always carry a "key=value" payload (debug_clean=, agent_flags_clean=,
# maps_verified=, jar_sha256=); k8s/unix selectors never use "=". This lets us detect
# jvm-selector presence in the agent's "PID attested to have selectors" log line.
JVM_SELECTOR_RE='debug_clean=|agent_flags_clean=|maps_verified=|jar_sha256='

test_body() {
  local pod start_cmd cmd_json raw_log host_pids hp denied code

  log "Deploying payments launched via classpath (no -jar) + extra jar on classpath"

  # Plant the extra jar and start the app WITHOUT -jar. strategy=Recreate (set by
  # write_payments_variant_manifest) makes this deny-first: the clean pod is torn down
  # first so the compromised pod is the one fetching its first SVID.
  #
  # Keep the start command on a SINGLE line (joined with ';'): write_payments_variant_manifest
  # emits one YAML sequence item per physical line, so a multi-line script would be split
  # into separate args and only its first line would run under `sh -c`.
  start_cmd="echo malicious-extra-jar-content > /tmp/extra-evil.jar; exec java -cp /app/payments-service.jar:/tmp/extra-evil.jar ${JAR_LAUNCHER_CLASS}"
  cmd_json=$(jq -n --arg script "$start_cmd" '["sh","-c",$script]')

  write_payments_variant_manifest "$MANIFEST" --command "$cmd_json"
  kubectl apply -f "$MANIFEST"
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

  # Positive control / precondition: the jvm plugin must actually be emitting jvm
  # selectors somewhere in this run. A globally-inactive plugin (wrong overlay, agent
  # drift) denies EVERY fresh attestation and would make this test pass for the wrong
  # reason. If this fails, re-run WITH setup so the custom-jvm overlay is deployed and
  # baseline mTLS passes first.
  if ! grep -qE "${JVM_SELECTOR_RE}|jvm attestation" "$raw_log"; then
    log "ASSERT FAIL (inconclusive): no jvm selectors anywhere in the agent log window — the jvm plugin looks inactive. Re-run with setup (custom-jvm overlay)."
    return 1
  fi
  record_evidence_signal "jvm-plugin-active"

  # Correlate the pod to its host PID(s). A crash-looping pod may attest under several
  # PIDs, so collect all of them from the pod-scoped "PID attested" lines.
  host_pids=$(grep -F "pod-name:${pod}" "$raw_log" 2>/dev/null | grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u)
  [[ -n "$host_pids" ]] || { log "ASSERT FAIL: no attestation line for pod $pod in $raw_log (did it attest?)"; return 1; }

  # Proof 1 (ErrNotJVM): no jvm selector emitted for THIS pod — the jar was not
  # discovered because there is no -jar to key on and the fat-jar is not memory-mapped.
  if grep -F "pod-name:${pod}" "$raw_log" | grep -qE "$JVM_SELECTOR_RE"; then
    log "ASSERT FAIL: jvm selectors present for $pod — the jar WAS discovered despite the -cp launch"
    return 1
  fi
  record_evidence_signal "no-jvm-selectors:${pod}"

  # Proof 2 (denial): the Workload API refused an identity for this workload's PID(s).
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
  # A pod with zero identity usually makes orders->payments return HTTP 000/5xx; recorded
  # for the summary only. We deliberately do NOT call assert_mtls_fails here — it treats
  # 000 as a probe error (not denial) and would spin for its whole retry budget.
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
log "Bypass classpath-launch (no -jar) test finished — attestation failed safe (SVID denied)"
