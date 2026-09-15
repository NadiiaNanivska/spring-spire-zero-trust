#!/usr/bin/env bash
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="level1-antidebug"
OUT_TEST="$OUT/$LABEL"
PTRACE_MANIFEST="$OUT_TEST/ptrace-pod.yaml"
mkdir -p "$OUT_TEST"

cleanup_ptrace() {
  # Wait for the tracer to exit before restoring the clean JVM.
  kubectl delete pod -n "$K8S_NAMESPACE" -l "attack-test=ptrace" --ignore-not-found=true --wait=true --timeout=60s 2>/dev/null || true
}

run_antidebug_test() {
  local pod node log_file ptrace_name
  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found"

  node=$(workload_node "$pod")
  ptrace_name="ptrace-attack-$$"
  log "Launching privileged re-attaching strace pod on node $node"


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

  apply_manifest "$PTRACE_MANIFEST" || return 1
  kubectl wait --for=condition=Ready "pod/$ptrace_name" -n "$K8S_NAMESPACE" --timeout=120s 2>/dev/null || true
  sleep 4

  # Restart under tracing so the first attestation is denied.
  log "Deleting healthy payments pod $pod to force a traced replacement"
  kubectl delete pod "$pod" -n "$K8S_NAMESPACE" --wait=true --timeout=60s 2>/dev/null || true

  local newpod="" w
  for ((w = 1; w <= 30; w++)); do
    newpod=$(workload_pod "$PAYMENTS_DEPLOY")
    [[ -n "$newpod" && "$newpod" != "$pod" ]] && break
    sleep 2
  done
  log "Replacement payments pod: ${newpod:-<none>}"
  sleep "$SETTLE_SEC"

  kubectl logs "$ptrace_name" -n "$K8S_NAMESPACE" >"$OUT_TEST/ptrace-pod.log" 2>&1 || true
  log "strace-pod: $(grep -E 'watch-loop|attached|tracerpid|no-java' "$OUT_TEST/ptrace-pod.log" 2>/dev/null | tr '\n' ' ')"

  log_file="$OUT/$LABEL/agent-attestor.log"

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
