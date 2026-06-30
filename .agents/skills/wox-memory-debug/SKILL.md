---
name: wox-memory-debug
description: Use when diagnosing Wox hidden-state memory usage on Windows or macOS, Wox core plus wox-ui over-budget memory, Flutter heap or WebView retention, Go heap profiles, or whether hidden Wox should stay near a small process-memory target.
---

# Wox Memory Debug

## Overview

Use this skill to debug hidden-state memory for the two main Wox processes: Go core (`wox`) and Flutter UI (`wox-ui`). Keep the first pass boring: measure the same number every time, prove the window is hidden, then attribute memory to Go heap, Flutter heap, or native/cache state.

## Baseline First

1. Confirm the target OS. This skill defines Windows and macOS workflows only. For Linux, stop and say Linux is not covered yet.
2. Confirm Wox is hidden before sampling:
   - Flutter should have gone through `WoxLauncherController.hideApp()` in `wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart`.
   - Core should have received `/on/hide` and updated `PostOnHide()` in `wox.core/ui/manager.go`.
   - Wait 10-30 seconds after hiding so close animations, preview cleanup, and GC pressure settle.
3. Run the OS sampler.

### Windows Sampler

```powershell
powershell -ExecutionPolicy Bypass -File C:\dev\Wox\.agents\skills\wox-memory-debug\scripts\sample-wox-memory.ps1 -Samples 3 -IntervalSeconds 10
```

Use the script's `TotalMB` as the baseline. The Windows v1 budget is `wox + wox-ui <= 200 MB`, using private working set, not RSS, commit size, or system memory percent.

### macOS Sampler

```bash
/Users/qianlifeng/Projects/Wox/.agents/skills/wox-memory-debug/scripts/sample-wox-memory-macos.sh --samples 3 --interval 10
```

Use the script's `TotalMB` as the baseline. macOS uses `vmmap -summary` `Physical footprint`, which is the closest shell-accessible match for Activity Monitor's Memory column and Wox's `processmemory.GetProcessMemoryBytes` darwin path. Do not compare macOS footprint directly with Windows private working set; compare macOS runs with macOS runs.

If debugger-launched process names do not match, pass explicit PIDs:

```bash
/Users/qianlifeng/Projects/Wox/.agents/skills/wox-memory-debug/scripts/sample-wox-memory-macos.sh --pid 1234 --pid 5678 --samples 3 --interval 10
```

## Attribution Order

Use the first matching branch; do not optimize before attribution.

| Signal | Check next |
| --- | --- |
| Go core is high | Use the dev-only `memory_profiling` system command, inspect `%USERPROFILE%\.wox\memory.prof`, then run `go tool pprof` from `wox.core`. |
| Flutter UI is high and Dart heap is high | Attach Flutter DevTools or VM Service Memory view, compare Dart heap before/after hiding, then inspect retained controllers/widgets. |
| Flutter UI is high but Dart heap is modest | Inspect native/cache owners: WebView cached sessions, preview controllers, image cache, screenshot/session state, settings/onboarding state. |
| macOS total is high after WebView previews | Check whether memory is in `wox-ui` footprint first, then inspect WKWebView/WebContent helper processes separately if the user asks to include WebKit child processes. |
| Total only grows after specific previews | Start from `WoxWebViewUtil`, platform WebView sessions, and file preview renderers before touching launcher layout. |
| Total only grows after queries | Check result icons/images, plugin result payloads, and query result cleanup before changing global caches. |

## Wox-Specific Anchors

- Combined memory already exists in dev builds through the `wox_memory` Glance item in `wox.core/plugin/system/glance/glance.go`.
- UI PID registration is intentional: Flutter reports its PID from `WoxApi.onUIReady()` so core can attribute `wox-ui`.
- Windows memory parity lives in `wox.core/util/processmemory/process_memory_windows.go`; it intentionally uses private working set to match Task Manager's default Memory column.
- macOS memory parity lives in `wox.core/util/processmemory/process_memory_darwin.go`; it intentionally uses process footprint to match Activity Monitor's Memory column more closely than RSS.
- Go heap snapshots come from `memory_profiling` in `wox.core/plugin/system/sys/sys.go`.
- Logs to check: `%USERPROFILE%\.wox\log\wox.log` and `%USERPROFILE%\.wox\log\ui.log` on Windows, or `~/.wox/log/wox.log` and `~/.wox/log/ui.log` on macOS.

## Common Mistakes

- Do not compare one run's private working set with another run's RSS or commit size.
- Do not compare macOS physical footprint directly with Windows private working set.
- Do not include plugin hosts or child runtimes in the 200 MB target unless the user explicitly asks.
- Do not include macOS WebKit helper processes in the two-process baseline unless the user explicitly asks.
- Do not profile while Wox is visible and call it a hidden-state baseline.
- Do not start with code edits. Produce baseline numbers and a likely owner first.
- Do not run Flutter build or smoke tests for this workflow unless the user asks.
