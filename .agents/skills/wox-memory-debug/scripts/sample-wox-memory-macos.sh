#!/usr/bin/env bash
set -euo pipefail

samples=1
interval_seconds=10
budget_mb=200
json=0
process_names=("wox" "wox-darwin-amd64" "wox-darwin-arm64")
pids=()

usage() {
  cat <<'EOF'
Usage: sample-wox-memory-macos.sh [options]

Options:
  --samples N       Number of samples to capture. Default: 1
  --interval N      Seconds between samples. Default: 10
  --budget N        Budget in MB shown in output. Default: 200
  --pid PID         Include an explicit process id. May be repeated.
  --process NAME    Include an additional process executable name. May be repeated.
  --json            Emit JSON instead of a text table.
  --help            Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --samples|-s)
      samples="$2"
      shift 2
      ;;
    --interval|-i)
      interval_seconds="$2"
      shift 2
      ;;
    --budget|-b)
      budget_mb="$2"
      shift 2
      ;;
    --pid|-p)
      pids+=("$2")
      shift 2
      ;;
    --process)
      process_names+=("$2")
      shift 2
      ;;
    --json)
      json=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "sample-wox-memory-macos.sh only supports macOS." >&2
  exit 1
fi

if ! [[ "$samples" =~ ^[0-9]+$ ]] || [[ "$samples" -lt 1 ]]; then
  echo "--samples must be at least 1." >&2
  exit 2
fi

if ! [[ "$interval_seconds" =~ ^[0-9]+$ ]]; then
  echo "--interval must be a non-negative integer." >&2
  exit 2
fi

contains() {
  local needle="$1"
  shift
  local value
  for value in "$@"; do
    [[ "$value" == "$needle" ]] && return 0
  done
  return 1
}

size_to_mb() {
  local value="$1"
  local number unit

  if [[ "$value" =~ ^([0-9]+(\.[0-9]+)?)([KMGTP]?)$ ]]; then
    number="${BASH_REMATCH[1]}"
    unit="${BASH_REMATCH[3]}"
  else
    echo "Unable to parse vmmap size: $value" >&2
    return 1
  fi

  awk -v number="$number" -v unit="$unit" '
    BEGIN {
      factor = 1
      if (unit == "K") factor = 1 / 1024
      else if (unit == "G") factor = 1024
      else if (unit == "T") factor = 1024 * 1024
      else if (unit == "P") factor = 1024 * 1024 * 1024
      printf "%.1f", number * factor
    }
  '
}

physical_footprint_mb() {
  local pid="$1"
  local footprint
  footprint="$(vmmap -summary "$pid" 2>/dev/null | awk '/^Physical footprint:[[:space:]]/ { print $3; exit }')"
  if [[ -z "$footprint" ]]; then
    echo "Unable to read Physical footprint for pid $pid. Try running from Terminal with enough permissions." >&2
    return 1
  fi
  size_to_mb "$footprint"
}

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf '%s' "$value"
}

collect_process_rows() {
  local pid comm command name name_match path_match memory_mb
  while read -r pid comm; do
    [[ -z "${pid:-}" || -z "${comm:-}" ]] && continue
    command="$(ps -p "$pid" -o command= 2>/dev/null || true)"

    if [[ "${#pids[@]}" -gt 0 ]]; then
      contains "$pid" "${pids[@]}" || continue
    else
      name="${comm##*/}"
      name_match=1
      path_match=1
      if contains "$name" "${process_names[@]}"; then
        name_match=0
      fi
      if [[ "$command" == *"/Wox/wox.core/"* ]]; then
        path_match=0
      fi
      if [[ "$name_match" -ne 0 && "$path_match" -ne 0 ]]; then
        continue
      fi
    fi

    name="${comm##*/}"
    if ! memory_mb="$(physical_footprint_mb "$pid")"; then
      continue
    fi

    printf '%s\t%s\t%s\t%s\t%s\n' "Wox" "$pid" "$name" "$memory_mb" "$comm"
  done < <(ps -axo pid=,comm=)
}

emit_text_sample() {
  local sample="$1"
  local total="$2"
  local rows="$3"
  local over_budget="False"
  awk -v total="$total" -v budget="$budget_mb" 'BEGIN { exit !(total > budget) }' && over_budget="True"

  printf 'Sample %s: TotalMB=%s BudgetMB=%s OverBudget=%s\n' "$sample" "$total" "$budget_mb" "$over_budget"
  printf '%-8s %-8s %-24s %-24s %s\n' "Role" "Pid" "Name" "PhysicalFootprintMB" "Path"
  printf '%s\n' "$rows" | awk -F '\t' '{ printf "%-8s %-8s %-24s %-24s %s\n", $1, $2, $3, $4, $5 }'
}

emit_json_sample() {
  local sample="$1"
  local total="$2"
  local rows="$3"
  local first=1
  local role pid name memory path
  local over_budget="false"
  awk -v total="$total" -v budget="$budget_mb" 'BEGIN { exit !(total > budget) }' && over_budget="true"

  printf '{"Sample":%s,"TotalMB":%s,"BudgetMB":%s,"OverBudget":%s,"Processes":[' "$sample" "$total" "$budget_mb" "$over_budget"
  while IFS=$'\t' read -r role pid name memory path; do
    [[ -z "${role:-}" ]] && continue
    if [[ "$first" -eq 0 ]]; then
      printf ','
    fi
    first=0
    printf '{"Role":"%s","Pid":%s,"Name":"%s","PhysicalFootprintMB":%s,"Path":"%s"}' \
      "$(json_escape "$role")" "$pid" "$(json_escape "$name")" "$memory" "$(json_escape "$path")"
  done <<< "$rows"
  printf ']}'
}

if [[ "$json" -eq 1 ]]; then
  printf '['
fi

for ((sample = 1; sample <= samples; sample++)); do
  rows="$(collect_process_rows)"
  if [[ -z "$rows" ]]; then
    echo "No Wox process found. Pass --pid for debugger-launched processes with temporary names." >&2
    exit 1
  fi

  total="$(printf '%s\n' "$rows" | awk -F '\t' '{ sum += $4 } END { printf "%.1f", sum }')"

  if [[ "$json" -eq 1 ]]; then
    if [[ "$sample" -gt 1 ]]; then
      printf ','
    fi
    emit_json_sample "$sample" "$total" "$rows"
  else
    emit_text_sample "$sample" "$total" "$rows"
  fi

  if [[ "$sample" -lt "$samples" ]]; then
    sleep "$interval_seconds"
  fi
done

if [[ "$json" -eq 1 ]]; then
  printf ']\n'
fi
