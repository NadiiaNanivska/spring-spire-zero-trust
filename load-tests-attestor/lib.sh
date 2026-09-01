#!/usr/bin/env bash
# Shared helpers for attestor overhead load tests.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$LIB_DIR/.." && pwd)"

export K8S_NAMESPACE="${K8S_NAMESPACE:-spire}"
export TRUST_DOMAIN="${TRUST_DOMAIN:-zerotrust.lab}"
export PROMETHEUS_URL="${PROMETHEUS_URL:-http://127.0.0.1:9090}"
export ORDERS_URL="${ORDERS_URL:-http://127.0.0.1:8080}"
export ORDERS_SVC_NAME="${ORDERS_SVC_NAME:-orders-service}"
export ORDERS_SVC_PORT="${ORDERS_SVC_PORT:-8080}"
export WARMUP_SEC="${WARMUP_SEC:-60}"
export METRICS_STEP="${METRICS_STEP:-5s}"

# Load-generation mode:
#   incluster (default) -> run k6 as a Pod hitting the Service via kube-proxy
#                          (survives rollouts/scale, no host port-forward).
#   local               -> run k6 on the host against ORDERS_URL (needs port-forward).
export K6_MODE="${K6_MODE:-incluster}"
export ORDERS_INCLUSTER_URL="${ORDERS_INCLUSTER_URL:-http://orders-service.spire.svc.cluster.local:8080}"
export K6_IMAGE="${K6_IMAGE:-grafana/k6:0.49.0}"
export JVM_ENTRY_SETTLE_SEC="${JVM_ENTRY_SETTLE_SEC:-30}"

PROM_PF_PID=""
ORDERS_PF_PID=""
ORDERS_PF_SUPERVISOR_PID=""

log() { printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
die() { log "ERROR: $*"; exit 1; }

cleanup_background() {
  if [[ -n "${PROM_PF_PID}" ]] && kill -0 "$PROM_PF_PID" 2>/dev/null; then
    kill "$PROM_PF_PID" 2>/dev/null || true
    wait "$PROM_PF_PID" 2>/dev/null || true
  fi
  stop_orders_port_forward
}

preflight() {
  local cmd
  for cmd in kubectl curl jq; do
    command -v "$cmd" >/dev/null 2>&1 || die "missing required command: $cmd"
  done
  # k6 binary is only needed for local load mode; in-cluster mode runs k6 as a Pod.
  if [[ "$K6_MODE" == "local" ]] && ! command -v k6 >/dev/null 2>&1; then
    die "missing required command: k6 (needed for K6_MODE=local)"
  fi
  kubectl cluster-info >/dev/null 2>&1 || die "kubectl cannot reach cluster"
  kubectl get deployment prometheus -n "$K8S_NAMESPACE" >/dev/null 2>&1 || \
    die "prometheus deployment not found in namespace $K8S_NAMESPACE (run: wsldev observability prometheus-deploy)"
}

resolve_wsldev() {
  if [[ -n "${WSLDEV_BIN:-}" ]] && [[ -x "$WSLDEV_BIN" ]]; then
    echo "$WSLDEV_BIN"
    return
  fi
  if command -v wsldev >/dev/null 2>&1; then
    command -v wsldev
    return
  fi
  local built="$REPO_ROOT/wsldev/wsldev"
  if [[ -x "$built" ]]; then
    echo "$built"
    return
  fi
  die "wsldev not found; build with: (cd wsldev && go build -o wsldev ./cmd/wsldev) or set WSLDEV_BIN"
}

wait_http_ready() {
  local url=$1
  local max_attempts=${2:-60}
  local i
  for ((i = 1; i <= max_attempts; i++)); do
    if curl -sf "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

port_forward_prometheus() {
  if curl -sf "${PROMETHEUS_URL}/-/ready" >/dev/null 2>&1; then
    log "Prometheus already reachable at $PROMETHEUS_URL"
    return 0
  fi
  log "Starting Prometheus port-forward..."
  kubectl port-forward -n "$K8S_NAMESPACE" "deployment/prometheus" 9090:9090 >/dev/null 2>&1 &
  PROM_PF_PID=$!
  wait_http_ready "${PROMETHEUS_URL}/-/ready" 90 || die "Prometheus not ready at $PROMETHEUS_URL"
  log "Prometheus ready at $PROMETHEUS_URL"
}

orders_local_port() {
  local url=${1:-$ORDERS_URL}
  if [[ "$url" =~ :([0-9]+)(/|$) ]]; then
    echo "${BASH_REMATCH[1]}"
  else
    echo "8080"
  fi
}

orders_api_reachable() {
  local code
  code=$(curl -s -m 3 -o /dev/null -w "%{http_code}" -X POST "${ORDERS_URL}/api/orders/create" \
    -H 'Content-Type: application/json' \
    -d '{"itemId":"port-forward-probe","quantity":1}' 2>/dev/null || echo "000")
  [[ "$code" =~ ^(200|201|500|502|503|504)$ ]]
}

ensure_orders_service() {
  local manifest="$REPO_ROOT/orders-service/order-service-svc.yaml"
  if kubectl get "svc/$ORDERS_SVC_NAME" -n "$K8S_NAMESPACE" >/dev/null 2>&1; then
    return 0
  fi
  [[ -f "$manifest" ]] || die "orders Service missing and no manifest at $manifest"
  log "Applying Service $ORDERS_SVC_NAME (stable port-forward target during rollouts)..."
  kubectl apply -f "$manifest"
  sleep 2
}

stop_orders_port_forward() {
  if [[ -n "${ORDERS_PF_SUPERVISOR_PID:-}" ]] && kill -0 "$ORDERS_PF_SUPERVISOR_PID" 2>/dev/null; then
    kill "$ORDERS_PF_SUPERVISOR_PID" 2>/dev/null || true
    wait "$ORDERS_PF_SUPERVISOR_PID" 2>/dev/null || true
    ORDERS_PF_SUPERVISOR_PID=""
  fi
  if [[ -n "${ORDERS_PF_PID:-}" ]] && kill -0 "$ORDERS_PF_PID" 2>/dev/null; then
    kill "$ORDERS_PF_PID" 2>/dev/null || true
    wait "$ORDERS_PF_PID" 2>/dev/null || true
  fi
  ORDERS_PF_PID=""
  pkill -f "kubectl port-forward.*svc/${ORDERS_SVC_NAME}.*${ORDERS_SVC_PORT}" 2>/dev/null || true
}

# kubectl port-forward to a Deployment dies when its pod is replaced (scenario C).
# Forward via Service so traffic follows Ready endpoints across rollouts.
_orders_port_forward_supervisor() {
  local local_port=$1
  while true; do
    kubectl port-forward -n "$K8S_NAMESPACE" "svc/$ORDERS_SVC_NAME" "${local_port}:${ORDERS_SVC_PORT}" >/dev/null 2>&1 &
    ORDERS_PF_PID=$!
    wait "$ORDERS_PF_PID" 2>/dev/null || true
    ORDERS_PF_PID=""
    sleep 2
  done
}

port_forward_orders() {
  local local_port
  local_port="$(orders_local_port)"

  if orders_api_reachable; then
    log "Orders API already reachable at $ORDERS_URL"
    return 0
  fi

  ensure_orders_service
  stop_orders_port_forward

  log "Starting orders port-forward svc/$ORDERS_SVC_NAME ${local_port}:${ORDERS_SVC_PORT} (supervised)..."
  _orders_port_forward_supervisor "$local_port" &
  ORDERS_PF_SUPERVISOR_PID=$!

  local i
  for ((i = 1; i <= 60; i++)); do
    if orders_api_reachable; then
      log "Orders API reachable at $ORDERS_URL"
      return 0
    fi
    sleep 1
  done
  log "WARN: orders API not responding at $ORDERS_URL yet (rollout may still be in progress)"
}

# Prepare whatever the load generator needs to reach orders.
# In-cluster mode needs no host port-forward (k6 hits the Service directly).
ensure_load_target() {
  ensure_orders_service
  if [[ "$K6_MODE" == "local" ]]; then
    port_forward_orders
  else
    log "K6_MODE=incluster: k6 will hit $ORDERS_INCLUSTER_URL (no host port-forward needed)"
  fi
}

# ensure_workload_apps deploys orders/payments when missing (first run / fresh cluster).
# A full deploy also syncs jar hashes into jvm-hashes-configmap.yaml and registers
# SPIRE entries via wsldev app deploy.
ensure_workload_apps() {
  local wsldev_bin missing=0 deploy
  wsldev_bin="$(resolve_wsldev)"

  for deploy in orders-service payments-service; do
    kubectl get deployment "$deploy" -n "$K8S_NAMESPACE" >/dev/null 2>&1 || missing=1
  done
  if [[ $missing -eq 0 ]]; then
    log "Workload apps already deployed (orders-service, payments-service)"
    return 0
  fi

  log "Workload apps not found; running wsldev app deploy payments orders"
  (cd "$REPO_ROOT" && "$wsldev_bin" app deploy payments orders) || \
    die "wsldev app deploy payments orders failed"
  wait_deployment_ready orders-service
  wait_deployment_ready payments-service
}

get_agent_parent_id() {
  kubectl exec -n "$K8S_NAMESPACE" spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server agent list -output json 2>/dev/null | \
    jq -r '.agents[0] | (.spiffe_id // ("spiffe://" + .id.trust_domain + .id.path)) // empty'
}

spire_entry_delete_for_spiffe() {
  local spiffe_id=$1
  local entries id
  entries=$(kubectl exec -n "$K8S_NAMESPACE" spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server entry show -spiffeID "$spiffe_id" -output json 2>/dev/null | \
    jq -r '.entries[]? | (.id // .entry_id) // empty' || true)
  for id in $entries; do
    [[ -z "$id" ]] && continue
    log "Deleting SPIRE entry $id for $spiffe_id"
    kubectl exec -n "$K8S_NAMESPACE" spire-server-0 -c spire-server -- \
      /opt/spire/bin/spire-server entry delete -entryID "$id" >/dev/null 2>&1 || true
  done
}

create_baseline_spire_entry() {
  local spiffe_id=$1
  local sa=$2
  local parent_id
  parent_id=$(get_agent_parent_id)
  [[ -n "$parent_id" ]] || die "cannot resolve SPIRE agent parent ID"

  spire_entry_delete_for_spiffe "$spiffe_id"
  log "Creating baseline SPIRE entry $spiffe_id (parent=$parent_id, sa=$sa)"
  kubectl exec -n "$K8S_NAMESPACE" spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server entry create \
    -spiffeID "$spiffe_id" \
    -parentID "$parent_id" \
    -selector "k8s:ns:${K8S_NAMESPACE}" \
    -selector "k8s:sa:${sa}" \
    >/dev/null
}

# ensure_baseline_spire_entries registers k8s-only entries for the default overlay
# (no JVM plugin). Needed because `wsldev app deploy` always registers JVM selectors,
# which would otherwise leave entries the default agent cannot satisfy.
ensure_baseline_spire_entries() {
  log "Registering baseline SPIRE entries (k8s selectors only)"
  create_baseline_spire_entry \
    "spiffe://${TRUST_DOMAIN}/ns/${K8S_NAMESPACE}/sa/payments-app" "payments-sa"
  create_baseline_spire_entry \
    "spiffe://${TRUST_DOMAIN}/ns/${K8S_NAMESPACE}/sa/orders-app" "orders-sa"
  log "Baseline SPIRE entries registered"
}

# ensure_spire_entries (re)creates JVM workload registration entries from
# spiffe-spire/base/jvm-hashes-configmap.yaml. The plugin only computes hashes;
# these entries pin jvm:jar_sha256=<hash> and enforce integrity on the server.
ensure_spire_entries() {
  local wsldev_bin
  wsldev_bin="$(resolve_wsldev)"
  log "Registering JVM workloads via wsldev spire register-jvm"
  (cd "$REPO_ROOT" && "$wsldev_bin" spire register-jvm) || \
    die "wsldev spire register-jvm failed (populate hashes first: wsldev app deploy payments orders)"
}

# verify_jvm_spire_entries checks that both JVM workloads have entries with
# jvm:jar_sha256 selectors (legacy k8s-only entries would skip jar enforcement).
verify_jvm_spire_entries() {
  local show
  show=$(kubectl exec -n "$K8S_NAMESPACE" spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server entry show 2>/dev/null) || \
    die "cannot list SPIRE entries for verification"

  for sa in payments-app orders-app; do
    if ! echo "$show" | grep -Fq "spiffe://${TRUST_DOMAIN}/ns/${K8S_NAMESPACE}/sa/${sa}"; then
      die "no SPIRE entry for sa/${sa}; run: wsldev spire register-jvm"
    fi
  done
  if ! echo "$show" | grep -Fq 'jar_sha256'; then
    die "SPIRE entries missing jvm:jar_sha256 selectors; run: wsldev spire register-jvm"
  fi
  log "JVM SPIRE entries verified (jar_sha256 present for workloads)"
}

# ensure_spire_entries_for_overlay picks baseline vs JVM registration entries.
ensure_spire_entries_for_overlay() {
  local overlay=${1:-custom-jvm}
  if [[ "$overlay" == "default" ]]; then
    ensure_baseline_spire_entries
  else
    ensure_spire_entries
    verify_jvm_spire_entries
  fi
}

rollout_restart_workloads() {
  log "Rolling restart orders-service and payments-service"
  kubectl rollout restart deployment/orders-service deployment/payments-service -n "$K8S_NAMESPACE"
  wait_deployment_ready orders-service
  wait_deployment_ready payments-service
}

settle_workloads_after_entries() {
  local sec=${1:-$JVM_ENTRY_SETTLE_SEC}
  log "Waiting ${sec}s for workloads to obtain SVIDs after SPIRE entry update"
  sleep "$sec"
}

deploy_overlay() {
  local overlay=$1
  local wsldev_bin
  wsldev_bin="$(resolve_wsldev)"
  log "Deploying SPIRE overlay: $overlay"
  (cd "$REPO_ROOT" && "$wsldev_bin" spire deploy --attestor "$overlay")
  kubectl rollout status "daemonset/spire-agent" -n "$K8S_NAMESPACE" --timeout=180s

  ensure_spire_entries_for_overlay "$overlay"

  if [[ "$overlay" == "custom-jvm" ]]; then
    rollout_restart_workloads
    settle_workloads_after_entries
  fi
}

wait_deployment_ready() {
  local name=$1
  kubectl rollout status "deployment/$name" -n "$K8S_NAMESPACE" --timeout=300s
}

reset_apps_replicas() {
  local replicas=${1:-1}
  log "Scaling orders-service and payments-service to $replicas replicas"
  kubectl scale deployment orders-service payments-service -n "$K8S_NAMESPACE" --replicas="$replicas"
  if [[ "$replicas" -gt 0 ]]; then
    wait_deployment_ready orders-service
    wait_deployment_ready payments-service
  else
    kubectl wait --for=delete pod -l 'app in (orders-service,payments-service)' -n "$K8S_NAMESPACE" --timeout=180s 2>/dev/null || true
  fi
}

warmup_sleep() {
  local sec=${1:-$WARMUP_SEC}
  log "Warmup ${sec}s..."
  sleep "$sec"
}

record_window() {
  local subdir=$1
  local start=$2
  local end=$3
  mkdir -p "$subdir"
  cat >"$subdir/window.env" <<EOF
START=$start
END=$end
EOF
}

write_run_meta() {
  local subdir=$1
  local overlay=$2
  local scenario=$3
  cat >"$subdir/meta.env" <<EOF
OVERLAY=$overlay
SCENARIO=$scenario
EOF
}

utc_now() {
  date -u +%s
}

run_k6_steady() {
  if [[ "$K6_MODE" == "local" ]]; then
    run_k6_local "$@"
  else
    run_k6_incluster "$@"
  fi
}

run_k6_local() {
  local duration=$1
  local rate=$2
  local outdir=$3
  mkdir -p "$outdir"
  (
    cd "$outdir"
    k6 run \
      -e BASE_URL="$ORDERS_URL" \
      -e RATE="$rate" \
      -e DURATION="$duration" \
      "$LIB_DIR/k6/steady-load.js" \
      | tee "k6-output.log" || true
  )
  if [[ -f "$outdir/results-steady-load.json" ]]; then
    mv "$outdir/results-steady-load.json" "$outdir/k6-summary.json"
  fi
}

# Run k6 as an in-cluster Pod hitting the Service via kube-proxy. This survives
# pod rollouts/scale (scenario B/C) because kube-proxy always routes to Ready
# endpoints -- unlike host `kubectl port-forward`, which binds to a single pod.
run_k6_incluster() {
  local duration=$1
  local rate=$2
  local outdir=$3
  mkdir -p "$outdir"

  local name cm i phase
  name="k6-load-$(date -u +%H%M%S)-$RANDOM"
  cm="${name}-script"

  kubectl create configmap "$cm" -n "$K8S_NAMESPACE" \
    --from-file=steady-load.js="$LIB_DIR/k6/steady-load.js" >/dev/null 2>&1 || {
    log "WARN: failed to create k6 ConfigMap"; return 1;
  }

  cat <<EOF | kubectl apply -f - >/dev/null 2>&1 || { log "WARN: failed to create k6 pod"; kubectl delete configmap "$cm" -n "$K8S_NAMESPACE" --ignore-not-found=true >/dev/null 2>&1; return 1; }
apiVersion: v1
kind: Pod
metadata:
  name: $name
  namespace: $K8S_NAMESPACE
  labels:
    app: k6-load
spec:
  restartPolicy: Never
  containers:
    - name: k6
      image: $K6_IMAGE
      imagePullPolicy: IfNotPresent
      workingDir: /tmp
      command: ["k6", "run", "/scripts/steady-load.js"]
      env:
        - name: BASE_URL
          value: "$ORDERS_INCLUSTER_URL"
        - name: RATE
          value: "$rate"
        - name: DURATION
          value: "$duration"
      volumeMounts:
        - name: script
          mountPath: /scripts
  volumes:
    - name: script
      configMap:
        name: $cm
EOF

  log "In-cluster k6 pod $name (rate=$rate duration=$duration target=$ORDERS_INCLUSTER_URL)"

  for ((i = 0; i < 120; i++)); do
    phase=$(kubectl get pod "$name" -n "$K8S_NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    [[ "$phase" == "Running" || "$phase" == "Succeeded" || "$phase" == "Failed" ]] && break
    sleep 1
  done

  # logs -f blocks until the pod finishes, so this doubles as the wait.
  kubectl logs -f "$name" -n "$K8S_NAMESPACE" 2>/dev/null | tee "$outdir/k6-output.log" || true

  sed -n 's/.*__K6_SUMMARY_BEGIN__\(.*\)__K6_SUMMARY_END__.*/\1/p' \
    "$outdir/k6-output.log" >"$outdir/k6-summary.json" 2>/dev/null || true
  if [[ ! -s "$outdir/k6-summary.json" ]]; then
    rm -f "$outdir/k6-summary.json"
    log "WARN: could not extract k6 summary from pod logs"
  fi

  kubectl delete pod "$name" -n "$K8S_NAMESPACE" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
  kubectl delete configmap "$cm" -n "$K8S_NAMESPACE" --ignore-not-found=true >/dev/null 2>&1 || true
}
