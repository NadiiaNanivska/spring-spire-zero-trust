#!/bin/bash
# run-all-tests.sh
# Майстер-скрипт для запуску всіх сценаріїв тестування JWT vs SPIRE

set -e  # Exit on error

# Кольори для виводу
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Конфігурація
JWT_URL="${JWT_URL:-http://localhost:8080}"
SPIRE_URL="${SPIRE_URL:-http://localhost:8080}"
JWT_TOKEN="${JWT_TOKEN:-}"

# Створення директорії для результатів
RESULTS_DIR="test-results-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$RESULTS_DIR"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  JWT vs SPIRE Testing Suite${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "JWT Endpoint:   ${GREEN}$JWT_URL${NC}"
echo -e "SPIRE Endpoint: ${GREEN}$SPIRE_URL${NC}"
echo -e "Results dir:    ${GREEN}$RESULTS_DIR${NC}"
echo ""

# Функція для запуску одного тесту
run_test() {
    local scenario=$1
    local auth_type=$2
    local base_url=$3
    local jwt_token=$4
    local t_start=$(date -u +%s)
    
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Running: Scenario $scenario - $auth_type${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    k6 run \
#        --out json="$RESULTS_DIR/scenario${scenario}-${auth_type}-raw.json" \
        -e BASE_URL="$base_url" \
        -e AUTH_TYPE="$auth_type" \
        -e JWT_TOKEN="$jwt_token" \
        "scenario${scenario}-*.js" \
        | tee "$RESULTS_DIR/scenario${scenario}-${auth_type}-output.log"
    
    local exit_code=$?

    local t_end=$(date -u +%s)

    local g_start=$((t_start * 1000))
    local g_end=$((t_end * 1000))

    local dashboard_id=""
        if [ "$auth_type" == "JWT" ]; then
            dashboard_id="jwt-by-instance"
        else
            dashboard_id="spire-by-instance"
        fi

    echo -e "🔗 Grafana [$auth_type]: ${BLUE}http://localhost:3000/d/$dashboard_id?from=$g_start&to=$g_end${NC}"
    
    if [ $exit_code -eq 0 ]; then
        echo -e "${GREEN}✓ Scenario $scenario ($auth_type) completed successfully${NC}"
    else
        echo -e "${RED}✗ Scenario $scenario ($auth_type) failed with exit code $exit_code${NC}"
    fi
    
    echo ""
    return $exit_code
}

# Функція для паузи між тестами
pause_between_tests() {
    local minutes=$1
    echo -e "${BLUE}⏸  Cooling down for $minutes minutes...${NC}"
    echo "   Use this time to:"
    echo "   - Check Grafana dashboards"
    echo "   - Take screenshots"
    echo "   - Export metrics"
    echo ""
    sleep "${minutes}m"
}

# Функція для експорту даних з Prometheus
export_prometheus_data() {
    local auth_type=$1
    local start_time=$2
    local end_time=$3
    local scenario=$4
    
    echo -e "${BLUE}📊 Exporting Prometheus data...${NC}"
    
    # Список ключових метрик
    metrics=(
        "http_server_requests_seconds_bucket"
        "process_cpu_usage"
        "jvm_memory_used_bytes"
        "spring_security_filterchains_seconds_sum"
        "spire_server_rpc_svid_v1_svid_batch_new_x509svid"
    )
    
    for metric in "${metrics[@]}"; do
        curl -G "http://localhost:9090/api/v1/query_range" \
            --data-urlencode "query=$metric" \
            --data-urlencode "start=$start_time" \
            --data-urlencode "end=$end_time" \
            --data-urlencode "step=15s" \
            -o "$RESULTS_DIR/prometheus-${scenario}-${auth_type}-${metric}.json" \
            2>/dev/null || echo "Warning: Failed to export $metric"
    done
    
    echo -e "${GREEN}✓ Prometheus data exported${NC}"
    echo ""
}

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# ПОЧАТОК ТЕСТУВАННЯ
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}  Pre-flight Checks${NC}"
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo ""

# Перевірка доступності endpoints
echo "Checking JWT endpoint..."
if curl -f -s -o /dev/null "$JWT_URL/actuator/health"; then
    echo -e "${GREEN}✓ JWT service is UP${NC}"
else
    echo -e "${RED}✗ JWT service is DOWN${NC}"
    exit 1
fi

echo "Checking SPIRE endpoint..."
if curl -f -s -o /dev/null "$SPIRE_URL/actuator/health"; then
    echo -e "${GREEN}✓ SPIRE service is UP${NC}"
else
    echo -e "${RED}✗ SPIRE service is DOWN${NC}"
    exit 1
fi

echo "Checking k6 installation..."
if command -v k6 &> /dev/null; then
    echo -e "${GREEN}✓ k6 is installed ($(k6 version))${NC}"
else
    echo -e "${RED}✗ k6 is not installed${NC}"
    echo "Install from: https://k6.io/docs/get-started/installation/"
    exit 1
fi

echo ""
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}  Ready to start testing!${NC}"
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
read -p "Press Enter to continue or Ctrl+C to abort..."
echo ""

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# SCENARIO 1: Steady State Baseline
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}  SCENARIO 1: Steady State Baseline${NC}"
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo ""

