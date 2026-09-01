#!/usr/bin/env bash
# Level 1: Anti-debug — ptrace / TracerPid detection.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="level1-antidebug"
OUT_TEST="$OUT/$LABEL"
PTRACE_MANIFEST="$OUT_TEST/ptrace-pod.yaml"
mkdir -p "$OUT_TEST"

cleanup_ptrace() {
  # Wait for the tracer to actually be gone (not --wait=false): its watch loop
  # re-attaches to any payments JVM it sees, so it MUST be terminated before we
  # restore the clean deployment, otherwise it would immediately re-trace (and
  # deny) the restored payments pod.
  kubectl delete pod -n "$K8S_NAMESPACE" -l "attack-test=ptrace" --ignore-not-found=true --wait=true --timeout=60s 2>/dev/null || true
}

run_antidebug_test() {
  local pod node log_file ptrace_name
  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found"

  node=$(workload_node "$pod")
  ptrace_name="ptrace-attack-$$"
  log "Launching privileged re-attaching strace pod on node $node"

  # Deny-first design: the strace container runs with hostPID:true, so /proc is the
  # node's process table. Instead of a one-shot attach to the currently-running
  # payments JVM, it runs a WATCH LOOP that continuously discovers the payments
  # JVM's host PID and (re)attaches strace whenever the target PID changes. This
  # lets it survive a pod replacement and re-trace the fresh JVM.
  #
  # Discovery matches on comm == "java" (via /proc/<pid>/comm), NOT on the cmdline
  # alone. This is deliberate: the strace pod's own `sh -c "...payments-service.jar
  # ..."` wrapper has the jar name in ITS cmdline, so a cmdline-only scan (or
  # `pgrep -f`) could make strace attach to itself. Filtering by comm == java plus a
  # self-PID skip guarantees we only ever target a real JVM; payments vs orders is
  # then disambiguated by the jar name in cmdline.
  cat >"$PTRACE_MANIFEST" <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $ptrace_name
  namespace: $K8S_NAMESPACE
  labels:
    attack-test: ptrace
spec:
  hostPID: true
  nodeName: $node
  restartPolicy: Never
  containers:
    - name: ptrace
      image: nicolaka/netshoot:latest
      securityContext:
        privileged: true
      command:
        - sh
        - -c
        - |
          me=\$\$
          traced=
          echo "watch-loop-started"
          while true; do
            target=
            for p in /proc/[0-9]*; do
              pid=\${p#/proc/}
              [ "\$pid" = "\$me" ] && continue
              c=\$(cat "\$p/comm" 2>/dev/null) || continue
              [ "\$c" = java ] || continue
              grep -qa payments-service.jar "\$p/cmdline" 2>/dev/null && { target=\$pid; break; }
            done
            if [ -n "\$target" ] && [ "\$target" != "\$traced" ]; then
              strace -f -p "\$target" -o /dev/null 2>>/tmp/strace.err &
              traced=\$target
              sleep 1
              echo "attached strace-target-host-pid=\$target tracerpid=\$(awk '/TracerPid/{print \$2}' /proc/\$target/status 2>/dev/null)"
            fi
            sleep 1
          done
EOF

  kubectl apply -f "$PTRACE_MANIFEST"
  # Wait for the strace pod to be Running (image pull can be slow the first time)
  # so the watch loop is already polling /proc before the fresh payments JVM boots.
  kubectl wait --for=condition=Ready "pod/$ptrace_name" -n "$K8S_NAMESPACE" --timeout=120s 2>/dev/null || true
  sleep 4

  # Deny-first trigger: delete the healthy payments pod so the ReplicaSet spins up a
  # replacement that boots WHILE the tracer is watching. The watch loop attaches to
  # the new JVM during its startup (before java-spiffe fetches its first SVID), so
  # the very first attestation reports debug_clean=false and the workload is denied
  # an SVID outright — no dependency on TTL expiry, and no pre-existing mTLS
  # connection to tear down. (Single-node kind: the replacement lands on the same
  # node the tracer is pinned to.)
  log "Deleting healthy payments pod $pod to force a traced replacement"
  kubectl delete pod "$pod" -n "$K8S_NAMESPACE" --wait=true --timeout=60s 2>/dev/null || true

  # Wait for the replacement JVM to appear so it has been traced + attested.
  local newpod="" w
  for ((w = 1; w <= 30; w++)); do
    newpod=$(workload_pod "$PAYMENTS_DEPLOY")
    [[ -n "$newpod" && "$newpod" != "$pod" ]] && break
    sleep 2
  done
  log "Replacement payments pod: ${newpod:-<none>}"
  sleep "$SETTLE_SEC"

  # Scrape strace-pod diagnostics so a future failure tells us exactly what happened
  # (did the loop start? which host PID(s) did it re-attach to?).
  kubectl logs "$ptrace_name" -n "$K8S_NAMESPACE" >"$OUT_TEST/ptrace-pod.log" 2>&1 || true
  log "strace-pod: $(grep -E 'watch-loop|attached|tracerpid|no-java' "$OUT_TEST/ptrace-pod.log" 2>/dev/null | tr '\n' ' ')"

  log_file="$OUT/$LABEL/agent-attestor.log"

  # The fresh payments pod attests asynchronously via the Workload API (no agent
  # restart needed — the new PID triggers a new attestation). Poll the agent logs
  # until the tampered attestation shows up instead of racing a single collect.
  local i found=1
  for ((i = 1; i <= 12; i++)); do
    collect_agent_logs "$LABEL" "$OUT"
    if grep -Eq 'debug_clean=false|tracer_pid=' "$log_file"; then
      found=0
      break
    fi
    log "waiting for tampered attestation in agent logs (attempt $i/12)"
    sleep 5
  done

  if [[ $found -ne 0 ]]; then
    log "ASSERT FAIL: log missing pattern: debug_clean=false|tracer_pid= (see $OUT_TEST/ptrace-pod.log)"
    return 1
  fi
  log "Anti-debug landed on fresh payments pod: agent recorded debug_clean=false"
  assert_svid_denied
}

test_body() {
  run_antidebug_test
}

trap cleanup_ptrace EXIT

if run_test_wrapper "level1-antidebug" "PASS" "$OUT_TEST" test_body; then
  :
else
  cleanup_ptrace
  restore_clean_deployments
  exit 1
fi

cleanup_ptrace
restore_clean_deployments
log "Level 1 anti-debug test finished"
