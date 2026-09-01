#!/usr/bin/env bash
# Generate comparison charts + tidy CSV from a run directory.
# Standalone: NOT called by run-all.sh.
# Usage: plot-results.sh [run-dir]
#   run-dir defaults to the newest results/run-* directory.
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_ROOT="${RESULTS_ROOT:-$LIB_DIR/results}"

RUN_DIR="${1:-}"
if [[ -z "$RUN_DIR" ]]; then
  RUN_DIR=$(find "$RESULTS_ROOT" -maxdepth 1 -type d -name 'run-*' 2>/dev/null | sort | tail -1)
  [[ -n "$RUN_DIR" ]] || { echo "ERROR: no run-* dir under $RESULTS_ROOT; pass one explicitly" >&2; exit 1; }
fi

PY=python3
command -v "$PY" >/dev/null 2>&1 || PY=python
command -v "$PY" >/dev/null 2>&1 || { echo "ERROR: python3 not found" >&2; exit 1; }

if ! "$PY" -c 'import matplotlib' >/dev/null 2>&1; then
  echo "matplotlib not found; install with: $PY -m pip install matplotlib" >&2
  exit 2
fi

echo "Plotting run: $RUN_DIR"
exec "$PY" "$LIB_DIR/plot_results.py" "$RUN_DIR"
