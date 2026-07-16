---
name: wox-memory-debug
description: Use when diagnosing Wox hidden-state memory usage on Windows or macOS, the single Wox process exceeding its memory budget, Go heap growth, WebView or native cache retention, or whether hidden Wox stays near a small process-memory target.
---

# Wox Memory Debug

## Overview

Use this skill to debug hidden-state memory for the single Wox process that contains both core and the Go UI. Keep the first pass boring: measure the same number every time, prove the window is hidden, then attribute memory to Go heap or native/cache state.

## Baseline First

1. Confirm the target OS. This skill defines Windows and macOS workflows only. For Linux, stop and say Linux is not covered yet.
2. Confirm Wox is hidden before sampling:
   - The embedded Go UI should have posted `/on/hide` to `PostOnHide()` in `wox.core/ui/manager.go`.
   - Wait 10-30 seconds after hiding so close animations, preview cleanup, and GC pressure settle.
3. Run the OS sampler.

### Windows Sampler

```powershell
powershell -ExecutionPolicy Bypass -File C:\dev\Wox\.agents\skills\wox-memory-debug\scripts\sample-wox-memory.ps1 -Samples 3 -IntervalSeconds 10
```

Use the script's `TotalMB` as the baseline. The Windows v1 budget is `wox <= 200 MB`, using private working set, not RSS, commit size, or system memory percent.

### macOS Sampler

```bash
/Users/qianlifeng/Projects/Wox/.agents/skills/wox-memory-debug/scripts/sample-wox-memory-macos.sh --samples 3 --interval 10
```

Use the script's `TotalMB` as the baseline. macOS uses `vmmap -summary` `Physical footprint`, which is the closest shell-accessible match for Activity Monitor's Memory column and Wox's `processmemory.GetProcessMemoryBytes` darwin path. Do not compare macOS footprint directly with Windows private working set; compare macOS runs with macOS runs.

If debugger-launched process names do not match, pass explicit PIDs:

```bash
/Users/qianlifeng/Projects/Wox/.agents/skills/wox-memory-debug/scripts/sample-wox-memory-macos.sh --pid 1234 --samples 3 --interval 10
```

## Attribution Order

Use the first matching branch; do not optimize before attribution.

| Signal | Check next |
| --- | --- |
| Go heap is high | Use the dev-only `memory_profiling` system command, inspect `%USERPROFILE%\.wox\memory.prof`, then run `go tool pprof` from `wox.core`. |
| Process memory is high but Go heap is modest | Inspect native/cache owners: WebView sessions, GPU image caches, screenshot state, settings state, and platform allocations. |
| macOS total is high after WebView previews | Inspect WKWebView/WebContent helper processes separately only if the user asks to include WebKit child processes. |
| Total only grows after specific previews | Start from platform WebView sessions and file preview renderers before touching launcher layout. |
| Total only grows after queries | Check result icons/images, plugin result payloads, and query result cleanup before changing global caches. |

## Wox-Specific Anchors

- Current-process memory already exists in dev builds through the `wox_memory` Glance item in `wox.core/plugin/system/glance/glance.go`.
- Windows memory parity lives in `wox.core/util/processmemory/process_memory_windows.go`; it intentionally uses private working set to match Task Manager's default Memory column.
- macOS memory parity lives in `wox.core/util/processmemory/process_memory_darwin.go`; it intentionally uses process footprint to match Activity Monitor's Memory column more closely than RSS.
- Go heap snapshots come from `memory_profiling` in `wox.core/plugin/system/sys/sys.go`.
- Logs to check: `%USERPROFILE%\.wox\log\wox.log` and `%USERPROFILE%\.wox\log\ui.log` on Windows, or `~/.wox/log/wox.log` and `~/.wox/log/ui.log` on macOS.

## Common Mistakes

- Do not compare one run's private working set with another run's RSS or commit size.
- Do not compare macOS physical footprint directly with Windows private working set.
- Do not include plugin hosts or child runtimes in the 200 MB target unless the user explicitly asks.
- Do not include macOS WebKit helper processes in the app-process baseline unless the user explicitly asks.
- Do not profile while Wox is visible and call it a hidden-state baseline.
- Do not start with code edits. Produce baseline numbers and a likely owner first.
- Do not run broad build or smoke suites for this workflow unless the user asks.
