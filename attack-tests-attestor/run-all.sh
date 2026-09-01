#!/usr/bin/env bash
# Master orchestrator for JVM attestor attack / resilience tests.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"

RUN_TS=$(date -u +%Y%m%d-%H%M%S)
RESULTS_DIR="${RESULTS_DIR:-$LIB_DIR/results/run-$RUN_TS}"
export RESULTS_DIR

SKIP_SETUP=0
TESTS=()

usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  --results-dir DIR   Output root (default: attack-tests-attestor/results/run-<timestamp>)
  --skip-setup        Skip setup.sh (cluster already prepared)
  --tests LIST        Comma-separated test scripts (default: all)
  -h, --help          Show help

Environment: K8S_NAMESPACE, SETTLE_SEC, LARGE_JAR_MB, WSLDEV_BIN
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --results-dir)
      RESULTS_DIR=$2
      export RESULTS_DIR
      shift 2
      ;;
    --skip-setup)
      SKIP_SETUP=1
      shift
      ;;
    --tests)
      IFS=',' read -ra TESTS <<<"$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

mkdir -p "$RESULTS_DIR"
trap cleanup_port_forwards EXIT

ALL_TESTS=(
  attack-antidebug.sh
  attack-tamper-flags.sh
  attack-tamper-env.sh
  attack-attach-socket.sh
  attack-jar-unknown.sh
  attack-cp-classpath.sh
  bypass-symlink.sh
  dos-large-jar.sh
)

if [[ ${#TESTS[@]} -eq 0 ]]; then
  TESTS=("${ALL_TESTS[@]}")
fi

log "Attack test run -> $RESULTS_DIR"
preflight

if [[ $SKIP_SETUP -eq 0 ]]; then
  "$LIB_DIR/setup.sh" "$RESULTS_DIR"
fi

FAILURES=0
SUMMARY="$RESULTS_DIR/summary.md"

{
  echo "# JVM Attestor Attack Test Summary"
  echo ""
  echo "Run: \`$RUN_TS\`"
  echo ""
  echo "| Test | Expected | Status | Evidence |"
  echo "|------|----------|--------|----------|"
} >"$SUMMARY"

for script in "${TESTS[@]}"; do
  path="$LIB_DIR/$script"
  [[ -f "$path" ]] || die "test script not found: $script"
  log "======== Running $script ========"
  if bash "$path" "$RESULTS_DIR"; then
    rc=0
  else
    rc=$?
    FAILURES=$((FAILURES + 1))
  fi

  name="${script%.sh}"
  meta=$(find "$RESULTS_DIR" -name meta.env -mmin -2 2>/dev/null | head -1)
  test_name="$name"
  expected="PASS"
  status="UNKNOWN"
  evidence=""
  if [[ -n "$meta" ]] && [[ -f "$meta" ]]; then
    # shellcheck disable=SC1090
    source "$meta"
    test_name="${TEST_NAME:-$name}"
    expected="${EXPECTED_STATUS:-PASS}"
    status="${STATUS:-UNKNOWN}"
    evidence="${EVIDENCE:-}"
  fi
  echo "| $test_name | $expected | $status | ${evidence:-n/a} |" >>"$SUMMARY"
  log "Finished $script (rc=$rc status=$status)"
done

{
  echo ""
  echo "## Totals"
  echo ""
  echo "- Tests run: ${#TESTS[@]}"
  echo "- Failures: $FAILURES"
  echo ""
  echo "Status legend: **PASS** = defense worked; **FAIL** = defense missed; **LIMITATION (expected)** = documented bypass boundary."
} >>"$SUMMARY"

log "All tests complete. Failures: $FAILURES"
log "Summary: $SUMMARY"
[[ $FAILURES -eq 0 ]] || exit 1
