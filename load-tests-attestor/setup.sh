#!/usr/bin/env bash
# Bootstrap cluster state for attestor load tests (workloads + custom-jvm + SPIRE entries).
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

log "=== Load test setup ==="
preflight
ensure_workload_apps
ensure_orders_service
deploy_overlay custom-jvm
log "Setup complete. Run scenarios or ./run-all.sh"
