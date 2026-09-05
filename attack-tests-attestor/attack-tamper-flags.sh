#!/usr/bin/env bash
# Level 2: Anti-tamper — table-driven dangerous JVM cmdline flags.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="level2-tamper-flags"
OUT_TEST="$OUT/$LABEL"
MANIFEST_DIR="$OUT_TEST/manifests"
mkdir -p "$MANIFEST_DIR"

# All 7 dangerous flag prefixes from antitamper.go. javaagent/agentpath use benign
# artefacts built at container start so the JVM boots and the attestor can observe
# the prefix (missing files crash before attestation and do not test detection).
declare -a FLAG_TESTS=(
  "javaagent"
  "-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=5005"
  "agentpath"
  "-Xrunjdwp:transport=dt_socket,server=y,suspend=n,address=5005"
  "-Xdebug"
  "-Djdk.attach.allowAttachSelf=true"
  "-Dcom.sun.management.jmxremote"
)

payments_flag_start_cmd() {
  local flag=$1
  case "$flag" in
    javaagent)
      cat <<'EOF'
mkdir -p /tmp/agent
echo 'public class Noop { public static void premain(String a) {} }' > /tmp/agent/Noop.java
javac /tmp/agent/Noop.java
printf 'Premain-Class: Noop\n' > /tmp/agent/MANIFEST.MF
jar cmf /tmp/agent/MANIFEST.MF /tmp/benign-agent.jar -C /tmp/agent Noop.class
exec java -javaagent:/tmp/benign-agent.jar -jar payments-service.jar
EOF
      ;;
    agentpath)
      cat <<'EOF'
JDWP=$(find "$JAVA_HOME" -name 'libjdwp.so' 2>/dev/null | head -1)
[ -n "$JDWP" ] && cp "$JDWP" /tmp/benign.so
exec java -agentpath:/tmp/benign.so=transport=dt_socket,server=y,suspend=n,address=*:5015 -jar payments-service.jar
EOF
      ;;
    *)
      printf 'exec java %s -jar payments-service.jar\n' "$flag"
      ;;
  esac
}

run_flag_test() {
  local flag=$1
  local safe_name log_file manifest pod start_cmd cmd_json
  safe_name=$(echo "$flag" | tr '/:=' '____' | tr -cd '[:alnum:]_-')
  manifest="$MANIFEST_DIR/payments-${safe_name}.yaml"
  start_cmd=$(payments_flag_start_cmd "$flag")
  cmd_json=$(jq -n --arg script "$start_cmd" '["sh","-c",$script]')
  log "Testing dangerous flag: $flag"

  write_payments_variant_manifest "$manifest" --command "$cmd_json"
  apply_manifest "$manifest" || return 1
  wait_deployment_settled "$PAYMENTS_DEPLOY" || true
  settle_workloads

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found after flag deploy: $flag"

  collect_agent_logs "${LABEL}-${safe_name}" "$OUT" "$SUBTEST_LOG_SINCE" "$pod"
  log_file="$OUT/${LABEL}-${safe_name}/agent-attestor.log"

  assert_denied_by_attestor "$pod" "$log_file" 'agent_flags_clean=false|suspicious_flag='
}

test_body() {
  local flag failed=0

  # Track failures explicitly instead of relying on errexit. run_test_wrapper calls
  # this as `if "$@"`, which disables errexit for the whole call tree, so a failing
  # run_flag_test would neither abort the loop nor change the result: test_body's
  # status would be that of the LAST flag alone. A flag that was never exercised
  # (e.g. a manifest the API server rejected) would then be reported as PASS.
  for flag in "${FLAG_TESTS[@]}"; do
    if ! run_flag_test "$flag"; then
      log "FLAG FAILED: $flag"
      failed=$((failed + 1))
    fi
  done

  if [[ $failed -gt 0 ]]; then
    log "ASSERT FAIL: $failed of ${#FLAG_TESTS[@]} dangerous flags were not detected or not exercised"
    return 1
  fi
  record_evidence_signal "flags-detected:${#FLAG_TESTS[@]}/${#FLAG_TESTS[@]}"
  return 0
}

if ! run_test_wrapper "level2-tamper-flags" "PASS" "$OUT_TEST" test_body; then
  restore_clean_deployments
  exit 1
fi

restore_clean_deployments
log "Level 2 tamper-flags test finished (${#FLAG_TESTS[@]} flags)"
