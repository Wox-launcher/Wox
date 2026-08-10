#!/usr/bin/env bash
set -euo pipefail

pid=""
samples=30
interval=1

usage() {
  cat <<'EOF'
Usage: sample-wox-cpu-macos.sh --pid PID [options]

Options:
  --pid PID       Wox debuggee process id.
  --samples N     Number of samples after the discarded top warm-up row. Default: 30
  --interval N    Integer seconds between samples. Default: 1
  --help          Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --pid)
      pid="$2"
      shift 2
      ;;
    --samples|-s)
      samples="$2"
      shift 2
      ;;
    --interval|-i)
      interval="$2"
      shift 2
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
  echo "sample-wox-cpu-macos.sh only supports macOS." >&2
  exit 1
fi
if ! [[ "$pid" =~ ^[0-9]+$ ]] || ! kill -0 "$pid" 2>/dev/null; then
  echo "--pid must identify a running process." >&2
  exit 2
fi
if ! [[ "$samples" =~ ^[0-9]+$ ]] || [[ "$samples" -lt 1 ]]; then
  echo "--samples must be at least 1." >&2
  exit 2
fi
if ! [[ "$interval" =~ ^[0-9]+$ ]] || [[ "$interval" == "0" ]]; then
  echo "--interval must be a positive integer." >&2
  exit 2
fi

top -l "$((samples + 1))" -s "$interval" -pid "$pid" -stats pid,cpu,command -ncols 200 | awk -v target="$pid" -v wanted="$samples" '
  BEGIN {
    print "Sample CPUPercent"
  }
  $1 == target && $2 ~ /^[0-9]+([.][0-9]+)?$/ {
    seen++
    if (seen == 1) next
    count++
    cpu = $2 + 0
    values[count] = cpu
    sum += cpu
    if (count == 1 || cpu > max) max = cpu
    printf "%d %.2f\n", count, cpu
  }
  END {
    if (count != wanted) {
      printf "Expected %d CPU samples, captured %d.\n", wanted, count > "/dev/stderr"
      exit 1
    }
    for (i = 2; i <= count; i++) {
      value = values[i]
      j = i - 1
      while (j >= 1 && values[j] > value) {
        values[j + 1] = values[j]
        j--
      }
      values[j + 1] = value
    }
    if (count % 2 == 1) median = values[(count + 1) / 2]
    else median = (values[count / 2] + values[count / 2 + 1]) / 2
    printf "Summary Samples=%d AverageCPUPercent=%.2f MedianCPUPercent=%.2f MaxCPUPercent=%.2f\n", count, sum / count, median, max
  }
'
