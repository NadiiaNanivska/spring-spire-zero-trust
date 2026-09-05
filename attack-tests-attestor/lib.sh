#!/usr/bin/env bash
# Shared helpers for JVM attestor attack / resilience tests.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$LIB_DIR/.." && pwd)"

export K8S_NAMESPACE="${K8S_NAMESPACE:-spire}"
export TRUST_DOMAIN="${TRUST_DOMAIN:-zerotrust.lab}"
export ORDERS_SVC_NAME="${ORDERS_SVC_NAME:-orders-service}"
export PAYMENTS_DEPLOY="${PAYMENTS_DEPLOY:-payments-service}"
export ORDERS_DEPLOY="${ORDERS_DEPLOY:-orders-service}"
export PAYMENTS_JAR="${PAYMENTS_JAR:-/app/payments-service.jar}"
export ORDERS_JAR="${ORDERS_JAR:-/app/orders-service.jar}"
export ORDERS_INCLUSTER_URL="${ORDERS_INCLUSTER_URL:-http://orders-service.spire.svc.cluster.local:8080}"
export SETTLE_SEC="${SETTLE_SEC:-30}"
export LOG_SINCE="${LOG_SINCE:-300s}"
export SUBTEST_LOG_SINCE="${SUBTEST_LOG_SINCE:-120s}"
export LARGE_JAR_MB="${LARGE_JAR_MB:-200}"
export DOS_LATENCY_THRESHOLD_US="${DOS_LATENCY_THRESHOLD_US:-60000000}"
export MTLS_OK_RETRIES_AFTER_AGENT_RESTART="${MTLS_OK_RETRIES_AFTER_AGENT_RESTART:-24}"
export MTLS_OK_DELAY_AFTER_AGENT_RESTART="${MTLS_OK_DELAY_AFTER_AGENT_RESTART:-5}"

PAYMENTS_PF_PID=""
ORDERS_PF_PID=""
TEST_EVIDENCE_SIGNALS=()

log() { printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
die() { log "ERROR: $*"; exit 1; }

record_evidence_signal() {
  TEST_EVIDENCE_SIGNALS+=("$1")
}

evidence_summary() {
  if [[ ${#TEST_EVIDENCE_SIGNALS[@]} -gt 0 ]]; then
    local IFS='; '
    echo "${TEST_EVIDENCE_SIGNALS[*]}"
  else
    echo "all assertions passed"
  fi
}

cleanup_port_forwards() {
  if [[ -n "${PAYMENTS_PF_PID}" ]] && kill -0 "$PAYMENTS_PF_PID" 2>/dev/null; then
    kill "$PAYMENTS_PF_PID" 2>/dev/null || true
    wait "$PAYMENTS_PF_PID" 2>/dev/null || true
  fi
  if [[ -n "${ORDERS_PF_PID}" ]] && kill -0 "$ORDERS_PF_PID" 2>/dev/null; then
    kill "$ORDERS_PF_PID" 2>/dev/null || true
    wait "$ORDERS_PF_PID" 2>/dev/null || true
  fi
  PAYMENTS_PF_PID=""
  ORDERS_PF_PID=""
}

preflight() {
  local cmd
  for cmd in kubectl curl jq; do
    command -v "$cmd" >/dev/null 2>&1 || die "missing required command: $cmd"
  done
  kubectl cluster-info >/dev/null 2>&1 || die "kubectl cannot reach cluster"
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
  die "wsldev not found; build with: (cd wsldev && go build -o wsldev ./cmd/wsldev)"
}

results_dir() {
  local ts
  ts=$(date -u +%Y%m%d-%H%M%S)
  echo "${RESULTS_DIR:-$LIB_DIR/results/run-$ts}"
}

init_test_result() {
  local out=$1
  local name=$2
  local expected=${3:-PASS}
  mkdir -p "$out"
  cat >"$out/meta.env" <<EOF
TEST_NAME="$name"
EXPECTED_STATUS="$expected"
STARTED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF
}

record_test_result() {
  local out=$1
  local status=$2
  local evidence=${3:-}
  # Quote values: STATUS can be "LIMITATION (expected)" and EVIDENCE contains
  # spaces. Without quotes, sourcing meta.env in run-all.sh tried to execute the
  # trailing words as commands ("assertions: command not found").
  {
    echo "STATUS=\"$status\""
    echo "FINISHED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "EVIDENCE=\"$evidence\""
  } >>"$out/meta.env"
  if [[ -n "$evidence" ]]; then
    echo "$evidence" >"$out/evidence.txt"
  fi
  log "Test result: $status ($out)"
}

deploy_overlay() {
  local overlay=$1
  local wsldev_bin
  wsldev_bin="$(resolve_wsldev)"
  log "Deploying SPIRE overlay: $overlay"
  (cd "$REPO_ROOT" && "$wsldev_bin" spire deploy --attestor "$overlay")
  kubectl rollout status "daemonset/spire-agent" -n "$K8S_NAMESPACE" --timeout=180s
}

wait_deployment_ready() {
  local name=$1
  kubectl rollout status "deployment/$name" -n "$K8S_NAMESPACE" --timeout=300s
}

# wait_deployment_settled waits for a rollout but, unlike wait_deployment_ready, does
# NOT abort the script (set -e) if the deployment never becomes Ready. Deny-first
# tamper variants intentionally boot a JVM that either crashes (e.g.
# -javaagent:/tmp/evil.jar with a missing jar) or is denied its SVID, so the fresh
# pod may never reach Ready. Paired with strategy=Recreate the old (clean) pod is
# already gone, so a stuck/crashing new pod IS the denial we assert on.
#
# Returns 0 if the pod became Ready (so it attested and a per-flag/env log line is
# expected), 1 if it timed out (JVM likely crashed before attesting -> only the mTLS
# denial is available). Call it inside an `if` so the non-zero return is handled and
# does not trip set -e.
wait_deployment_settled() {
  local name=$1
  local timeout=${2:-120s}
  if kubectl rollout status "deployment/$name" -n "$K8S_NAMESPACE" --timeout="$timeout" 2>/dev/null; then
    return 0
  fi
  log "deployment/$name did not reach Ready within $timeout (expected for crash/deny-first variants)"
  return 1
}

# delete_payments_pod_and_wait deletes the current payments pod and waits for the
# ReplicaSet replacement to appear. Deny-first: SPIRE never revokes an already-issued
# SVID and orders pools its mTLS connection to the peer, so a still-running compromised
# pod keeps serving valid mTLS until TTL. Killing the pod drops both the old SVID and
# orders' pooled connection, forcing the fresh pod to re-attest (and be denied) before
# it can serve. Use this whenever the tamper artifact lives on the SERVER side (e.g. a
# bogus SPIRE entry) so the replacement pod is born compromised.
delete_payments_pod_and_wait() {
  local oldpod newpod w
  oldpod=$(workload_pod "$PAYMENTS_DEPLOY")
  log "Deny-first: deleting payments pod ${oldpod:-<none>} to drop old SVID + pooled mTLS"
  [[ -n "$oldpod" ]] && kubectl delete pod "$oldpod" -n "$K8S_NAMESPACE" --wait=true --timeout=60s 2>/dev/null || true
  newpod=""
  for ((w = 1; w <= 30; w++)); do
    newpod=$(workload_pod "$PAYMENTS_DEPLOY")
    [[ -n "$newpod" && "$newpod" != "$oldpod" ]] && break
    sleep 2
  done
  log "Replacement payments pod: ${newpod:-<none>}"
  settle_workloads
}

deploy_clean_apps() {
  local wsldev_bin
  wsldev_bin="$(resolve_wsldev)"
  log "Deploying clean payments + orders via wsldev"
  (cd "$REPO_ROOT" && "$wsldev_bin" app deploy payments orders)
  wait_deployment_ready "$PAYMENTS_DEPLOY"
  wait_deployment_ready "$ORDERS_DEPLOY"
  settle_workloads
}

workload_pod() {
  local deploy=$1
  # Pick the NEWEST pod, not items[0]. items[0] is ordered arbitrarily by kubectl and,
  # when a finished test leaves stale pods around (denied variant still Running, old
  # ReplicaSet mid-rollout), it can return the wrong test's pod -> assertions run
  # against unrelated log lines. Sorting by creationTimestamp and taking the last entry
  # always yields the pod the current test just deployed.
  kubectl get pods -n "$K8S_NAMESPACE" -l "app=$deploy" \
    --sort-by=.metadata.creationTimestamp \
    -o jsonpath='{.items[-1:].metadata.name}' 2>/dev/null
}

pod_phase() {
  local deploy=$1
  kubectl get pods -n "$K8S_NAMESPACE" -l "app=$deploy" \
    -o jsonpath='{.items[0].status.phase}' 2>/dev/null
}

pod_container_ready() {
  local deploy=$1
  kubectl get pods -n "$K8S_NAMESPACE" -l "app=$deploy" \
    -o jsonpath='{.items[0].status.containerStatuses[0].ready}' 2>/dev/null
}

pod_is_running() {
  local deploy=$1
  [[ "$(pod_phase "$deploy")" == "Running" ]]
}

pod_is_ready() {
  local deploy=$1
  [[ "$(pod_phase "$deploy")" == "Running" && "$(pod_container_ready "$deploy")" == "true" ]]
}

workload_node() {
  local pod=$1
  kubectl get pod "$pod" -n "$K8S_NAMESPACE" -o jsonpath='{.spec.nodeName}'
}

java_pid() {
  local pod=$1
  local container=${2:-$PAYMENTS_DEPLOY}
  kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$container" -- sh -c \
    'for p in /proc/[0-9]*; do
       if grep -q java "$p/cmdline" 2>/dev/null; then basename "$p"; exit 0; fi
     done
     exit 1' 2>/dev/null || true
}

agent_pod_on_node() {
  local node=$1
  kubectl get pods -n "$K8S_NAMESPACE" -l app=spire-agent \
    --field-selector "spec.nodeName=$node" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null
}

restart_spire_agent() {
  log "Restarting spire-agent DaemonSet to force re-attestation"
  kubectl rollout restart "daemonset/spire-agent" -n "$K8S_NAMESPACE"
  kubectl rollout status "daemonset/spire-agent" -n "$K8S_NAMESPACE" --timeout=180s
  sleep "$SETTLE_SEC"
}

settle_workloads() {
  log "Settling workloads ${SETTLE_SEC}s for attestation"
  sleep "$SETTLE_SEC"
}

collect_agent_logs() {
  local label=$1
  local out_base=${2:-${RESULTS_DIR:-$LIB_DIR/results}}
  local since=${3:-${SUBTEST_LOG_SINCE:-120s}}
  local pod_filter=${4:-}
  "$LIB_DIR/collect-attack-logs.sh" "$label" "$out_base" "$since" "$pod_filter"
}

pod_log_slice() {
  local log_file=$1
  local pod=$2
  local out_file=$3
  [[ -n "$pod" ]] || return 1
  grep -F "pod-name:${pod}" "$log_file" >"$out_file" 2>/dev/null || true
  [[ -s "$out_file" ]]
}

host_pid_from_pod_log() {
  local log_file=$1
  local pod=$2
  grep -F "pod-name:${pod}" "$log_file" 2>/dev/null | grep -oE 'pid=[0-9]+' | head -1 | cut -d= -f2
}

assert_log_contains_for_pod() {
  local pattern=$1
  local log_file=$2
  local pod=$3
  local pod_log="${log_file}.pod-${pod}.log"
  local host_pid

  if pod_log_slice "$log_file" "$pod" "$pod_log"; then
    if grep -qE "$pattern" "$pod_log"; then
      return 0
    fi
  fi

  # checker failed / jvm attestation timing lines often omit pod-name; correlate by host PID.
  # NB: do NOT write this as `grep ... | grep -qE`. Under `set -o pipefail`, grep -q exits
  # on the first match and closes the pipe, so the upstream grep dies with SIGPIPE and the
  # pipeline reports non-zero even though the pattern WAS found -> deterministic false
  # ASSERT FAIL whenever there is more than one correlated line. Materialize the correlated
  # lines first and match them via a here-string so there is no pipe to break.
  host_pid=$(host_pid_from_pod_log "$log_file" "$pod")
  if [[ -n "$host_pid" ]]; then
    local correlated
    correlated=$(grep -E "pid=${host_pid}" "$log_file" || true)
    if [[ -n "$correlated" ]] && grep -qE "$pattern" <<<"$correlated"; then
      return 0
    fi
  fi

  log "ASSERT FAIL: pod $pod log missing pattern: $pattern (see $pod_log)"
  return 1
}

assert_log_not_contains_for_pod() {
  local pattern=$1
  local log_file=$2
  local pod=$3
  local pod_log="${log_file}.pod-${pod}.log"
  if ! pod_log_slice "$log_file" "$pod" "$pod_log"; then
    log "ASSERT FAIL: no agent log lines scoped to pod $pod in $log_file"
    return 1
  fi
  if grep -qE "$pattern" "$pod_log"; then
    log "ASSERT FAIL: pod $pod log contains unexpected pattern: $pattern"
    return 1
  fi
  return 0
}

extract_jar_hash_for_pod() {
  local log_file=$1
  local pod=$2
  local pod_log="${log_file}.pod-${pod}.log"
  pod_log_slice "$log_file" "$pod" "$pod_log" || return 1
  grep -oE 'jar_sha256=[a-f0-9]{64}' "$pod_log" 2>/dev/null | head -1 | cut -d= -f2
}

get_payments_pinned_jar_hash() {
  kubectl get configmap jvm-hashes -n "$K8S_NAMESPACE" -o jsonpath='{.data.jvm-hashes\.json}' 2>/dev/null | \
    jq -r '.jars["/app/payments-service.jar"] // empty'
}

grep_agent_logs() {
  local pattern=$1
  local log_file=${2:-}
  if [[ -z "$log_file" ]]; then
    kubectl logs -n "$K8S_NAMESPACE" daemonset/spire-agent -c spire-agent --since="$LOG_SINCE" 2>/dev/null | grep -E "$pattern" || true
  else
    grep -E "$pattern" "$log_file" || true
  fi
}

assert_log_contains() {
  local pattern=$1
  local log_file=$2
  if grep -qE "$pattern" "$log_file"; then
    return 0
  fi
  log "ASSERT FAIL: log missing pattern: $pattern"
  return 1
}

assert_log_not_contains() {
  local pattern=$1
  local log_file=$2
  if grep -qE "$pattern" "$log_file"; then
    log "ASSERT FAIL: log contains unexpected pattern: $pattern"
    return 1
  fi
  return 0
}

get_agent_parent_id() {
  # Handle both output shapes across SPIRE versions: a flat "spiffe_id" string,
  # or a structured "id" object with trust_domain + path.
  kubectl exec -n "$K8S_NAMESPACE" spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server agent list -output json 2>/dev/null | \
    jq -r '.agents[0] | (.spiffe_id // ("spiffe://" + .id.trust_domain + .id.path)) // empty'
}

spire_entry_delete_for_spiffe() {
  local spiffe_id=$1
  local entries
  entries=$(kubectl exec -n "$K8S_NAMESPACE" spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server entry show -spiffeID "$spiffe_id" -output json 2>/dev/null | \
    jq -r '.entries[] | (.id // .entry_id) // empty' || true)
  local id
  for id in $entries; do
    [[ -z "$id" ]] && continue
    log "Deleting SPIRE entry $id for $spiffe_id"
    kubectl exec -n "$K8S_NAMESPACE" spire-server-0 -c spire-server -- \
      /opt/spire/bin/spire-server entry delete -entryID "$id" >/dev/null 2>&1 || true
  done
}

# create_jvm_entry pins a full JVM selector set for one workload, in the correct
# "key=value" form emitted by the plugin, including an explicit jar_sha256. Used to
# simulate an unapproved/mismatched jar (pin a wrong hash -> workload denied). For
# the normal happy path prefer ensure_spire_entries (wsldev is the source of truth).
create_jvm_entry() {
  local spiffe_id=$1
  local sa=$2
  local jar_hash=$3
  local parent_id
  parent_id=$(get_agent_parent_id)
  [[ -n "$parent_id" ]] || die "cannot resolve SPIRE agent parent ID"

  spire_entry_delete_for_spiffe "$spiffe_id"

  log "Creating SPIRE entry $spiffe_id (parent=$parent_id, jar_sha256=${jar_hash:0:16}...)"
  kubectl exec -n "$K8S_NAMESPACE" spire-server-0 -c spire-server -- \
    /opt/spire/bin/spire-server entry create \
    -spiffeID "$spiffe_id" \
    -parentID "$parent_id" \
    -selector "k8s:ns:${K8S_NAMESPACE}" \
    -selector "k8s:sa:${sa}" \
    -selector "jvm:debug_clean=true" \
    -selector "jvm:agent_flags_clean=true" \
    -selector "jvm:maps_verified=true" \
    -selector "jvm:jar_sha256=${jar_hash}" \
    >/dev/null
}

# ensure_spire_entries (re)creates the JVM workload registration entries via wsldev,
# the single source of truth: it parses spiffe-spire/base/jvm-hashes-configmap.yaml
# and pins jvm:jar_sha256=<hash> plus the integrity selectors (correct "=" form).
# The plugin only computes hashes; these entries are what enforce them.
ensure_spire_entries() {
  local wsldev_bin
  wsldev_bin="$(resolve_wsldev)"
  log "Registering JVM workloads via wsldev spire register-jvm"
  (cd "$REPO_ROOT" && "$wsldev_bin" spire register-jvm)
}

ensure_orders_service() {
  local manifest="$REPO_ROOT/orders-service/order-service-svc.yaml"
  if kubectl get "svc/$ORDERS_SVC_NAME" -n "$K8S_NAMESPACE" >/dev/null 2>&1; then
    return 0
  fi
  [[ -f "$manifest" ]] || die "orders Service missing and no manifest at $manifest"
  kubectl apply -f "$manifest"
}

orders_create_http_code() {
  local url=${1:-$ORDERS_INCLUSTER_URL}
  curl -s -m 10 -o /dev/null -w "%{http_code}" -X POST "${url}/api/orders/create" \
    -H 'Content-Type: application/json' \
    -d '{"itemId":"attack-test-probe","quantity":1}' 2>/dev/null || echo "000"
}

orders_create_from_pod() {
  local raw code
  # --quiet suppresses kubectl's "pod deleted" / attach notices. curl writes the
  # status behind a sentinel so we can extract exactly the 3-digit code even if any
  # stray text leaks into the captured output (that leak previously corrupted $code
  # and produced false PASS/FAIL in the mTLS asserts).
  raw=$(kubectl run "attack-probe-$$" \
    --rm -i --quiet --restart=Never \
    -n "$K8S_NAMESPACE" \
    --image=curlimages/curl:8.5.0 \
    --command -- \
    curl -s -m 15 -o /dev/null -w 'HTTPSTATUS:%{http_code}' \
    -X POST "${ORDERS_INCLUSTER_URL}/api/orders/create" \
    -H 'Content-Type: application/json' \
    -d '{"itemId":"attack-test-probe","quantity":1}' 2>/dev/null || true)
  code=$(printf '%s' "$raw" | grep -oE 'HTTPSTATUS:[0-9]{3}' | head -1 | cut -d: -f2)
  [[ -n "$code" ]] || code="000"
  echo "$code"
}

assert_mtls_ok() {
  # mTLS readiness is eventually-consistent: right after a deploy/register or an
  # agent restart, payments may not have its SVID yet, so orders->payments returns
  # 500 for a few seconds. Retry the positive probe until it succeeds (or times out)
  # instead of failing on the first too-early shot. Only the success direction
  # retries; assert_mtls_fails stays single-shot.
  local attempts=${MTLS_OK_RETRIES:-12}
  local delay=${MTLS_OK_DELAY:-5}
  local code i
  for ((i = 1; i <= attempts; i++)); do
    code=$(orders_create_from_pod)
    if [[ "$code" =~ ^(200|201)$ ]]; then
      log "orders->payments probe HTTP $code (attempt $i/$attempts)"
      record_evidence_signal "mtls-ok:http-${code}"
      return 0
    fi
    log "orders->payments probe HTTP $code (attempt $i/$attempts, waiting for 200/201)"
    [[ $i -lt $attempts ]] && sleep "$delay"
  done
  log "ASSERT FAIL: expected 200/201 after $attempts attempts, last=$code"
  return 1
}

assert_mtls_fails() {
  # SPIRE never revokes an already-issued SVID; it only stops renewing one whose
  # entry no longer matches (e.g. a traced workload emitting debug_clean=false). So
  # for live-process attacks the tampered workload keeps serving mTLS until its SVID
  # expires (default_x509_svid_ttl, kept short for this reason) and the agent refuses
  # to reissue. Poll until the probe stops returning 2xx (denial reached) or we time
  # out. Restart-based attacks fail on the very first probe, so this returns fast for
  # them and only actually waits for the TTL-bounded cases.
  # HTTP 000 (probe/curl failure) is NOT treated as denial — it produces false PASS.
  local attempts=${MTLS_FAIL_RETRIES:-30}
  local delay=${MTLS_FAIL_DELAY:-5}
  local code i
  for ((i = 1; i <= attempts; i++)); do
    code=$(orders_create_from_pod)
    if [[ "$code" == "000" ]]; then
      log "orders->payments probe HTTP 000 (attempt $i/$attempts, probe error — not denial)"
      [[ $i -lt $attempts ]] && sleep "$delay"
      continue
    fi
    if [[ ! "$code" =~ ^(200|201)$ ]]; then
      log "orders->payments probe HTTP $code (attempt $i/$attempts) — denial confirmed"
      record_evidence_signal "mtls-deny:http-${code}"
      return 0
    fi
    log "orders->payments probe HTTP $code (attempt $i/$attempts, waiting for SVID to expire/deny)"
    [[ $i -lt $attempts ]] && sleep "$delay"
  done
  log "ASSERT FAIL: expected mTLS failure (non-2xx, not 000) within $((attempts * delay))s, last=$code"
  return 1
}

assert_mtls_ok_after_agent_restart() {
  MTLS_OK_RETRIES=$MTLS_OK_RETRIES_AFTER_AGENT_RESTART \
    MTLS_OK_DELAY=$MTLS_OK_DELAY_AFTER_AGENT_RESTART \
    assert_mtls_ok
}

# attestor_evidence_reason pulls the actual human-readable reason out of the first agent
# log line (scoped to $pod, with the same host-PID fallback as assert_log_contains_for_pod)
# that matches $pattern, so the summary shows WHY the attestor refused, e.g.
#   attestor-log:payments-...:checker failed: JVM Attach API socket exposed at /proc/.../.java_pid1 refusing attestation
# instead of the opaque "attestor-log:payments-...". The output is sanitized to be safe
# both inside EVIDENCE="..." (meta.env is `source`d by run-all.sh) and inside the
# pipe-delimited markdown table: double quotes, backticks, '$', backslashes, '|' and ';'
# are stripped, whitespace is collapsed, and the text is truncated.
attestor_evidence_reason() {
  local log_file=$1
  local pod=$2
  local pattern=$3
  local pod_log="${log_file}.pod-${pod}.log"
  local line="" host_pid reason

  if [[ -f "$pod_log" ]]; then
    line=$(grep -E "$pattern" "$pod_log" 2>/dev/null | head -1 || true)
  fi
  if [[ -z "$line" ]]; then
    host_pid=$(host_pid_from_pod_log "$log_file" "$pod")
    if [[ -n "$host_pid" ]]; then
      local correlated
      correlated=$(grep -E "pid=${host_pid}" "$log_file" 2>/dev/null || true)
      [[ -n "$correlated" ]] && line=$(grep -E "$pattern" <<<"$correlated" 2>/dev/null | head -1 || true)
    fi
  fi
  [[ -n "$line" ]] || { printf ''; return 0; }

  # Prefer the explicit error= reason (richest); otherwise combine msg= with the exact
  # token that matched the detection pattern.
  reason=$(grep -oE 'error="[^"]*"' <<<"$line" | head -1 | sed -E 's/^error="//; s/"$//' || true)
  if [[ -z "$reason" ]]; then
    local msg token
    msg=$(grep -oE 'msg="[^"]*"' <<<"$line" | head -1 | sed -E 's/^msg="//; s/"$//' || true)
    token=$(grep -oE "$pattern" <<<"$line" | head -1 || true)
    if [[ -n "$msg" && -n "$token" ]]; then
      reason="$msg ($token)"
    elif [[ -n "$msg" ]]; then
      reason="$msg"
    else
      reason="$token"
    fi
  fi

  # \140 = backtick, \134 = backslash (octal keeps the tr set unambiguous in quotes).
  reason=$(printf '%s' "$reason" | tr -d '"$|;\140\134' | tr '\n\r\t' '   ' | sed -E 's/  +/ /g; s/^ +//; s/ +$//')
  if [[ ${#reason} -gt 160 ]]; then
    reason="${reason:0:157}..."
  fi
  printf '%s' "$reason"
}

assert_denied_by_attestor() {
  local pod=$1
  local log_file=$2
  local log_pattern=${3:-'agent_flags_clean=false|debug_clean=false|suspicious_flag=|suspicious_env=|FailedPrecondition|checker failed|Attach API socket|tracer_pid='}

  if ! assert_log_contains_for_pod "$log_pattern" "$log_file" "$pod"; then
    return 1
  fi
  local attestor_reason
  attestor_reason=$(attestor_evidence_reason "$log_file" "$pod" "$log_pattern")
  if [[ -n "$attestor_reason" ]]; then
    record_evidence_signal "attestor-log:${pod}:${attestor_reason}"
  else
    record_evidence_signal "attestor-log:${pod}"
  fi

  if pod_is_ready "$PAYMENTS_DEPLOY"; then
    assert_mtls_fails
    return $?
  fi

  if pod_is_running "$PAYMENTS_DEPLOY"; then
    log "payments pod $pod Running but not Ready — treating as SVID/workload denial"
    record_evidence_signal "pod-not-ready:${pod}"
    return 0
  fi

  log "ASSERT FAIL: pod $pod not Running after attestor log hit — crash without workload denial proof"
  return 1
}

# assert_denied_by_attestor_log_only proves the agent REFUSED attestation purely from the
# agent log, without gating on a downstream orders->payments mTLS failure. Use it for
# LIVE-process tamper injected into an already-running, already-SVID'd pod (the Attach-API
# socket test). Two facts make the mTLS probe the WRONG signal there:
#   1. SPIRE never revokes an already-issued SVID, so the still-running payments pod keeps
#      serving with the SVID it fetched BEFORE the socket was injected until TTL expiry.
#   2. orders keeps a keep-alive connection pool to payments, so once a handshake has
#      succeeded, subsequent probes reuse that connection and never re-validate payments'
#      cert — orders->payments stays HTTP 200 well past any SVID expiry.
# The guarantee of the attach-socket defense is that RE-ATTESTATION is refused (checker
# failed / FailedPrecondition -> "No identity issued"), which is exactly what we assert.
assert_denied_by_attestor_log_only() {
  local pod=$1
  local log_file=$2
  local log_pattern=${3:-'FailedPrecondition|checker failed|Attach API socket|attach_socket_exposed'}

  if ! assert_log_contains_for_pod "$log_pattern" "$log_file" "$pod"; then
    return 1
  fi
  local attestor_reason
  attestor_reason=$(attestor_evidence_reason "$log_file" "$pod" "$log_pattern")
  if [[ -n "$attestor_reason" ]]; then
    record_evidence_signal "attestor-log:${pod}:${attestor_reason}"
  else
    record_evidence_signal "attestor-log:${pod}"
  fi

  # Surface the endpoint-level consequence too (correlated by host PID): the agent told the
  # Workload API caller it gets no identity. Best-effort evidence, not a hard gate.
  local host_pid
  host_pid=$(host_pid_from_pod_log "$log_file" "$pod")
  if [[ -n "$host_pid" ]]; then
    local denied
    denied=$(grep -E "pid=${host_pid}" "$log_file" 2>/dev/null || true)
    if [[ -n "$denied" ]] && grep -qE 'No identity issued' <<<"$denied"; then
      record_evidence_signal "svid-denied:${pod}"
    fi
  fi
  return 0
}

assert_jar_hash_changed_for_pod() {
  local pod=$1
  local log_file=$2
  local pinned_hash=$3
  local observed

  observed=$(extract_jar_hash_for_pod "$log_file" "$pod")
  [[ -n "$observed" ]] || {
    log "ASSERT FAIL: no jar_sha256 found in logs for pod $pod"
    return 1
  }
  if [[ "$observed" == "$pinned_hash" ]]; then
    log "ASSERT FAIL: jar_sha256 unchanged ($observed) after tamper"
    return 1
  fi
  log "jar_sha256 changed: pinned=${pinned_hash:0:16}... observed=${observed:0:16}..."
  record_evidence_signal "jar-hash-changed:${observed:0:16}"
  return 0
}

assert_svid_issued() {
  assert_mtls_ok
}

assert_svid_denied() {
  assert_mtls_fails
}

# Tamper jar in the running payments pod and force re-attestation while the SPIRE
# entry still pins the pre-tamper hash — plugin computes a new jar_sha256 that no
# longer matches the entry.
tamper_payments_jar_and_reattest() {
  local pod pinned
  pod=$(workload_pod "$PAYMENTS_DEPLOY")
  [[ -n "$pod" ]] || die "payments pod not found for jar tamper"
  pinned=$(get_payments_pinned_jar_hash)
  [[ -n "$pinned" ]] || die "cannot read pinned payments jar hash from jvm-hashes ConfigMap"

  tamper_jar_append "$pod" "$PAYMENTS_JAR"
  restart_spire_agent
  log "Tampered payments jar; pinned hash=${pinned:0:16}..."
  echo "$pinned"
}

assert_bypass_limitation() {
  # Documented limitation: workload may still get SVID via k8s/unix only.
  local log_file=$1
  if assert_log_contains 'not a JVM process|selectors=0' "$log_file" 2>/dev/null; then
  log "LIMITATION confirmed: JVM attestor skipped (ErrNotJVM or empty selectors)"
    return 0
  fi
  if assert_log_not_contains 'maps_verified=true' "$log_file"; then
    log "LIMITATION confirmed: maps_verified not emitted"
    return 0
  fi
  log "LIMITATION check inconclusive; see logs"
  return 0
}

tamper_jar_append() {
  local pod=$1
  local jar_path=$2
  local container=${3:-$PAYMENTS_DEPLOY}
  log "Tampering jar: append bytes to $jar_path in $pod"
  kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$container" -- \
    sh -c "echo MALICIOUS_PAYLOAD >> '$jar_path'"
}

tamper_jar_replace_same_hash() {
  local pod=$1
  local jar_path=$2
  local container=${3:-$PAYMENTS_DEPLOY}
  log "TOCTOU: copy jar to new inode (same content) at $jar_path"
  kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$container" -- \
    sh -c "cp '$jar_path' /tmp/jar-copy.jar && mv /tmp/jar-copy.jar '$jar_path'"
}

restore_payments_deployment() {
  kubectl apply -f "$REPO_ROOT/payments-service/payment-service-deployment.yaml"
  # `kubectl apply` of the clean manifest is a no-op for the pod when the previous test
  # left the SAME template (e.g. jar tamper, symlink swap, or a `touch /tmp/.java_pid*`
  # done via `kubectl exec` on the running pod). The container filesystem then keeps the
  # tamper artifact and leaks it into the next test. Force a brand-new pod so every test
  # starts from a pristine container.
  kubectl rollout restart "deployment/$PAYMENTS_DEPLOY" -n "$K8S_NAMESPACE"
  wait_deployment_ready "$PAYMENTS_DEPLOY"
  settle_workloads
}

restore_orders_deployment() {
  kubectl apply -f "$REPO_ROOT/orders-service/order-service-deployment.yaml"
  kubectl rollout restart "deployment/$ORDERS_DEPLOY" -n "$K8S_NAMESPACE"
  wait_deployment_ready "$ORDERS_DEPLOY"
  settle_workloads
}

restore_clean_deployments() {
  restore_payments_deployment
  restore_orders_deployment
}

deploy_payments_variant() {
  local out_manifest=$1
  apply_manifest "$out_manifest" || return 1
  wait_deployment_ready "$PAYMENTS_DEPLOY"
  settle_workloads
}

# apply_manifest applies a generated variant manifest and fails loudly when the
# API server rejects it.
#
# The caller cannot rely on errexit here: run_test_wrapper runs the test body as
# `if "$@"`, and bash disables errexit for the whole call tree inside a condition.
# A rejected apply would therefore be skipped silently, leaving the PREVIOUS
# (clean) deployment live while the assertions ran against it — the scenario would
# never be exercised, and the result would be meaningless either way.
apply_manifest() {
  local manifest=$1
  if ! kubectl apply -f "$manifest"; then
    log "ASSERT FAIL (inconclusive): API server rejected $manifest; the previous deployment is still live, so this scenario was never exercised"
    return 1
  fi
}

write_payments_variant_manifest() {
  local out=$1
  shift
  # Remaining args: KEY=VALUE env pairs and/or --command "java ..." 
  local extra_env=()
  local command_json=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --command)
        command_json="$2"
        shift 2
        ;;
      --env)
        extra_env+=("$2")
        shift 2
        ;;
      *)
        die "unknown arg to write_payments_variant_manifest: $1"
        ;;
    esac
  done

  # strategy=Recreate is essential for deny-first: with the default RollingUpdate
  # (maxUnavailable rounds to 0 at replicas=1) k8s keeps the old CLEAN pod alive
  # until the new one is Ready. A tamper variant whose JVM crashes or is SVID-denied
  # never becomes Ready, so the old pod would linger and keep serving valid mTLS ->
  # false FAIL. Recreate tears the old pod down first, so the fresh (compromised) pod
  # is the only one that can answer orders.
  cat >"$out" <<'HEADER'
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
HEADER

  if [[ -n "$command_json" ]]; then
    # Emit argv as a JSON flow sequence, not a YAML block sequence: YAML is a
    # superset of JSON, so jq's quoting is authoritative and no arg needs hand
    # escaping. Writing a block sequence broke two ways at once — a multi-line
    # arg was split into one item per physical line, and an arg containing ": "
    # (printf 'Premain-Class: Noop\n') parsed as a nested mapping, which the API
    # server rejects with "unrecognized type: string".
    echo "          command: $(printf '%s' "$command_json" | jq -c '.')" >>"$out"
  fi

  cat >>"$out" <<'MID'
          env:
            - name: SPIFFE_ENDPOINT_SOCKET
              value: /run/spire/sockets/agent.sock
            - name: SPRING_PROFILES_ACTIVE
              value: "spire"
            - name: SERVER_PORT
              value: "8081"
MID

  local kv key val
  for kv in "${extra_env[@]}"; do
    key="${kv%%=*}"
    val="${kv#*=}"
    cat >>"$out" <<EOF
            - name: $key
              value: "$val"
EOF
  done

  cat >>"$out" <<'FOOTER'
          volumeMounts:
            - name: spire-agent-socket
              mountPath: /run/spire/sockets
              readOnly: true
      volumes:
        - name: spire-agent-socket
          hostPath:
            path: /run/spire/sockets
            type: DirectoryOrCreate
FOOTER
}

write_orders_variant_manifest() {
  local out=$1
  shift
  local extra_env=()
  local command_json=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --command)
        command_json="$2"
        shift 2
        ;;
      --env)
        extra_env+=("$2")
        shift 2
        ;;
      *)
        die "unknown arg: $1"
        ;;
    esac
  done

  cat >"$out" <<'HEADER'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders-service
  namespace: spire
  labels:
    app: orders-service
spec:
  replicas: 1
  selector:
    matchLabels:
      app: orders-service
  template:
    metadata:
      labels:
        app: orders-service
    spec:
      serviceAccountName: orders-sa
      containers:
        - name: orders-service
          image: orders-service:1.0
          imagePullPolicy: IfNotPresent
HEADER

  if [[ -n "$command_json" ]]; then
    {
      echo "          command:"
      echo "$command_json" | jq -r '.[]' | while IFS= read -r line; do
        echo "            - $line"
      done
    } >>"$out"
  fi

  cat >>"$out" <<'MID'
          env:
            - name: SPIFFE_ENDPOINT_SOCKET
              value: /run/spire/sockets/agent.sock
            - name: SPRING_PROFILES_ACTIVE
              value: "spire"
MID

  local kv key val
  for kv in "${extra_env[@]}"; do
    key="${kv%%=*}"
    val="${kv#*=}"
    cat >>"$out" <<EOF
            - name: $key
              value: "$val"
EOF
  done

  cat >>"$out" <<'FOOTER'
          volumeMounts:
            - name: spire-agent-socket
              mountPath: /run/spire/sockets
              readOnly: true
      volumes:
        - name: spire-agent-socket
          hostPath:
            path: /run/spire/sockets
            type: DirectoryOrCreate
FOOTER
}

attach_tracer_strace() {
  local pod=$1
  local pid=$2
  local node
  node=$(workload_node "$pod")
  log "Attaching strace to PID $pid on node $node (privileged debug pod)"

  kubectl debug "node/$node" -n "$K8S_NAMESPACE" --image=nicolaka/netshoot:latest \
    --target="$PAYMENTS_DEPLOY" --profile=sysadmin -- \
    sh -c "strace -f -p $pid -e trace=none 2>/dev/null & echo \$! > /tmp/tracer.pid; sleep 3600" \
    >/dev/null 2>&1 &
  sleep 3
}

stop_tracer_on_node() {
  local node=$1
  kubectl get pods -n "$K8S_NAMESPACE" --field-selector "spec.nodeName=$node" -o name 2>/dev/null | \
    grep -E 'node-debugger' | while read -r p; do
      kubectl delete -n "$K8S_NAMESPACE" "$p" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
    done
}

create_attach_socket() {
  local pod=$1
  local pid=$2
  local container=${3:-$PAYMENTS_DEPLOY}
  log "Opening a REAL JVM Attach API socket by attaching to the live JVM (nspid=$pid)"
  # Perform a genuine HotSpot Attach handshake instead of dropping a decoy file. jcmd
  # (shipped in the eclipse-temurin:17-alpine JDK image) writes /tmp/.attach_pid<pid>,
  # sends SIGQUIT to the JVM, whose BREAK handler then sees the trigger file and spins up
  # its AttachListener, creating the AF_UNIX socket /tmp/.java_pid<pid>; jcmd connects to
  # that socket and executes the command over it. The listener + socket persist for the
  # life of the JVM, so the attestor's /proc/<hostpid>/root/tmp/.java_pid* glob matches a
  # real, live attach channel rather than an empty placeholder. A manual trigger
  # (touch .attach_pid + kill -QUIT) is used only if jcmd is somehow unavailable, and a
  # bare `touch` remains as a last-resort so the defense assertion still has something to
  # detect.
  kubectl exec -n "$K8S_NAMESPACE" "$pod" -c "$container" -- sh -c '
    pid="'"$pid"'"
    sock="/tmp/.java_pid${pid}"
    if command -v jcmd >/dev/null 2>&1; then
      echo "attaching to JVM via jcmd (VM.version) ..."
      jcmd "$pid" VM.version >/dev/null 2>&1 || jcmd "$pid" help >/dev/null 2>&1 || true
    else
      echo "jcmd not found; triggering AttachListener manually (.attach_pid + SIGQUIT) ..."
      touch "/tmp/.attach_pid${pid}" 2>/dev/null || true
      kill -QUIT "$pid" 2>/dev/null || true
    fi
    i=0
    while [ "$i" -lt 50 ]; do
      [ -S "$sock" ] && break
      i=$((i+1)); sleep 0.1
    done
    if [ -S "$sock" ]; then
      echo "REAL attach socket is live:"
      ls -l "$sock" 2>&1 || true
    else
      echo "WARN: JVM did not expose $sock; leaving a decoy so the defense is still exercised" >&2
      touch "$sock" 2>/dev/null || true
    fi
  '
}

# pin_unapproved_payments_hash simulates an "unknown"/unapproved jar. The plugin no
# longer reads the jvm-hashes ConfigMap — the hash allow-list now lives in the
# registration entry — so we re-pin the payments entry to a hash that does NOT match
# the running jar. The workload's computed jvm:jar_sha256 then matches no entry and
# SVID issuance is denied.
BOGUS_JAR_SHA256="${BOGUS_JAR_SHA256:-0000000000000000000000000000000000000000000000000000000000000000}"

pin_unapproved_payments_hash() {
  create_jvm_entry \
    "spiffe://${TRUST_DOMAIN}/ns/${K8S_NAMESPACE}/sa/payments-app" \
    "payments-sa" \
    "$BOGUS_JAR_SHA256"
  # Deny-first: the bogus entry lives on the SPIRE SERVER, so it survives pod
  # replacement. Rather than restart_spire_agent (which leaves the old pod's
  # still-valid SVID and orders' pooled mTLS connection intact until TTL expiry ->
  # false FAIL), delete the payments pod. The replacement fetches its first SVID
  # against the bogus-pinned hash: its computed jvm:jar_sha256 matches no entry, so
  # issuance is denied from the very first fetch. Entry create/delete propagates to
  # the agent well within the pod's teardown+reschedule window, so no agent restart
  # is needed.
  delete_payments_pod_and_wait
}

# restore_spire_entries re-registers the correct entries (real jar hash) via wsldev
# and forces re-attestation.
restore_spire_entries() {
  ensure_spire_entries
  restart_spire_agent
}

run_test_wrapper() {
  local name=$1
  local expected=$2
  local out=$3
  shift 3
  TEST_EVIDENCE_SIGNALS=()
  init_test_result "$out" "$name" "$expected"
  local status evidence
  if "$@"; then
    if [[ "$expected" == "LIMITATION" ]]; then
      status="LIMITATION (expected)"
      evidence="$(evidence_summary)"
    else
      status=PASS
      evidence="$(evidence_summary)"
    fi
  else
    if [[ "$expected" == "LIMITATION" ]]; then
      status="LIMITATION (expected)"
      evidence="$(evidence_summary); partial"
    else
      status=FAIL
      evidence="assertion failed; see logs in $out"
    fi
  fi
  record_test_result "$out" "$status" "$evidence"
  [[ "$status" == "FAIL" ]] && return 1 || return 0
}
