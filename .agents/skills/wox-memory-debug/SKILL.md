---
name: wox-memory-debug
description: Diagnose memory leaks in the current single-process Wox Go UI by launching the real debug build with its automation endpoint enabled, replaying representative launcher searches, sampling the same process across repeated workload blocks, and comparing Go heap profiles when retained memory keeps growing. Use for Wox memory-leak checks, search-result retention, Go UI memory growth, or repeated-query memory regressions on Windows or macOS.
---

# Wox Memory Debug

## Goal

Run the real Go UI in debug mode with the `wox_automation` endpoint, exercise normal launcher searches through the semantics tree, and decide whether memory settles after warm-up or grows with repeated work. Treat Wox as one Go process and use post-warm-up growth rather than an absolute memory budget as the leak criterion.

## Run the Debug Build

1. Stop other Wox instances so sampling cannot select the wrong process.
2. Start the real Wox debug build under Delve with both production dependencies and the automation endpoint enabled:

```bash
cd /Users/qianlifeng/Projects/Wox/wox.core
WOX_AUTOMATION_INFO_FILE=/tmp/wox-memory-automation.json /Users/qianlifeng/go/bin/dlv debug . --build-flags=-tags=sqlite_fts5,wox_automation
```

3. At the Delve prompt, run `continue`.
4. Wait until startup and plugin initialization finish, then wait for `WOX_AUTOMATION_INFO_FILE` to be written.
5. Record the debuggee PID from the debugger. Always pass this PID to the sampler because debugger-built executables may have temporary names.

Keep the debugger running normally. Do not pause at breakpoints while collecting memory samples. This uses the real Wox process with only the automation server compiled in; do not launch the separate Go UI smoke-test runner.

## Establish a Warm Baseline

Use the bundled automation workload driver against the real launcher:

```bash
cd /Users/qianlifeng/Projects/Wox/wox.core
go run ../.agents/skills/wox-memory-debug/scripts/run-query-workload.go -info /tmp/wox-memory-automation.json -mode queries -count 20 -seed 1
```

1. Run two warm-up blocks before recording the baseline.
2. In each block, replay 20 queries drawn repeatedly from these safe categories:
   - Calculator, such as `1+1`.
   - System command lookup, such as `settings`.
   - Application or general search, such as `wox`.
   - File-oriented search, such as `readme`.
3. Use a different deterministic seed for each block so the workload is reproducible while still varying result types.
4. The driver waits for the query value, allows results to settle, records the visible result count and semantics generation, clears the query, and continues. It does not execute results or commands.
5. At the end of the block, the driver hides the launcher. Wait 10 seconds so every checkpoint uses the same idle state.

Take the warm baseline only after these blocks. Initial startup growth, lazy font loading, icon decoding, and cache creation are expected and are not leak evidence.

## Sample the Same Process

### macOS

```bash
/Users/qianlifeng/Projects/Wox/.agents/skills/wox-memory-debug/scripts/sample-wox-memory-macos.sh --pid <PID> --samples 3 --interval 2
```

Use `PhysicalFootprintMB`. Absolute debug memory is not the leak criterion.

### Windows

```powershell
powershell -ExecutionPolicy Bypass -File C:\dev\Wox\.agents\skills\wox-memory-debug\scripts\sample-wox-memory.ps1 -Pids <PID> -Samples 3 -IntervalSeconds 2
```

Use `PrivateWorkingSetMB`.

Do not compare macOS physical footprint with Windows private working set. Compare checkpoints from the same PID, OS, debug session, workload, and idle state.

## Run the Measured Workload

1. Record the warm baseline.
2. Run five measured blocks of 10 or 20 queries using distinct seeds and the same query pool.
3. After each block, clear the query, hide the launcher, wait 10 seconds, and sample the same PID three times.
4. Record the median of each three-sample checkpoint to reduce sampler noise.
5. Report cumulative query count, median memory, change from the warm baseline, and the observed shape of the series.

Use more blocks only when the trend is ambiguous. Keep the process alive for the whole run; restarting Wox invalidates the comparison.

## Decide Whether Memory Leaks

Interpret the post-warm-up series, not a single number:

- **No leak signal:** memory rises during warm-up and then plateaus, oscillates within a stable range, or drops after an idle checkpoint.
- **Possible leak:** the settled checkpoint median keeps increasing across at least three consecutive measured blocks and the increase is materially larger than sampler jitter.
- **Strong leak signal:** retained growth continues after another identical workload, scales with cumulative query count, and does not settle during a longer 30-60 second idle checkpoint.

Go may retain heap arenas after objects become unreachable, so a high or non-decreasing process footprint alone is not proof. Report the result as `no leak signal`, `possible leak`, or `strong leak signal`, together with the measurements that support it.

## Attribute Persistent Growth

Only profile after the repeated-query run shows a possible or strong leak signal.

1. Trigger the dev-only memory profiling action through the automation driver after warm-up:

```bash
go run ../.agents/skills/wox-memory-debug/scripts/run-query-workload.go -info /tmp/wox-memory-automation.json -mode profile
```
2. Copy the generated profile immediately because the next capture overwrites it:

```bash
cp ~/.wox/memory.prof /tmp/wox-memory-before.prof
```

On Windows, copy `%USERPROFILE%\.wox\memory.prof` to a distinct temporary file instead.

3. Repeat the measured workload, run the command again, and copy the second profile:

```bash
cp ~/.wox/memory.prof /tmp/wox-memory-after.prof
```

4. Compare retained Go heap growth from `wox.core`:

```bash
go tool pprof -top -base /tmp/wox-memory-before.prof /tmp/wox-memory-after.prof
```

If process memory grows but the Go heap delta stays small, inspect Go UI native owners next: GPU textures and image caches, decoded result icons, preview resources, platform window allocations, and query-result cleanup. On macOS, compare `vmmap <PID> -summary` checkpoints and pay particular attention to `IOAccelerator` and `IOSurface`. If repeated identical queries still grow, inspect lazy-image cache identity, per-draw Metal texture creation, and drawable-size churn before attributing growth to unique query strings.

Use `-query terminal` with query mode for an identical-query control only when the mixed workload needs further attribution.

## Report

Include:

- OS, debug configuration, PID, workload, and checkpoint timing.
- A checkpoint table with cumulative queries and median process memory.
- The trend classification and whether a longer confirmation block was needed.
- Go heap delta owners only when profiling was necessary.
- Any limitation that prevented consistent UI automation or reliable sampling.

Do not edit production code until the measurements identify a reproducible trend and a likely owner.
