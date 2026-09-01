#!/usr/bin/env bash
# Level 2: Anti-tamper — table-driven dangerous JVM environment variables.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

OUT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}
LABEL="level2-tamper-env"
OUT_TEST="$OUT/$LABEL"
MANIFEST_DIR="$OUT_TEST/manifests"
mkdir -p "$MANIFEST_DIR"

declare -a ENV_TESTS=(
  "JAVA_TOOL_OPTIONS"
  "_JAVA_OPTIONS"
  "JDK_JAVA_OPTIONS"
  "IBM_JAVA_OPTIONS"
)

DANGEROUS_ENV_VALUE="-Dcom.sun.management.jmxremote"

run_env_test() {
  local env_name=$1
  local manifest log_file pod
  manifest="$MANIFEST_DIR/payments-env-${env_name}.yaml"
  log "Testing dangerous env: $env_name=$DANGEROUS_ENV_VALUE"

  cat >"$manifest" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments-service
  namespace: spire
  labels:
    app: payments-service
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: payments-service
  template:
    metadata:
      labels:
        app: payments-service
    spec:
      serviceAccountName: payments-sa
      containers:
        - name: payments-service
          image: payments-service:1.0
          imagePullPolicy: IfNotPresent
          env:
            - name: SPIFFE_ENDPOINT_SOCKET
              value: /run/spire/sockets/agent.sock
            - name: SPRING_PROFILES_ACTIVE
              value: "spire"
            - name: SERVER_PORT
              value: "8081"
            - name: $env_name
              value: "$DANGEROUS_ENV_VALUE"
          volumeMounts:
            - name: spire-agent-socket
              mountPath: /run/spire/sockets
              readOnly: true
      volumes:
        - name: spire-agent-socket
          hostPath:
            path: /run/spire/sockets
            type: DirectoryOrCreate
EOF

  kubectl apply -f "$manifest"
  wait_deployment_settled "$PAYMENTS_DEPLOY" || true
  settle_workloads

  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found after env deploy: $env_name"

  collect_agent_logs "${LABEL}-${env_name}" "$OUT" "$SUBTEST_LOG_SINCE" "$pod"
  log_file="$OUT/${LABEL}-${env_name}/agent-attestor.log"

  assert_denied_by_attestor "$pod" "$log_file" 'agent_flags_clean=false|suspicious_env='
}

test_body() {
  local env_name
  for env_name in "${ENV_TESTS[@]}"; do
    run_env_test "$env_name"
  done
}

if ! run_test_wrapper "level2-tamper-env" "PASS" "$OUT_TEST" test_body; then
  restore_clean_deployments
  exit 1
fi

restore_clean_deployments
log "Level 2 tamper-env test finished (${#ENV_TESTS[@]} env vars)"
