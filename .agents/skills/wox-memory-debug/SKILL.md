---
name: wox-memory-debug
description: Diagnose memory leaks and reduce query-time and idle memory in the single-process Wox Go UI on Windows or macOS. Use the real automation-enabled debug build to measure searches and settings lifecycles, attribute Go and native memory, implement focused optimizations, and verify memory savings without responsiveness or behavior regressions.
---

# Wox Memory Debug

## Goal

This skill has two equally important goals:

1. Detect memory leaks through repeated searches and independent settings-window open/close cycles.
2. Reduce memory occupied during queries and while idle, including stable allocations that never leak.

Run the real Go UI with the `wox_automation` endpoint. Unless the user limits the task to diagnosis, continue from measurement to attribution, focused implementation, and before/after verification. A `no leak signal` result completes only the leak check; it is not a reason to skip footprint optimization. If measured owners offer no justified improvement, report the evidence and tradeoffs rather than inventing a saving.

Treat Wox as one Go process. Use post-warm-up growth as the leak criterion, and reproducible reductions in comparable query and idle measurements as the optimization criterion. Historical measurements are context, not universal memory budgets. Read `wox.core/ui/README.md`, especially its platform memory guidance and hidden-state ownership rules, before changing memory lifecycle behavior.

## Run the Debug Build

1. Stop other Wox instances so sampling cannot select the wrong process.
2. Start the real Wox debug build under Delve with both production dependencies and the automation endpoint enabled:

```bash
cd /Users/qianlifeng/Projects/Wox/wox.core
WOX_AUTOMATION_INFO_FILE=/tmp/wox-memory-automation.json /Users/qianlifeng/go/bin/dlv debug . --build-flags=-tags=sqlite_fts5,wox_automation
```

On Windows, set `$env:_NO_DEBUG_HEAP = '1'` before starting Delve so the Windows debug heap does not distort allocation and fragmentation behavior. Use the local checkout and tool paths. Keep Go/compiler flags, renderer, heap sampling, real user data, enabled plugins, display scale, and debugger mode consistent between runs. If heap sampling is disabled, enable it for both sides of a profiling comparison (for example, `GODEBUG=memprofilerate=524288`) and establish a fresh baseline.

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
5. At the end of the block, the driver hides the launcher. Wait 45 seconds on Windows or 10 seconds on macOS so every checkpoint uses the same idle state.

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

Do not compare macOS physical footprint with Windows private working set. Keep one PID alive throughout each workload series. Before/after builds require separate processes: warm each independently with the same workload and compare equivalent lifecycle states, not adjacent checkpoints across a restart.

Confirm the launcher stays hidden throughout idle sampling, using `window.state` with `instanceName: "primary"` and logs when necessary. A hotkey, settings window, or other interaction that changes the checkpoint state invalidates that checkpoint; record the interference and repeat it after a full idle wait.

## Measure Query and Idle Footprint

Collect these separately before optimization and replay the same measurements afterward:

| State | Measurement |
| --- | --- |
| Active queries | Sample the same PID throughout deterministic mixed-query blocks at a fixed cadence; report the median and observed maximum, query count, and cadence. These samples are separate from the hidden checkpoint at block end. |
| Settled visible results | Hold representative queries from each safe category until results settle, then take three samples before clearing or hiding. Keep result/preview state and display scale comparable. |
| Visible idle | Clear the query, leave the launcher visible without interaction for a fixed recorded interval, then take three samples. |
| Hidden idle | Clear and hide, wait 45 seconds on Windows or 10 seconds on macOS, then take three samples. Use an additional 30-60 seconds of idle when the result has not settled. |

The bundled query driver clears each query quickly and hides at block end. Use concurrent sampling for active-query measurements and a separate automation sequence to hold settled results or visible idle; a post-block sample cannot describe query-time memory. Label sampled maxima as observed maxima, since short allocation peaks can fall between samples. Keep sampler overhead and cadence identical before and after.

Record query latency, first-query latency after hidden cleanup, and CPU behavior alongside memory. Measure process footprint and Go live heap separately; on Windows also record private commit when evaluating page release so reduced residency is not mistaken for reduced ownership. Capture expensive profiles or native breakdowns outside latency measurements.

## Run the Measured Query Workload

1. Record the warm baseline.
2. Run five measured blocks of 10 or 20 queries using distinct seeds and the same query pool.
3. After each block, clear the query, hide the launcher, wait 45 seconds on Windows or 10 seconds on macOS, and sample the same PID three times.
4. Record the median of each three-sample checkpoint to reduce sampler noise.
5. Report cumulative query count, median memory, change from the warm baseline, and the observed shape of the series.

Use more blocks only when the trend is ambiguous. Keep the process alive for the whole run; restarting Wox invalidates the comparison.

## Test Settings Window Cleanup

Run this lifecycle check after the query workload. It opens settings through the real `Open Wox Settings` result, waits for the independent settings semantics host, closes the settings window, waits until the launcher host owns automation again, and clears the opening query:

