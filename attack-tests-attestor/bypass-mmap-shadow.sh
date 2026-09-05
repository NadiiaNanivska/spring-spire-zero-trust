#!/usr/bin/env bash
# Bypass D: hide the descriptor table behind a mapped jar.
#
# This is the sharpened version of the classpath attack. The extra jar is loaded
# exactly as in bypass-cp-classpath, but the process ALSO maps the approved jar
# into its own address space before launching:
#
#   RandomAccessFile f = new RandomAccessFile("/app/payments-service.jar","r");
#   keep = f.getChannel().map(READ_ONLY, 0, f.length());   // now visible in maps
#   org.springframework.boot.loader.launch.JarLauncher.main(args);
#
# Why that used to work. Discovery consulted its sources in turn and returned the
# FIRST non-empty one. A Spring Boot fat-jar is read with pread(), so maps is
# normally empty and the fd table answers. One FileChannel.map call inverts that:
# maps now holds exactly one entry — the approved jar, at the approved path, with
# the approved bytes and a real inode — so discovery stopped there and the extra
# jar sitting in the fd table was never scanned. The workload then published the
# complete clean selector set (jar_sha256, jar_set_sha256, maps_verified=true,
# hash_via_kernel_handle=true, inode_consistent=true) while running attacker code,
# and the SPIRE entry matched. That is a full authentication bypass, not a
# degradation: jar_set_sha256 cannot close the extra-code hole if the extra code
# is never discovered.
#
# The fix unions the kernel sources instead of short-circuiting, so the mapped jar
# and the open one are both counted. The signature of the union working is
# jar_source=maps+fd — that selector is what distinguishes this run from a plain
# classpath attack, and it is the specific regression signal to assert.
#
# NOTE: requires a plugin build whose discovery unions maps and fd. A build that
# takes the first non-empty source is genuinely vulnerable here and will fail this
# test by issuing an SVID.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="bypass-mmap-shadow"
OUT_TEST="$OUT/$LABEL"
MANIFEST="$OUT_TEST/payments-mmap-shadow.yaml"
mkdir -p "$OUT_TEST"

JAR_LAUNCHER_CLASS="${JAR_LAUNCHER_CLASS:-org.springframework.boot.loader.launch.JarLauncher}"
EVIL_JAR="/tmp/extra-evil.jar"
SHADOW_DIR="/tmp/shadow"

JVM_SELECTOR_RE='debug_clean=|agent_flags_clean=|maps_verified=|jar_sha256='

# The bootstrap that maps the approved jar and then hands off to the Spring Boot
# launcher.
#
# The mapping is held in a static field on purpose: a MappedByteBuffer is unmapped
# once it becomes unreachable, and a mapping collected before the workload attests
# would silently turn this run back into bypass-cp-classpath.
#
# It is shipped base64-encoded. The start command has to survive being embedded in
# JSON, then in a YAML sequence item, then in `sh -c`, and Java source is full of
# quotes and braces; base64 makes it a single shell-safe token.
shadow_source_b64() {
  cat <<JAVA | base64 | tr -d '\n'
import java.io.FileInputStream;
import java.io.InputStream;
import java.io.RandomAccessFile;
import java.nio.MappedByteBuffer;
import java.nio.channels.FileChannel;

public class MmapShadow {
    static MappedByteBuffer keep;
    // A mapping does not keep the fd it was created from open — once map() returns,
    // the RandomAccessFile above is otherwise unused and eligible for finalization,
    // so a later GC can close its native descriptor without disturbing the mapping.
    // The evil jar has no such mapping to fall back on: its only presence in this
    // process is the descriptor the classloader opened for it on the classpath, and
    // nothing here pins that descriptor from the Java side. Holding an explicit
    // reference to it removes the dependence on classloader-internal timing, so the
    // fd is guaranteed open at whatever moment the workload gets attested.
    static InputStream evilHandle;

    public static void main(String[] args) throws Exception {
        evilHandle = new FileInputStream("$EVIL_JAR");
        RandomAccessFile f = new RandomAccessFile("$PAYMENTS_JAR", "r");
        keep = f.getChannel().map(FileChannel.MapMode.READ_ONLY, 0, f.length());
        $JAR_LAUNCHER_CLASS.main(args);
    }
}
JAVA
}

