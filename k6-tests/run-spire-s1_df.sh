#!/bin/bash

# Кольори
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Конфігурація (зміни URL, якщо треба)
SPIRE_URL="${SPIRE_URL:-http://localhost:8080}"
RESULTS_DIR="test-results-df-spire-s1-steady-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$RESULTS_DIR"

# Функція для запуску тесту (з твого файлу)
run_test() {
    local scenario=$1
    local auth_type=$2
    local base_url=$3
    local t_start=$(date -u +%s)

    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Running: Scenario $scenario - Steady State - $auth_type${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    k6 run \
        -e BASE_URL="$base_url" \
        -e AUTH_TYPE="$auth_type" \
        "scenario${scenario}"-*.js \
        | tee "$RESULTS_DIR/scenario${scenario}-steady-df-${auth_type}-output.log"

    local exit_code=$?
    local t_end=$(date -u +%s)

    local g_start=$((t_start * 1000))
    local g_end=$((t_end * 1000))
    local dashboard_id="spire-by-instance"

    echo ""
    echo -e "🔗 Grafana [$auth_type]: ${BLUE}http://localhost:3000/d/$dashboard_id?from=$g_start&to=$g_end${NC}"

    return $exit_code
}

export_prometheus_data() {
    local auth_type=$1
    local start_time=$2
    local end_time=$3
    local scenario=$4

    echo -e "${BLUE}📊 Exporting Prometheus data...${NC}"
    metrics=("http_server_requests_seconds_bucket" "process_cpu_usage" "jvm_memory_used_bytes" "spring_security_filterchains_seconds_sum" "spire_server_rpc_svid_v1_svid_batch_new_x509svid")

    for metric in "${metrics[@]}"; do
        curl -G "http://localhost:9090/api/v1/query_range" \
            --data-urlencode "query=$metric" \
            --data-urlencode "start=$start_time" \
            --data-urlencode "end=$end_time" \
            --data-urlencode "step=15s" \
            -o "$RESULTS_DIR/prometheus-${scenario}-${auth_type}-${metric}.json" 2>/dev/null
    done
    echo -e "${GREEN}✓ Prometheus data exported${NC}\n"
}


echo -e "${BLUE}🚀 Starting single test: SPIRE Scenario 1 Steady State df${NC}"
echo -e "Target: ${GREEN}$SPIRE_URL${NC}\n"

START_TIME=$(date -u +%s)

if run_test "1" "SPIRE" "$SPIRE_URL" ""; then
    END_TIME=$(date -u +%s)
    export_prometheus_data "SPIRE" "$START_TIME" "$END_TIME" "1"
    echo -e "${GREEN}✅ Test finished! Check the Grafana link above.${NC}"
else
    echo -e "${RED}❌ Test failed.${NC}"
fi