```bash
cd /Users/qianlifeng/Projects/Wox/wox.core
go run ../.agents/skills/wox-memory-debug/scripts/run-query-workload.go -info /tmp/wox-memory-automation.json -mode settings -count 1
```

1. Run one settings cycle as warm-up. Wait 45 seconds on Windows or 10 seconds on macOS and record the closed-settings baseline.
2. Run at least three measured blocks of five open/close cycles:

```bash
go run ../.agents/skills/wox-memory-debug/scripts/run-query-workload.go -info /tmp/wox-memory-automation.json -mode settings -count 5
```

3. After each block, keep both launcher and settings closed, wait 45 seconds on Windows or 10 seconds on macOS, and sample the same PID three times. Record the median.
4. Confirm every cycle reports both an `opened_generation` and `closed_generation`. The driver considers close complete only after `settings-search-field` disappears and `launcher.query.input` returns, which means `settingsView` and `settingsHost` have left the active automation surface.
5. Compare measured block medians with the post-warm-up closed-settings baseline. Repeat one identical five-cycle block after a 30-second idle checkpoint when the trend is ambiguous.

The first settings open may retain shared fonts, icons, and reusable renderer caches. Do not require the process to return to its pre-first-open value. Window-scoped settings state and native resources must not accumulate across later cycles: after warm-up, closed-settings checkpoints should plateau within sampler jitter instead of growing with cumulative open/close count.

## macOS Debug Reference

Use the August 10, 2026 macOS arm64 run as a comparison point for the current settings cleanup behavior, not as an absolute budget. The run used macOS 26.5.2, Go 1.26.2, Delve 1.26.2, the `sqlite_fts5,wox_automation` build tags, real Wox data, one PID, a 10-second hidden/closed wait, and three `PhysicalFootprintMB` samples at two-second intervals.

The query baseline was recorded after two 20-query warm-up blocks. The settings baseline was recorded after one complete warm-up open/close cycle:

| Checkpoint | Median PhysicalFootprintMB | Change from closed-settings baseline |
| --- | ---: | ---: |
| Hidden after 40 warm-up queries | 89.1 MB | N/A |
| Settings closed warm baseline | 99.5 MB | 0.0 MB |
| 5 measured settings cycles | 100.9 MB | +1.4 MB |
| 10 measured settings cycles | 101.4 MB | +1.9 MB |
| 15 measured settings cycles | 101.7 MB | +2.2 MB |
| 30-second idle confirmation | 102.3 MB | +2.8 MB |

Classify this series as `no leak signal`: the closed-settings checkpoints remained around 100-102 MB, and the 30-second confirmation stayed within observed sampler and allocator variation. `vmmap` reported no resident IOSurface memory after close; CoreGraphics and IOAccelerator residency was negligible. A Go heap comparison captured during the investigation showed only about 15 KB of retained delta, which redirected attribution to native window ownership rather than Go settings state.

Preserve these cleanup contracts when investigating a regression:

- Keep at most one live `wox.settings` window, while still treating each close/reopen as a new native lifetime.
- Release settings-only plugin catalogs, search indexes, forms, and application candidates on close, and invalidate in-flight loads so they cannot refill released caches.
- Load the plugin store and application candidates only when their UI needs them.
- Release the macOS `NSWindow`, view, and delegate hierarchy synchronously on close. Do not defer them to the application-lifetime autorelease pool.

Before the native close fix, a separate same-shape debug run rose from 104.8 MB after the warm settings close to 112.8 MB after 15 measured cycles. Treat a return of that cumulative, window-count-scaled shape as a regression even when the absolute starting footprint differs.

## Windows Debug Reference

Use the September 22, 2026 Windows run as a comparison point for hidden idle memory, not as a budget. The run used a dev build with `sqlite_fts5,wox_automation`, real Wox data, two 20-query warm-up blocks, and three `PrivateWorkingSetMB` samples 45 seconds after hide, when both the 10-second and 30-second release stages had run:

| Checkpoint | Median PrivateWorkingSetMB | Live Go heap |
| --- | ---: | ---: |
| Staged trims plus prepared search text and pinyin release | 63.8 MB | 17.2 MB |
| Shared plugin logger and shared fsnotify watcher | 60.8 MB | 15.2 MB |
| Live-object reductions (`wox.core/ui/README.md`, "Hidden-state ownership rules") | 55.4 MB | 11.0 MB |

Hidden memory drops in two steps: about 2-4 MB at 10 seconds and about 40 MB at 30 seconds, so sampling before 45 seconds compares different lifecycle states. When attributing what remains, use Delve `goroutines -group userloc` for goroutine ownership and the `woxmemory go` / `woxmemory native` commands for the Go-versus-native split; the largest native owners are NT heap fragmentation from show/hide churn and renderer objects that survive hide.

## Decide Whether Memory Leaks

Interpret the post-warm-up series, not a single number:

