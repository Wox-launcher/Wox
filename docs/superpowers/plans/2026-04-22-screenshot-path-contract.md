# Screenshot Path Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace screenshot result base64 transport with a path-based contract, persist exported screenshots under the Wox data directory, and bring Windows/Linux screenshot startup preparation closer to the optimized macOS flow.

**Architecture:** Keep Flutter as the owner of screenshot capture, composition, and export. Flutter writes the final PNG into the Wox screenshot directory and returns only metadata plus `screenshotPath` to Go. Go continues handling clipboard and plugin-facing completion semantics by reading the exported file from disk. Windows and Linux runners split snapshot metadata from expensive image encoding so screenshot startup stops blocking on full multi-display PNG serialization.

**Tech Stack:** Go backend (`wox.core`), Flutter/GetX UI, macOS Swift runner, Windows Win32 C++, Linux GTK/C++

---

### Task 1: Change the screenshot result contract to return `screenshotPath`

**Files:**
- Modify: `wox.core/common/ui.go`
- Modify: `wox.core/plugin/system/screenshot.go`
- Modify: `wox.ui.flutter/wox/lib/entity/screenshot_session.dart`

- [ ] Replace `PngBase64` and `OutputHandled` with `ScreenshotPath` in the shared Go contract and keep the existing status/error fields untouched.
- [ ] Update the Flutter entity model so completed screenshot results serialize `screenshotPath` instead of embedding image bytes in websocket JSON.
- [ ] Update Go screenshot completion handling to read the file at `ScreenshotPath`, validate that it exists, and keep clipboard behavior in the backend.

### Task 2: Export screenshots to the Wox screenshot directory from Flutter

**Files:**
- Modify: `wox.ui.flutter/wox/lib/controllers/wox_screenshot_controller.dart`
- Modify: `wox.ui.flutter/wox/lib/utils/log.dart`

- [ ] Add a Flutter helper that resolves the Wox data directory consistently with the existing UI log path convention and writes screenshots into `screenshots/`.
- [ ] Name files with the sortable pattern `YYYYMMDD_HHMMSS_wox_snapshots.png`, only appending a numeric suffix when the base filename already exists.
- [ ] Remove the macOS-only native clipboard fast path from screenshot confirmation so every platform produces the same `screenshotPath` result.

### Task 3: Make Windows and Linux snapshot capture metadata-first

**Files:**
- Modify: `wox.ui.flutter/wox/windows/runner/flutter_window.h`
- Modify: `wox.ui.flutter/wox/windows/runner/flutter_window.cpp`
- Modify: `wox.ui.flutter/wox/linux/runner/my_application.h`
- Modify: `wox.ui.flutter/wox/linux/runner/my_application.cc`
- Modify: `wox.ui.flutter/wox/lib/utils/screenshot/screenshot_platform_bridge.dart`
- Modify: `wox.ui.flutter/wox/lib/controllers/wox_screenshot_controller.dart`

- [ ] Add `captureDisplayMetadata` and `loadDisplaySnapshots` runner methods on Windows and Linux so Flutter can request geometry first and image payloads later.
- [ ] Add runner-side snapshot caches: Windows stores captured monitor bitmaps until hydration, Linux stores X11 monitor pixbufs or one Wayland portal desktop pixbuf plus portal monitor metadata until hydration.
- [ ] Update the Flutter screenshot controller to use metadata-first startup for Windows and Linux, preserving the existing macOS deferred hydration path shape instead of introducing another controller-specific branch.
- [ ] Keep comments next to each changed optimization point explaining why the old eager encoding path was too slow and why the new cache/hydration split is used.

### Task 4: Update smoke coverage and verify builds

**Files:**
- Modify: `wox.ui.flutter/wox/integration_test/launcher_screenshot_smoke_test.dart`

- [ ] Change screenshot smoke tests to expect `screenshotPath`, verify the file exists, and assert the PNG content is non-empty by reading from disk instead of decoding websocket base64.
- [ ] Add coverage that the exported filename matches the new timestamped screenshot naming convention.
- [ ] Run Flutter screenshot integration tests that cover the updated contract.
- [ ] Run `make build` in `wox.core` after the Go-side contract change.