test_body() {
  local pod start_cmd cmd_json raw_log log_file host_pids hp denied code
  local pinned_hash expected_set mapped_jars

  pinned_hash=$(get_payments_pinned_jar_hash)
  [[ -n "$pinned_hash" ]] || die "cannot read pinned payments jar hash from jvm-hashes ConfigMap"

  # The digest wsldev pinned: SHA-256 over one "<path>:<sha256>\n" line per jar,
  # ordered by path. One line for a clean workload.
  expected_set=$(printf '%s:%s\n' "$PAYMENTS_JAR" "$pinned_hash" | sha256sum | awk '{print $1}')
  log "clean jar_set_sha256=${expected_set:0:16}... (pinned jar=${pinned_hash:0:16}...)"

  log "Deploying payments that mmaps its own jar, with an extra jar ahead of it on the classpath"

  # Single physical line: write_payments_variant_manifest emits one YAML sequence
  # item per line, so a multi-line script would be split into separate args.
  start_cmd="mkdir -p /tmp/evil-src ${SHADOW_DIR}; echo evil-marker > /tmp/evil-src/evil.txt; (jar cf ${EVIL_JAR} -C /tmp/evil-src . || cp ${PAYMENTS_JAR} ${EVIL_JAR}); echo $(shadow_source_b64) | base64 -d > ${SHADOW_DIR}/MmapShadow.java; javac -cp ${PAYMENTS_JAR} -d ${SHADOW_DIR} ${SHADOW_DIR}/MmapShadow.java; exec java -cp ${EVIL_JAR}:${PAYMENTS_JAR}:${SHADOW_DIR} MmapShadow"
  cmd_json=$(jq -n --arg script "$start_cmd" '["sh","-c",$script]')

  write_payments_variant_manifest "$MANIFEST" --command "$cmd_json"
  apply_manifest "$MANIFEST" || return 1
  # Denial is the expected outcome, so the pod may never reach Ready.
  wait_deployment_settled "$PAYMENTS_DEPLOY" || true
  settle_workloads

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found after mmap-shadow launch"

  # Negative control, and the whole reason this test is distinct from
  # bypass-cp-classpath: the mapping must actually exist in the JVM's address
  # space. If javac failed, or the buffer was collected, maps is empty and this
  # run degenerates into the plain classpath attack — which would PASS for the
  # wrong reason and leave the shadowing bypass untested.
  mapped_jars=$(kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$PAYMENTS_DEPLOY" -- \
    sh -c "grep -c '${PAYMENTS_JAR}' /proc/1/maps 2>/dev/null || true" | tr -d '\r')
  if [[ -z "$mapped_jars" || "$mapped_jars" == "0" ]]; then
    log "ASSERT FAIL (inconclusive): ${PAYMENTS_JAR} is not mapped in /proc/1/maps — the mmap shadow never took effect"
    return 1
  fi
  log "approved jar is mapped in the JVM address space (${mapped_jars} maps entries)"
  record_evidence_signal "mmap-shadow-in-place:${mapped_jars}-maps-entries"

  collect_agent_logs "$LABEL" "$OUT" "$SUBTEST_LOG_SINCE" "$pod"
  raw_log="$OUT/$LABEL/agent-raw.log"
  log_file="$OUT/$LABEL/agent-attestor.log"
  [[ -s "$raw_log" ]] || { log "ASSERT FAIL: no agent raw log at $raw_log"; return 1; }

  # Positive control: a globally inactive plugin denies every attestation and would
  # make this test pass for the wrong reason.
  if ! grep -qE "${JVM_SELECTOR_RE}|jvm attestation" "$raw_log"; then
    log "ASSERT FAIL (inconclusive): no jvm selectors in the agent log window — the jvm plugin looks inactive. Re-run with setup (custom-jvm overlay)."
    return 1
  fi
  record_evidence_signal "jvm-plugin-active"

  host_pids=$(grep -F "pod-name:${pod}" "$raw_log" 2>/dev/null | grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u)
  [[ -n "$host_pids" ]] || { log "ASSERT FAIL: no attestation line for pod $pod in $raw_log (did it attest?)"; return 1; }

  # Proof 1: the regression signal. Both kernel sources contributed, so the mapped
  # jar did not suppress the descriptor table. A build with the old short-circuit
  # reports jar_source=maps here.
  # assert_log_contains_for_pod greps with -E (extended regex): an unescaped '+'
  # means "one or more of the preceding char", so it would look for "mapsfd"
  # rather than the literal "maps+fd" selector value and never match.
  assert_log_contains_for_pod 'jar_source=maps\+fd' "$log_file" "$pod" || return 1
  record_evidence_signal "discovery:maps+fd-unioned"

  # Proof 2: the subset hole is real — the approved per-jar selector survives, so an
  # entry pinned only on jar_sha256 would still have matched.
  assert_log_contains_for_pod "jar_sha256=${pinned_hash}" "$log_file" "$pod" || return 1
  record_evidence_signal "approved-jar-selector-still-present"

  # Proof 3: the defense. The extra jar is in the set, so the pinned digest breaks.
  if grep -F "pod-name:${pod}" "$raw_log" | grep -qF "jar_set_sha256=${expected_set}"; then
    log "ASSERT FAIL: jar_set_sha256 still equals the clean value — the extra jar was hidden from discovery (this is the bypass)"
    return 1
  fi
  record_evidence_signal "jar-set-digest-changed"

  # Proof 4: the Workload API actually refused an identity.
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

if ! run_test_wrapper "bypass-mmap-shadow" "PASS" "$OUT_TEST" test_body; then
  restore_clean_deployments
  exit 1
fi

restore_clean_deployments
log "Bypass mmap-shadow test finished — mapped jar did not hide the fd table, SVID denied"
