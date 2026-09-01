#!/usr/bin/env bash
# Compare default vs custom-jvm runs -> results/summary.csv and summary.md
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$LIB_DIR/lib.sh"
RESULTS_ROOT=${1:-${RESULTS_DIR:-$LIB_DIR/results}}

SUMMARY_CSV="$RESULTS_ROOT/summary.csv"
SUMMARY_MD="$RESULTS_ROOT/summary.md"

# Mean of all sample values across all series over the window.
# tonumber parses "NaN" into a NaN number here (not an error) and NaN == NaN is
# true in jq 1.7, so drop non-finite values with isnan/isinfinite.
prom_avg() {
  local file=$1
  [[ -f "$file" ]] || { echo ""; return; }
  jq -r '
    [.data.result[].values[][1] | (tonumber? // empty) | select((isnan or isinfinite) | not)]
    | if length == 0 then "" else (add / length) end
  ' "$file" 2>/dev/null || echo ""
}

# Peak sample across all series (kept only where a spike is meaningful).
prom_max() {
  local file=$1
  [[ -f "$file" ]] || { echo ""; return; }
  jq -r '
    [.data.result[].values[][1] | (tonumber? // empty) | select((isnan or isinfinite) | not)]
    | if length == 0 then "" else max end
  ' "$file" 2>/dev/null || echo ""
}

# For monotonic counters: sum over series of (last - first) across the window.
prom_counter_delta() {
  local file=$1
  [[ -f "$file" ]] || { echo ""; return; }
  jq -r '
    [ .data.result[]
      | (.values | map(.[1] | (tonumber? // empty) | select((isnan or isinfinite) | not)))
      | select(length > 0)
      | (.[-1] - .[0]) ]
    | if length == 0 then "" else add end
  ' "$file" 2>/dev/null || echo ""
}

# Mean full workload-attestation time in ms, computed from the raw histogram
# _sum/_count counters (SPIRE reports elapsed time in milliseconds, so no /1e6).
attestation_avg_ms() {
  local prom=$1
  local s c
  s=$(prom_counter_delta "$prom/attestation_elapsed_sum.json")
  c=$(prom_counter_delta "$prom/attestation_elapsed_count.json")
  if [[ -n "$s" && -n "$c" ]]; then
    awk -v s="$s" -v c="$c" 'BEGIN { if (c > 0) printf "%.3f", s / c; else printf "" }'
  else
    echo ""
  fi
}

k6_metric() {
  local file=$1
  local path=$2
  [[ -f "$file" ]] || { echo ""; return; }
  jq -r "$path // \"\"" "$file" 2>/dev/null || echo ""
}

csv_stats() {
  local file=$1
  local col=$2
  [[ -f "$file" ]] || { echo ","; return; }
  awk -F, -v c="$col" '
    NR==1 { for (i=1;i<=NF;i++) if ($i==c) ci=i; next }
    ci && $ci ~ /^[0-9]+([.][0-9]+)?$/ { sum+=$ci; n++; if ($ci>max||max=="") max=$ci }
    END { if (n>0) printf "%.2f,%.2f", sum/n, max; else printf "," }
  ' "$file"
}

emit_delta() {
  # $1=default $2=custom ; prints signed pct or empty
  local d=$1 c=$2
  if [[ -n "$d" && -n "$c" && "$d" != "0" && "$d" != "null" ]]; then
    awk -v d="$d" -v c="$c" 'BEGIN { printf "%.1f", (c - d) / d * 100 }'
  fi
}

find_latest_run() {
  local overlay=$1
  local scenario=$2
  find "$RESULTS_ROOT" -maxdepth 1 -type d -name "${overlay}-scenario-${scenario}-*" 2>/dev/null | sort | tail -1
}

SCENARIOS=(a b c)

{
  echo "scenario,metric,default,custom_jvm,delta_pct"
  for sc in "${SCENARIOS[@]}"; do
    def_dir=$(find_latest_run "default" "$sc")
    cj_dir=$(find_latest_run "custom-jvm" "$sc")
    [[ -n "$def_dir" && -n "$cj_dir" ]] || continue

    def_prom="$def_dir/prometheus"
    cj_prom="$cj_dir/prometheus"

    # Attestation time (ms) from raw counters -- the primary, unit-correct signal.
    d=$(attestation_avg_ms "$def_prom")
    c=$(attestation_avg_ms "$cj_prom")
    echo "$sc,attestation_avg_ms,$d,$c,$(emit_delta "$d" "$c")"

    # Mean-over-window metrics (stable; not noisy single peaks).
    metrics_avg=(
      "agent_cpu_cores_avg|agent_cpu.json"
      "agent_memory_mb_avg|agent_memory_mb.json"
      "server_cpu_cores_avg|server_cpu.json"
      "server_memory_mb_avg|server_memory_mb.json"
      "http_p95_ms_avg|http_req_p95_ms.json"
      "http_p99_ms_avg|http_req_p99_ms.json"
      "http_5xx_rate_avg|http_5xx_rate.json"
      "svid_issued_rate_avg|svid_issued_rate.json"
    )
    for m in "${metrics_avg[@]}"; do
      name=${m%%|*}
      file=${m#*|}
      d=$(prom_avg "$def_prom/$file")
      c=$(prom_avg "$cj_prom/$file")
      echo "$sc,$name,$d,$c,$(emit_delta "$d" "$c")"
    done

    # Peak HTTP p99 -- a spike here is meaningful (rollout/scale disruption).
    d=$(prom_max "$def_prom/http_req_p99_ms.json")
    c=$(prom_max "$cj_prom/http_req_p99_ms.json")
    echo "$sc,http_p99_ms_max,$d,$c,$(emit_delta "$d" "$c")"

    # k6 client-side view.
    def_k6="$def_dir/k6-summary.json"
    cj_k6="$cj_dir/k6-summary.json"
    if [[ -f "$def_k6" && -f "$cj_k6" ]]; then
      d=$(k6_metric "$def_k6" '.metrics.http_req_duration.values["p(95)"]')
      c=$(k6_metric "$cj_k6" '.metrics.http_req_duration.values["p(95)"]')
      echo "$sc,k6_http_p95_ms,$d,$c,$(emit_delta "$d" "$c")"
      d=$(k6_metric "$def_k6" '.metrics.http_req_failed.values.rate // .metrics.errors.values.rate')
      c=$(k6_metric "$cj_k6" '.metrics.http_req_failed.values.rate // .metrics.errors.values.rate')
      echo "$sc,k6_error_rate,$d,$c,"
    fi

    # Plugin-only cost from agent logs (custom side only; the cleanest overhead).
    cj_csv="$cj_dir/attestor-timing.csv"
    if [[ -f "$cj_csv" ]]; then
      read -r avg_jar max_jar <<<"$(csv_stats "$cj_csv" jar_hash_us | tr ',' ' ')"
      read -r avg_total max_total <<<"$(csv_stats "$cj_csv" total_us | tr ',' ' ')"
      echo "$sc,jvm_jar_hash_us_avg,,${avg_jar},"
      echo "$sc,jvm_jar_hash_us_max,,${max_jar},"
      echo "$sc,jvm_attest_total_us_avg,,${avg_total},"
      echo "$sc,jvm_attest_total_us_max,,${max_total},"
    fi
  done
} >"$SUMMARY_CSV"

{
  echo "# Attestor overhead comparison (default vs custom-jvm)"
  echo ""
  echo "Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo ""
  echo "## How to read"
  echo ""
  echo "- \`delta %\` = (custom-jvm − default) / default × 100. Negative = custom-jvm lower."
  echo "- For latency / CPU / memory / error metrics, **lower is better**; a positive delta is plugin overhead."
  echo "- \`attestation_avg_ms\` = mean of the **whole** workload attestation (k8s + unix + jvm) in ms,"
  echo "  from raw \`_sum\`/\`_count\`. The k8s attestor (kubelet call) dominates, so small deltas here are noise."
  echo "- \`jvm_*_us\` rows = **plugin-only** cost in **microseconds** (from agent logs); custom-jvm side only."
  echo "  These are the cleanest measure of what the plugin itself costs. Compare S-A (warm cache)"
  echo "  vs S-B (cold compute) \`jvm_jar_hash_us_*\` to read the hash-cache benefit."
  echo "- Prometheus rows are **means over the measurement window** (except \`http_p99_ms_max\`)."
  echo "- default and custom-jvm are separate runs, so treat small (<~30%) deltas as run-to-run noise."
  echo ""
  # Round numeric cells to 4 significant figures for readability; leave blanks as em-dash.
  fmt() {
    local v=$1
    [[ -z "$v" || "$v" == "null" ]] && { printf '—'; return; }
    if [[ "$v" =~ ^-?[0-9]+([.][0-9]+)?([eE][-+]?[0-9]+)?$ ]]; then
      awk -v v="$v" 'BEGIN { printf "%.4g", v }'
    else
      printf '%s' "$v"
    fi
  }
  echo "| scenario | metric | default | custom-jvm | delta % |"
  echo "|----------|--------|---------|------------|---------|"
  tail -n +2 "$SUMMARY_CSV" | while IFS=, read -r sc m d c delta; do
    echo "| $sc | $m | $(fmt "$d") | $(fmt "$c") | ${delta:-—} |"
  done
  echo ""
  echo "Raw runs under: \`$RESULTS_ROOT\`"
} >"$SUMMARY_MD"

log "Wrote $SUMMARY_CSV and $SUMMARY_MD"
