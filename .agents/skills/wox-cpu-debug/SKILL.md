---
name: wox-cpu-debug
description: Measure CPU usage, attribute CPU hotspots, and optimize them without materially increasing Go or native memory in the real single-process Wox Go UI. Use for continuous-query CPU regressions, high idle or hidden CPU, plugin/query hot paths, renderer or native AppKit activity, background work that continues after hide, and comparisons of Go pprof data with macOS process and stack samples.
---

# Wox CPU Debug

## Goal

Run the real Wox debug build with `wox_automation`, warm the same process, replay deterministic queries, and measure query-active and hidden CPU independently. Use Go CPU profiles for Go ownership and macOS PID/native samples for whole-process behavior. Keep profilers in separate identical runs so one profiler does not distort another.

## Run the Debug Build

1. Stop every other Wox instance.
2. Start the real debug build under Delve:

```bash
cd /Users/qianlifeng/Projects/Wox/wox.core
WOX_AUTOMATION_INFO_FILE=/tmp/wox-cpu-automation.json /Users/qianlifeng/go/bin/dlv debug . --build-flags=-tags=sqlite_fts5,wox_automation
```

3. Run `continue`, wait for startup and plugin initialization, and record the debuggee PID.
4. Keep this PID alive and running normally for the complete comparison. Never sample Delve or `debugserver` instead of the debuggee.

Build the automation workload once before measuring so Go compilation is outside every CPU window:

```bash
cd /Users/qianlifeng/Projects/Wox/wox.core
go build -o /tmp/wox-cpu-workload ../.agents/skills/wox-cpu-debug/scripts/run-cpu-workload.go
```

## Warm the Process

Run a 15-second mixed-query block, hide the launcher, and wait 10 seconds:

```bash
/tmp/wox-cpu-workload -info /tmp/wox-cpu-automation.json -mode queries -duration 15s -seed 1
sleep 10
```

Warm-up absorbs startup, application indexing, font loading, icon decoding, and lazy caches. Restarting Wox after warm-up invalidates the comparison.

## Capture Continuous-Query Go CPU

Trigger Wox's built-in 30-second `runtime/pprof` action and keep deterministic queries running for that complete window:

```bash
/tmp/wox-cpu-workload \
  -info /tmp/wox-cpu-automation.json \
  -mode profile-queries \
  -seed 11 \
  -output /tmp/wox-cpu-queries.prof
```

Inspect both self time and cumulative ownership:

```bash
go tool pprof -top -nodecount=30 /tmp/wox-cpu-queries.prof
go tool pprof -top -cum -nodecount=30 /tmp/wox-cpu-queries.prof
```

Read the profile header's `Duration` and `Total samples` ratio as average effective core use during the 30-second window. Record the top flat and cumulative owners. Use `go tool pprof -list '<pattern>'` for source-level attribution only after a stable owner appears.

Repeat with `-query terminal` or another fixed query when mixed queries need attribution. Compare mixed and fixed runs from the same PID; do not compare a profile captured during startup with a warmed profile.

## Measure macOS Query CPU

Use a separate workload-only run for process CPU percentages:

```bash
ready=$(mktemp /tmp/wox-cpu-query.XXXXXX)
/tmp/wox-cpu-workload -info /tmp/wox-cpu-automation.json -mode queries -duration 35s -seed 11 -ready-file "$ready" &
workload_pid=$!
while [[ ! -s "$ready" ]]; do sleep 0.1; done
/Users/qianlifeng/Projects/Wox/.agents/skills/wox-cpu-debug/scripts/sample-wox-cpu-macos.sh --pid <PID> --samples 30 --interval 1
wait "$workload_pid"
```

Use `AverageCPUPercent`, `MedianCPUPercent`, and `MaxCPUPercent`. macOS `%CPU` uses one-core units and may exceed 100 when multiple cores are active.

When Go pprof does not explain process CPU, repeat the workload in a separate run and capture native stacks:

```bash
ready=$(mktemp /tmp/wox-cpu-query-native.XXXXXX)
/tmp/wox-cpu-workload -info /tmp/wox-cpu-automation.json -mode queries -duration 35s -seed 11 -ready-file "$ready" &
workload_pid=$!
while [[ ! -s "$ready" ]]; do sleep 0.1; done
/usr/bin/sample <PID> 30 -file /tmp/wox-cpu-queries-native.txt
wait "$workload_pid"
```

Do not run `sample` and Go CPU profiling together. Attribute CoreText, CoreGraphics, AppKit, image decoding, cgo, and platform event-loop stacks from the native capture.

## Measure Hidden CPU

Hide Wox first, signal readiness only after the hide completes, and sample the settled process:

```bash
ready=$(mktemp /tmp/wox-cpu-hidden.XXXXXX)
/tmp/wox-cpu-workload -info /tmp/wox-cpu-automation.json -mode hidden -duration 35s -ready-file "$ready" &
workload_pid=$!
while [[ ! -s "$ready" ]]; do sleep 0.1; done
sleep 2
/Users/qianlifeng/Projects/Wox/.agents/skills/wox-cpu-debug/scripts/sample-wox-cpu-macos.sh --pid <PID> --samples 30 --interval 1
wait "$workload_pid"
```

Capture hidden native stacks in another identical run when median CPU stays above sampler noise or periodic spikes recur:

```bash
ready=$(mktemp /tmp/wox-cpu-hidden-native.XXXXXX)
/tmp/wox-cpu-workload -info /tmp/wox-cpu-automation.json -mode hidden -duration 35s -ready-file "$ready" &
workload_pid=$!
while [[ ! -s "$ready" ]]; do sleep 0.1; done
sleep 2
/usr/bin/sample <PID> 30 -file /tmp/wox-cpu-hidden-native.txt
wait "$workload_pid"
```

Use `profile-hidden` only as a secondary Go-owner check:

```bash
/tmp/wox-cpu-workload -info /tmp/wox-cpu-automation.json -mode profile-hidden -output /tmp/wox-cpu-hidden.prof
```

The CPU-profile action must briefly show the launcher to activate itself, so `profile-hidden` contains one-time activation and hide work at the start. Treat a function as a hidden hotspot only when it also appears in the settled OS/native run or consumes enough repeated samples to dominate that one-time transition.

## Interpret Hotspots

Classify query and hidden phases separately:

- **Expected query work:** plugin query execution, fuzzy matching, file/application lookup, result adaptation, layout, text measurement, image decode, and rendering that stops after hide.
- **Query hotspot candidate:** one owner repeatedly dominates flat or cumulative samples across identical profiles and scales with query frequency.
- **No hidden CPU signal:** median settles at sampler noise, occasional spikes do not repeat, and a 30-second native sample has no recurring application stack.
- **Possible hidden regression:** CPU remains above noise across three settled windows or recurring stacks show timers, animation/frame scheduling, renderer submission, Glance refresh, query cleanup, indexing, GC pressure, or asynchronous reloads after hide.
- **Strong hidden regression:** the same owner persists in Go and/or native samples, reproduces after another warm run, and disappears when its feature is disabled or its lifecycle ends.

If process CPU is high but Go pprof is quiet, investigate native or cgo owners before changing Go query code. If Go pprof is high but process samples are low, verify that the profile was captured from the same PID and that activation/transition work did not dominate a mostly idle window.

Do not edit production code until the workload and owner reproduce. Add targeted `util.GetLogger()` diagnostics only when profiles identify a narrow lifecycle or scheduling path but cannot prove why it remains active.

## Protect Memory While Optimizing CPU

Treat memory as a required guardrail for every CPU optimization:

- Prefer eliminating duplicate work, coalescing invalidations, stopping unnecessary scheduling, and reusing already-owned bounded resources.
- Do not trade CPU for unbounded caches, duplicate decoded images or pixel buffers, retained render surfaces, longer-lived queues, or state that survives hide/close without a lifecycle reason.
- Give every new cache, pool, or buffer a hard bound and an explicit release path. Clear window-scoped native resources on hide or close unless retaining them is required and measured.
- For changes that alter caching, buffering, pooling, object lifetime, image ownership, or native resources, compare warmed before/after process physical footprint and Go retained heap under the same workload. Use `wox-memory-debug` for repeated-cycle attribution when the delta is unclear.
- Treat a repeatable increase above 10 MiB or 5% of the warmed process footprint as material. Rework or reject the optimization unless the user explicitly accepts the tradeoff.

Do not conclude that memory is unchanged from Go heap data alone. CoreGraphics, AppKit, IOSurface, decoded images, cgo allocations, and plugin hosts can increase process memory without appearing in the Go heap.

## Report

Include:

- OS, CPU architecture, debug tags, PID, warm-up, query pool or fixed query, and sample duration.
- Query average/median/max process CPU and Go profile total samples versus duration.
- Query top flat and cumulative Go owners, plus native owners when captured.
- Hidden average/median/max process CPU and recurring Go/native stacks.
- Before/after process footprint and Go retained-heap deltas when the optimization can affect memory ownership, including cache bounds and hide/close release behavior.
- Whether activation, compilation, indexing, or another limitation contaminated a window.
- Classification for query CPU and hidden CPU, with the exact evidence supporting each.