START_TIME=$(date -u +%s)
run_test "1" "JWT" "$JWT_URL" "$JWT_TOKEN"
END_TIME=$(date -u +%s)
export_prometheus_data "JWT" "$START_TIME" "$END_TIME" "1"

pause_between_tests 5

START_TIME=$(date -u +%s)
run_test "1" "SPIRE" "$SPIRE_URL" ""
END_TIME=$(date -u +%s)
export_prometheus_data "SPIRE" "$START_TIME" "$END_TIME" "1"

pause_between_tests 5

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# SCENARIO 2: Connection Churn
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}  SCENARIO 2: Connection Churn${NC}"
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo ""

START_TIME=$(date -u +%s)
run_test "2" "JWT" "$JWT_URL" "$JWT_TOKEN"
END_TIME=$(date -u +%s)
export_prometheus_data "JWT" "$START_TIME" "$END_TIME" "2"

pause_between_tests 5

START_TIME=$(date -u +%s)
run_test "2" "SPIRE" "$SPIRE_URL" ""
END_TIME=$(date -u +%s)
export_prometheus_data "SPIRE" "$START_TIME" "$END_TIME" "2"

pause_between_tests 5

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# SCENARIO 3: Keep-Alive Efficiency
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}  SCENARIO 3: Keep-Alive Efficiency${NC}"
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo ""

START_TIME=$(date -u +%s)
run_test "3" "JWT" "$JWT_URL" "$JWT_TOKEN"
END_TIME=$(date -u +%s)
export_prometheus_data "JWT" "$START_TIME" "$END_TIME" "3"

pause_between_tests 5

START_TIME=$(date -u +%s)
run_test "3" "SPIRE" "$SPIRE_URL" ""
END_TIME=$(date -u +%s)
export_prometheus_data "SPIRE" "$START_TIME" "$END_TIME" "3"

pause_between_tests 5

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# SCENARIO 4: Network Payload
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}  SCENARIO 4: Network Payload Analysis${NC}"
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo ""

START_TIME=$(date -u +%s)
run_test "4" "JWT" "$JWT_URL" "$JWT_TOKEN"
END_TIME=$(date -u +%s)
export_prometheus_data "JWT" "$START_TIME" "$END_TIME" "4"

pause_between_tests 5

START_TIME=$(date -u +%s)
run_test "4" "SPIRE" "$SPIRE_URL" ""
END_TIME=$(date -u +%s)
export_prometheus_data "SPIRE" "$START_TIME" "$END_TIME" "4"

pause_between_tests 5

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# SCENARIO 5: Identity Stress (Manual)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}  SCENARIO 5: Identity Issuance Stress${NC}"
echo -e "${GREEN}  (Requires manual kubectl scaling)${NC}"
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo ""
echo -e "${YELLOW}This test requires manual intervention.${NC}"
echo "Run separately with:"
echo "  k6 run -e BASE_URL=$SPIRE_URL -e AUTH_TYPE=SPIRE scenario5-identity-stress.js"
echo ""
echo "Skip this test? (y/n)"
read -r skip_scenario5

if [ "$skip_scenario5" != "y" ]; then
    START_TIME=$(date -u +%s)
    run_test "5" "SPIRE" "$SPIRE_URL" ""
    END_TIME=$(date -u +%s)
    export_prometheus_data "SPIRE" "$START_TIME" "$END_TIME" "5"
fi

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# ФІНАЛЬНИЙ ЗВІТ
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

echo ""
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}  🎉 All Tests Completed!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo ""
echo "Results saved to: $RESULTS_DIR/"
echo ""
echo "Next steps:"
echo "  1. Review JSON results in $RESULTS_DIR/"
echo "  2. Export Grafana dashboards to PDF"
echo "  3. Take screenshots of key metrics"
echo "  4. Run analysis: python3 analyze_results.py"
echo ""
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}  Don't forget to document:${NC}"
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo "  • Grafana dashboard screenshots"
echo "  • Prometheus query exports"
echo "  • System configuration details"
echo "  • Unexpected observations"
echo ""
