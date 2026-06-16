# Screenshot Native Clipboard Handoff Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move screenshot clipboard writing out of Go and into the Flutter plus platform-runner screenshot pipeline while keeping a durable exported PNG path under `woxDataDirectory`.

**Architecture:** Go allocates the export path and passes it into the screenshot request. Flutter renders and writes the final PNG to that path, then immediately asks the platform runner to write the image file to the clipboard. The result always returns `screenshotPath` on successful export and uses explicit warning fields for clipboard failures.

**Tech Stack:** Go, Flutter/Dart, macOS AppKit, Windows Win32/GDI+, Linux GTK/GDK

---

### Task 1: Update Screenshot Request And Result Contracts

**Files:**
- Modify: `wox.core/common/ui.go`
- Modify: `wox.ui.flutter/wox/lib/entity/screenshot_session.dart`

- [ ] Add `exportFilePath` to `CaptureScreenshotRequest` in both Go and Dart.
- [ ] Add `clipboardWriteSucceeded` and `clipboardWarningMessage` to `CaptureScreenshotResult` in both Go and Dart.
- [ ] Keep `status`, `screenshotPath`, and existing failure fields compatible with the current websocket contract.

### Task 2: Move Export Path Allocation Into Go

**Files:**
- Modify: `wox.core/plugin/system/screenshot.go`

- [ ] Add a helper that allocates screenshot export files under `path.Join(util.GetLocation().GetWoxDataDirectory(), "screenshots")`.
- [ ] Populate `request.ExportFilePath` before invoking `CaptureScreenshot`.
- [ ] Remove Go-side screenshot clipboard write logic and decode path.
- [ ] Treat `completed` plus clipboard warning as a successful screenshot with an extra warning notification.

### Task 3: Make Flutter Use The Provided Export Path

**Files:**
- Modify: `wox.ui.flutter/wox/lib/controllers/wox_screenshot_controller.dart`

- [ ] Remove Flutter-owned screenshot directory allocation from the controller.
- [ ] Write the rendered PNG to `request.exportFilePath`.
- [ ] After export succeeds, call the platform bridge to write the exported image file to the clipboard when `output == clipboard`.
- [ ] Return `completed` with warning fields populated when clipboard write fails after export.

### Task 4: Replace RGBA Clipboard Bridge With File-Based Clipboard Export

**Files:**
- Modify: `wox.ui.flutter/wox/lib/utils/screenshot/screenshot_platform_bridge.dart`

- [ ] Replace `writeClipboardImageRgbaFile(...)` with `writeClipboardImageFile(filePath)`.
- [ ] Keep method-channel transport small by passing only the exported file path.
- [ ] Preserve missing-plugin fallback behavior as an explicit unsupported clipboard export failure.

### Task 5: Add macOS Screenshot Clipboard Export From File

**Files:**
- Modify: `wox.ui.flutter/wox/macos/Runner/AppDelegate.swift`

- [ ] Replace the screenshot-specific RGBA clipboard method with a file-based clipboard export method.
- [ ] Load the exported file through AppKit, write it to `NSPasteboard`, and return detailed clipboard errors through the existing `DisplayCaptureError` path.

### Task 6: Add Windows Screenshot Clipboard Export From File

**Files:**
- Modify: `wox.ui.flutter/wox/windows/runner/flutter_window.cpp`

- [ ] Add a method-channel branch for screenshot clipboard export from a file path.
- [ ] Load the exported PNG in the runner, convert it into clipboard-compatible native formats, and publish them through Win32 clipboard APIs.
- [ ] Return method-channel errors when file loading or clipboard writes fail.

### Task 7: Add Linux Screenshot Clipboard Export From File

**Files:**
- Modify: `wox.ui.flutter/wox/linux/runner/my_application.cc`

- [ ] Add a method-channel branch for screenshot clipboard export from a file path.
- [ ] Load the exported PNG into a `GdkPixbuf` and publish it to the GTK clipboard.
- [ ] Return method-channel errors when file loading or clipboard writes fail.

### Task 8: Build Verification

**Files:**
- None

- [ ] Run `make build` in `wox.core`.
- [ ] Report any platform-specific build limitations if local toolchains prevent full verification.
