#!/bin/bash
# run-scenario5-scaling.sh
# Запускає scenario5 (паралельне навантаження) і автоматично масштабує
# deployment точно в момент, коли навантаження досягає піку (scale-up)
# та коли воно спадає (scale-down). Час синхронізований з фазами
# в scenario5-identity-stress.js: T_RAMP_END=180s, T_POST_SCALE_END=540s.

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

SPIRE_URL="${SPIRE_URL:-http://localhost:8080}"
AUTH_TYPE="${AUTH_TYPE:-SPIRE}"
NAMESPACE="${K8S_NAMESPACE:-spire}"
DEPLOYMENT="${K8S_DEPLOYMENT:-payments-service}"
MIN_REPLICAS="${MIN_REPLICAS:-1}"
MAX_REPLICAS="${MAX_REPLICAS:-15}"

# Мають збігатися з константами T_RAMP_END / T_POST_SCALE_END у .js файлі
SCALE_UP_AT_SEC="${SCALE_UP_AT_SEC:-180}"
SCALE_DOWN_AT_SEC="${SCALE_DOWN_AT_SEC:-540}"

RESULTS_DIR="test-results-scenario5-scaling-${AUTH_TYPE}-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$RESULTS_DIR"

echo -e "${BLUE}🚀 Scenario 5: Horizontal Scaling Under Parallel Load${NC}"
echo -e "Target: ${GREEN}$SPIRE_URL${NC}"
echo -e "Deployment: ${GREEN}${DEPLOYMENT}${NC} (namespace: ${NAMESPACE})"
echo -e "Replicas: ${GREEN}${MIN_REPLICAS} -> ${MAX_REPLICAS} -> ${MIN_REPLICAS}${NC}\n"

scale_deployment() {
  local replicas=$1
  local label=$2
  echo -e "${YELLOW}⚡ [$(date +%H:%M:%S)] Scaling ${DEPLOYMENT} to ${replicas} replicas (${label})${NC}"
  kubectl scale deployment/"$DEPLOYMENT" -n "$NAMESPACE" --replicas="$replicas"
}

export_prometheus_data() {
  local start_time=$1
  local end_time=$2

  echo -e "${BLUE}📊 Exporting Prometheus data...${NC}"
  metrics=(
    "http_server_requests_seconds_bucket"
    "process_cpu_usage"
    "jvm_memory_used_bytes"
    "spring_security_filterchains_seconds_sum"
    "spire_server_rpc_svid_v1_svid_batch_new_x509svid"
    "kube_deployment_status_replicas_ready"
    "kube_deployment_status_replicas"
  )
  for metric in "${metrics[@]}"; do
    curl -sG "http://localhost:9090/api/v1/query_range" \
      --data-urlencode "query=$metric" \
      --data-urlencode "start=$start_time" \
      --data-urlencode "end=$end_time" \
      --data-urlencode "step=15s" \
      -o "$RESULTS_DIR/prometheus-scenario5-${AUTH_TYPE}-${metric}.json" 2>/dev/null
  done
  echo -e "${GREEN}✓ Prometheus data exported${NC}\n"
}

START_TIME=$(date -u +%s)

# --- Запускаємо k6 у фоні -------------------------------------------------
k6 run \
  -e BASE_URL="$SPIRE_URL" \
  -e AUTH_TYPE="$AUTH_TYPE" \
  -e K8S_NAMESPACE="$NAMESPACE" \
  -e K8S_DEPLOYMENT="$DEPLOYMENT" \
  -e MIN_REPLICAS="$MIN_REPLICAS" \
  -e MAX_REPLICAS="$MAX_REPLICAS" \
  scenario5-identity-stress.js \
  | tee "$RESULTS_DIR/scenario5-${AUTH_TYPE}-output.log" &
K6_PID=$!

# --- Плануємо scale-up рівно на початок фази ramp_up ----------------------
(
  sleep "$SCALE_UP_AT_SEC"
  scale_deployment "$MAX_REPLICAS" "scale-out під навантаженням"
) &

# --- Плануємо scale-down рівно на кінець фази post_scale -------------------
(
  sleep "$SCALE_DOWN_AT_SEC"
  scale_deployment "$MIN_REPLICAS" "scale-in під спадаючим навантаженням"
) &

# Чекаємо завершення k6 (основний критерій закінчення тесту)
wait "$K6_PID"
K6_EXIT_CODE=$?

END_TIME=$(date -u +%s)

if [ $K6_EXIT_CODE -eq 0 ]; then
  export_prometheus_data "$START_TIME" "$END_TIME"

  g_start=$((START_TIME * 1000))
  g_end=$((END_TIME * 1000))
  dashboard_id="spire-by-instance"
  if [ "$AUTH_TYPE" == "JWT" ]; then
    dashboard_id="jwt-by-instance"
  fi

  echo -e "🔗 Grafana [$AUTH_TYPE]: ${BLUE}http://localhost:3000/d/$dashboard_id?from=$g_start&to=$g_end${NC}"
  echo -e "${GREEN}✅ Test finished!${NC}"
else
  echo -e "${RED}❌ Test failed (exit code $K6_EXIT_CODE).${NC}"
fi

# --- Безпека: гарантуємо повернення до MIN_REPLICAS навіть при збої тесту --
echo -e "${YELLOW}↩️  Ensuring deployment is scaled back to ${MIN_REPLICAS} replicas...${NC}"
scale_deployment "$MIN_REPLICAS" "cleanup"

exit $K6_EXIT_CODE
