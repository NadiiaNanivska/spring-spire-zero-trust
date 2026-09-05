#!/usr/bin/env bash
# Aggregate N independent run-all.sh results + generate thesis tables and CI plots.
#
# Usage:
#   ./aggregate-multi.sh results/run-ts1 results/run-ts2 results/run-ts3
#   ./aggregate-multi.sh --glob 'results/run-*' --last 3
#   ./aggregate-multi.sh --last 3          # newest 3 runs under results/
#
# Outputs under results/aggregate-<utc-ts>/ (or --out-dir):
#   run_scalars.csv, summary_stats.csv, results_tables.html, long_all.csv, plots_clean/
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_ROOT="${RESULTS_ROOT:-$LIB_DIR/results}"

PY=python3
command -v "$PY" >/dev/null 2>&1 || PY=python
command -v "$PY" >/dev/null 2>&1 || { echo "ERROR: python3 not found" >&2; exit 1; }

for mod in pandas matplotlib seaborn; do
  if ! "$PY" -c "import $mod" >/dev/null 2>&1; then
    echo "ERROR: missing Python module '$mod'. Install with:" >&2
    echo "  $PY -m pip install pandas matplotlib seaborn" >&2
    exit 2
  fi
done

usage() {
  cat <<EOF
Usage: $0 [options] [run-dir ...]

Aggregate multiple load-test run directories into statistically valid summaries.

Options:
  --glob PATTERN   Glob for run dirs (default when no dirs given: results/run-*)
  --last N         Keep only the N most recent matched run directories
  --out-dir DIR    Output directory (default: results/aggregate-<utc-ts>)
  -h, --help       Show this help

Examples:
  $0 results/run-20260721-132745 results/run-20260721-160830 results/run-20260729-165811
  $0 --last 3
  $0 --glob 'results/run-*' --last 3 --out-dir results/aggregate-thesis
EOF
}

RUN_DIRS=()
GLOB_PATTERN=""
LAST_N=0
OUT_DIR=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --glob)
      GLOB_PATTERN=$2
      shift 2
      ;;
    --last)
      LAST_N=$2
      shift 2
      ;;
    --out-dir)
      OUT_DIR=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)
      echo "ERROR: unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
    *)
      RUN_DIRS+=("$1")
      shift
      ;;
  esac
done

AGG_ARGS=()
if [[ ${#RUN_DIRS[@]} -gt 0 ]]; then
  AGG_ARGS+=("${RUN_DIRS[@]}")
elif [[ -n "$GLOB_PATTERN" ]]; then
  AGG_ARGS+=(--glob "$GLOB_PATTERN")
else
  AGG_ARGS+=(--glob "$RESULTS_ROOT/run-*")
fi

if [[ "$LAST_N" -gt 0 ]]; then
  AGG_ARGS+=(--last "$LAST_N")
fi

if [[ -n "$OUT_DIR" ]]; then
  AGG_ARGS+=(--out-dir "$OUT_DIR")
fi

cd "$LIB_DIR"
echo "=== Aggregating runs ==="
"$PY" "$LIB_DIR/aggregate_runs.py" "${AGG_ARGS[@]}"

# Resolve output dir (aggregate_runs prints it; re-derive if --out-dir was set)
if [[ -n "$OUT_DIR" ]]; then
  AGG_OUT="$OUT_DIR"
else
  AGG_OUT=$(find "$RESULTS_ROOT" -maxdepth 1 -type d -name 'aggregate-*' 2>/dev/null | sort | tail -1)
  [[ -n "$AGG_OUT" ]] || { echo "ERROR: could not find aggregate output dir" >&2; exit 1; }
fi

echo ""
if [[ -f "$LIB_DIR/pooled_stats.py" ]]; then
  echo "=== Pooled-sample stats + Mann-Whitney U (alternative to run-level t-CI) ==="
  "$PY" "$LIB_DIR/pooled_stats.py" "$AGG_OUT" \
    || echo "WARNING: pooled_stats.py failed; continuing without pooled analysis." >&2
else
  echo "=== Skipping pooled-sample stats (pooled_stats.py not present) ==="
fi

echo ""
echo "=== Generating time-series plots (run-to-run 95% t-CI) ==="
"$PY" "$LIB_DIR/spire_metrics_visualization.py" "$AGG_OUT"

echo ""
echo "All done. Thesis artifacts:"
echo "  Tables (t-CI)        : $AGG_OUT/results_tables.html"
echo "  Stats  (t-CI)        : $AGG_OUT/summary_stats.csv"
echo "  Plots  (t-CI bands)  : $AGG_OUT/plots_clean/"
if [[ -f "$AGG_OUT/pooled_stats.csv" ]]; then
  echo "  Stats  (pooled M-W)  : $AGG_OUT/pooled_stats.csv"
  echo "  Compare (pooled M-W) : $AGG_OUT/pooled_stats_comparison.csv"
  echo "  Tables (pooled M-W)  : $AGG_OUT/results_tables_pooled.html"
fi