- **No leak signal:** memory rises during warm-up and then plateaus, oscillates within a stable range, or drops after an idle checkpoint.
- **Possible leak:** the settled checkpoint median keeps increasing across at least three consecutive measured blocks and the increase is materially larger than sampler jitter.
- **Strong leak signal:** retained growth continues after another identical workload, scales with cumulative query or settings-cycle count, and does not settle during a longer 30-60 second idle checkpoint.

Apply the same classification independently to cumulative queries and cumulative settings cycles. A stable query series does not rule out a settings-window lifecycle leak.

Go may retain heap arenas after objects become unreachable, so a high or non-decreasing process footprint alone is not proof. Report the result as `no leak signal`, `possible leak`, or `strong leak signal`, together with the measurements that support it.

## Attribute Growth and Stable Footprint

Profile when attributing either persistent growth or reducible query/idle footprint. A stable process can still retain unnecessary data, caches, native resources, or allocation capacity. Inspect absolute retained owners as well as deltas; a small delta does not explain a large steady footprint.

For hidden Windows checkpoints, prefer the automation-only `perf.memory_snapshot` with an optional distinct `profilePath`. It captures Go statistics and native resident-page ownership without showing the launcher or forcing GC. Inspect `wox.core/ui/automation/server_enabled.go` for the current transport and result fields. Use equivalent natural cleanup and sampling settings for both captures. The launcher command below is another capture path, but showing it recreates renderer resources and is not a hidden-state measurement.

1. Trigger the Wox Memory plugin's `woxmemory profile ` command through the automation driver after warm-up:

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

If process memory grows but the Go heap delta stays small, inspect Go UI native owners next: GPU textures and image caches, decoded result icons, preview resources, platform window allocations, and query-result cleanup. For settings-only growth, inspect `settingsView`, `settingsHost`, settings editors/forms, theme wallpaper previews, cloud/model state, asynchronous reloads, and native window/renderer destruction. On macOS, compare `vmmap <PID> -summary` checkpoints and pay particular attention to `IOAccelerator` and `IOSurface`. If repeated identical queries still grow, inspect lazy-image cache identity, per-draw Metal texture creation, and drawable-size churn before attributing growth to unique query strings.

For stable footprint, inspect the largest absolute owners with `go tool pprof -top` and allocation call paths, then reconcile them with native memory. Prioritize duplicated query data, result/action closures, oversized image decoding, cache contents and lifetime, eager settings/plugin loads, SQLite caches, renderer resources, and thread ownership according to measurements. Distinguish live allocations from reusable capacity, native fragmentation, and file-backed resident pages; reducing one does not guarantee the same reduction in process footprint.

Use `-query terminal` with query mode for an identical-query control only when the mixed workload needs further attribution.

## Optimize and Verify

1. Establish reproducible query and idle baselines and identify a likely owner before editing production code. A reproducible footprint with attributable avoidable ownership is sufficient; a leak is not required.
2. Implement a focused change at the owning feature or shared capability. Prefer avoiding allocations and duplicate ownership, decoding at the required size, loading data on demand, bounding caches, and releasing resources at their actual lifecycle boundary. Preserve query results, previews, settings behavior, and intentional background features.
3. Format and run relevant focused checks, then rebuild and rerun the same warm-up, query/idle measurements, and leak workloads. Keep one PID per build's series. Compare memory in MB and percent, including active-query maxima, settled results, visible idle, and hidden idle; do not hide an increase in one state behind a decrease in another.
4. Check query latency, first use after idle, CPU, and visible rendering. Reject or revise changes whose memory savings introduce material responsiveness, CPU, or behavior regressions. When savings are near sampler variation, repeat matched baseline/candidate runs before claiming improvement.
5. Continue with the next measured owner while a meaningful, low-risk reduction remains. Finish with verified savings or an evidence-based explanation of the remaining ownership and why further changes are not justified within the task.

Do not lower the reported footprint by disabling ordinary functionality, changing the workload/data/renderer between runs, repeatedly forcing GC, or evicting the process working set with `EmptyWorkingSet` / `SetProcessWorkingSetSize(-1, -1)`. Normal lifecycle cleanup may return unused memory, but measurement-only trims do not count as an optimization. Report file-page residency and private commit separately when relevant, and retain the latency cost of rebuilding released caches in the comparison.

## Report

Include:

- OS, revision/build, debug configuration, PID for each run, workload, and checkpoint timing.
- Before/after query and idle footprint table, with active-query median/observed maximum, settled-result samples, visible idle, hidden idle, and absolute/percentage savings. If no change was justified, report the measured baseline and owners investigated instead of implying optimization was completed.
- A checkpoint table with cumulative queries and median process memory.
- A separate closed-settings checkpoint table with cumulative open/close cycles and median process memory.
- The trend classification and whether a longer confirmation block was needed.
- Go and native owners supporting the optimization or leak diagnosis, including stable retained owners when relevant.
- Changes made, validation results, query/first-after-idle latency and CPU tradeoffs, and remaining opportunities or blockers. State footprint optimization results separately from leak classification.
- Any limitation that prevented consistent UI automation or reliable sampling.

Do not stop at `no leak signal` when query and idle footprint optimization remains in scope.
