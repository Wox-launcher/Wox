# Windows Native Resize Rendering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix Windows query-result-driven auto-resize so the launcher result area does not remain visually stale, clipped, or offset after the window height changes.

**Architecture:** Treat this as a Windows runner synchronization bug first. Instrument the native `WM_SIZE` path, prove whether `HandleTopLevelWindowProc(...)` short-circuits child-window resizing, then make child-window synchronization unconditional in the handled `WM_SIZE` path. Only add a deferred native reconciliation pass if the direct child-sync fix still leaves high-DPI stale frames.

**Tech Stack:** Flutter desktop for Windows, Win32 runner (`flutter_window.cpp` / `win32_window.cpp`), Dart integration smoke tests

---

### Task 1: Instrument the Windows resize path and prove the control-flow hypothesis

**Files:**
- Modify: `wox.ui.flutter/wox/windows/runner/flutter_window.h`
- Modify: `wox.ui.flutter/wox/windows/runner/flutter_window.cpp`

- [ ] **Step 1: Declare focused native helpers for geometry logging and child sync**

Add these private declarations to `wox.ui.flutter/wox/windows/runner/flutter_window.h`:

```cpp
  void SyncFlutterChildWindowToClientArea(HWND hwnd, const char* source, bool engine_handled);
  std::string RectToString(const RECT& rect) const;
  RECT GetWindowRectSafe(HWND hwnd) const;
```

The helper contract is:
- `SyncFlutterChildWindowToClientArea(...)` reads the final client rect from the root window and applies it to `child_window_`
- `RectToString(...)` formats native rects for logs
- `GetWindowRectSafe(...)` is a small convenience wrapper for logging the child/root bounds

- [ ] **Step 2: Add instrumentation around `setSize` and `setBounds`**

In `wox.ui.flutter/wox/windows/runner/flutter_window.cpp`, instrument the `setSize` and `setBounds` method-channel branches so each resize request logs:
- logical size requested from Dart
- physical size passed into `SetWindowPos(...)`
- root window rect after `SetWindowPos(...)`
- client rect after `SetWindowPos(...)`
- child rect after the call returns

Use a concise pattern like:

```cpp
RECT window_rect{};
RECT client_rect{};
GetWindowRect(hwnd, &window_rect);
GetClientRect(hwnd, &client_rect);
RECT child_rect = child_window_ ? GetWindowRectSafe(child_window_) : RECT{};

std::ostringstream oss;
oss << "setSize: logical=" << width << "x" << height
    << ", physical=" << scaledWidth << "x" << scaledHeight
    << ", root=" << RectToString(window_rect)
    << ", client=" << RectToString(client_rect)
    << ", child=" << RectToString(child_rect);
Log(oss.str());
```

Add `#include <sstream>` near the top of `flutter_window.cpp`.

- [ ] **Step 3: Log whether Flutter engine handles `WM_SIZE` before base dispatch**

In `FlutterWindow::MessageHandler(...)`, special-case `WM_SIZE` so the handled/unhandled branch is visible in logs before changing behavior:

```cpp
if (message == WM_SIZE) {
  std::optional<LRESULT> top_level_result;
  if (flutter_controller_) {
    top_level_result = flutter_controller_->HandleTopLevelWindowProc(hwnd, message, wparam, lparam);
  }

  std::ostringstream oss;
  oss << "WM_SIZE: engineHandled=" << (top_level_result.has_value() ? "true" : "false");
  Log(oss.str());

  if (top_level_result) {
    return *top_level_result;
  }

  return Win32Window::MessageHandler(hwnd, message, wparam, lparam);
}
```

Keep this as instrumentation only for Task 1. Do **not** change child-sync behavior yet.

- [ ] **Step 4: Build the Windows runner and verify the code compiles**

Run on a Windows development machine:

```bash
cd /mnt/c/dev/Wox/wox.ui.flutter/wox
flutter build windows --debug
```

Expected:
- build completes successfully
- no new C++ compile errors from helper declarations or logging code

- [ ] **Step 5: Reproduce on the 175% DPI machine and capture the decisive log**

Manual repro sequence:
1. Launch Wox on Windows with display scale set to `175%`
2. Trigger a query that grows the result list height
3. Trigger a second query that shrinks the result list height
4. Trigger the failing case shown in the screenshots

Capture whether the logs show:
- `WM_SIZE: engineHandled=true` in the failing path
- root/client rect already updated
- child rect still reflecting the old size

This task is complete only when the logs either confirm or falsify the `WM_SIZE` short-circuit hypothesis.

### Task 2: Make child-window synchronization unconditional in the `WM_SIZE` path

**Files:**
- Modify: `wox.ui.flutter/wox/windows/runner/flutter_window.h`
- Modify: `wox.ui.flutter/wox/windows/runner/flutter_window.cpp`

- [ ] **Step 1: Implement `SyncFlutterChildWindowToClientArea(...)`**

In `wox.ui.flutter/wox/windows/runner/flutter_window.cpp`, add the helper implementation:

```cpp
void FlutterWindow::SyncFlutterChildWindowToClientArea(HWND hwnd, const char* source, bool engine_handled) {
  if (child_window_ == nullptr || !IsWindow(child_window_)) {
    return;
  }

  RECT client_rect{};
  GetClientRect(hwnd, &client_rect);
  const int width = client_rect.right - client_rect.left;
  const int height = client_rect.bottom - client_rect.top;

  MoveWindow(child_window_, client_rect.left, client_rect.top, width, height, TRUE);

  RECT child_rect = GetWindowRectSafe(child_window_);
  std::ostringstream oss;
  oss << source
      << ": engineHandled=" << (engine_handled ? "true" : "false")
      << ", client=" << RectToString(client_rect)
      << ", child=" << RectToString(child_rect);
  Log(oss.str());
}
```

Implementation rules:
- operate on `child_window_`, not a new HWND
- treat repeated `MoveWindow(...)` to the same rect as acceptable and idempotent
- keep the helper side-effect-free beyond child geometry sync and logging

- [ ] **Step 2: Update `FlutterWindow::MessageHandler(...)` so handled `WM_SIZE` still syncs the child window**

Replace the Task 1 temporary `WM_SIZE` branch with the primary fix:

```cpp
if (message == WM_SIZE) {
  std::optional<LRESULT> top_level_result;
  if (flutter_controller_) {
    top_level_result = flutter_controller_->HandleTopLevelWindowProc(hwnd, message, wparam, lparam);
  }

  SyncFlutterChildWindowToClientArea(hwnd, "WM_SIZE", top_level_result.has_value());

  if (top_level_result) {
    return *top_level_result;
  }

  return Win32Window::MessageHandler(hwnd, message, wparam, lparam);
}
```

Do **not** remove the existing `Win32Window::MessageHandler(...)` `WM_SIZE` branch in this task. Keep it as a fallback while validating the new path.

- [ ] **Step 3: Leave `SWP_FRAMECHANGED` and the Dart-side `forceDwmRecomposition` workaround unchanged**

Do **not** modify:
- `SWP_FRAMECHANGED` usage in `setSize` / `setBounds`
- the Dart `forceDwmRecomposition` / `+1px` workaround in `wox_launcher_controller.dart`

This task is intentionally scoped to the child-window sync hole only.

- [ ] **Step 4: Rebuild and rerun the exact manual repro matrix**

Run:

```bash
cd /mnt/c/dev/Wox/wox.ui.flutter/wox
flutter build windows --debug
```

Manual verification on Windows:
- `100%` DPI control case
- `175%` DPI repro case
- result count grows
- result count shrinks

Expected:
- every `WM_SIZE` path logs matching client and child geometry
- the stale/clipped result viewport no longer reproduces in the previously failing flows

### Task 3: Add smoke coverage for repeated grow/shrink resize transitions

**Files:**
- Create: `wox.ui.flutter/wox/integration_test/launcher_resize_smoke_test.dart`
- Modify: `wox.ui.flutter/wox/integration_test/launcher_smoke_test.dart`
- Modify: `wox.ui.flutter/wox/integration_test/smoke_test_helper.dart`

- [ ] **Step 1: Add a helper that waits for the native window height to match Flutter’s expected target height**

In `wox.ui.flutter/wox/integration_test/smoke_test_helper.dart`, add:

```dart
Future<void> waitForWindowHeightToMatchController(
  WidgetTester tester,
  WoxLauncherController controller, {
  double tolerance = 2,
  Duration timeout = const Duration(seconds: 10),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(const Duration(milliseconds: 200));
    final actual = await windowManager.getSize();
    final expected = controller.calculateWindowHeight();
    if ((actual.height - expected).abs() <= tolerance) {
      return;
    }
  }

  fail('Window height did not match controller.calculateWindowHeight() within $timeout.');
}
```

This helper does **not** prove the visual bug is gone. It only gives the smoke suite a stable way to assert that repeated native resizes are still being applied.

- [ ] **Step 2: Add a Windows-focused resize smoke test that exercises repeated grow/shrink cycles**

Create `wox.ui.flutter/wox/integration_test/launcher_resize_smoke_test.dart` with a test shaped like:

```dart
void registerLauncherResizeSmokeTests() {
  group('T7: Resize Smoke Tests', () {
    testWidgets('T7-01: repeated result grow/shrink cycles keep native window height in sync', (tester) async {
      if (!Platform.isWindows) {
        return;
      }

      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);

      for (var i = 0; i < 10; i++) {
        await queryAndWaitForResults(tester, controller, 'wox launcher test xyz123');
        await waitForWindowHeightToMatchController(tester, controller);
        final heightWithResults = (await windowManager.getSize()).height;

        tester.testTextInput.enterText('');
        await tester.pump();
        await waitForNoActiveResults(tester, controller);
        await waitForWindowHeightToMatchController(tester, controller);
        final heightWithoutResults = (await windowManager.getSize()).height;

        expect(heightWithResults, greaterThan(heightWithoutResults));
      }
    });
  });
}
```

This smoke covers:
- repeated grow/shrink transitions
- Windows-only path
- native window size synchronization through the existing window manager channel

- [ ] **Step 3: Register the new smoke file in the launcher smoke aggregator**

Update `wox.ui.flutter/wox/integration_test/launcher_smoke_test.dart`:

```dart
import 'launcher_resize_smoke_test.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  registerLauncherStartupSmokeTests();
  registerLauncherCoreSmokeTests();
  registerLauncherKeyFunctionalitySmokeTests();
  registerLauncherPluginSmokeTests();
  registerSystemPluginSmokeTests();
  registerLauncherToolbarMsgSmokeTests();
  registerLauncherResizeSmokeTests();
}
```

- [ ] **Step 4: Run the targeted integration smoke on Windows**

Run:

```bash
cd /mnt/c/dev/Wox/wox.ui.flutter/wox
flutter test integration_test/launcher_smoke_test.dart -d windows
```

Expected:
- existing launcher smoke tests still pass
- the new resize smoke test passes for repeated grow/shrink cycles

### Task 4: Add deferred native reconciliation only if Task 2 fixes geometry but visual stale frames still remain

**Files:**
- Modify: `wox.ui.flutter/wox/windows/runner/flutter_window.h`
- Modify: `wox.ui.flutter/wox/windows/runner/flutter_window.cpp`

- [ ] **Step 1: Add a coalesced deferred-resize mechanism**

Add private state in `flutter_window.h`:

```cpp
  static constexpr UINT kDeferredResizeSyncMessage = WM_APP + 1;
  uint64_t resize_sync_generation_ = 0;
  uint64_t queued_resize_sync_generation_ = 0;
```

Post only the latest generation from the `WM_SIZE` path:

```cpp
queued_resize_sync_generation_ = ++resize_sync_generation_;
PostMessage(hwnd, kDeferredResizeSyncMessage, 0, static_cast<LPARAM>(queued_resize_sync_generation_));
```

- [ ] **Step 2: Reconcile child geometry and redraw without background erase**

Handle the deferred message in `FlutterWindow::MessageHandler(...)`:

```cpp
if (message == kDeferredResizeSyncMessage) {
  const auto generation = static_cast<uint64_t>(lparam);
  if (generation != queued_resize_sync_generation_) {
    return 0;
  }

  SyncFlutterChildWindowToClientArea(hwnd, "deferred-resize-sync", false);
  RedrawWindow(hwnd, nullptr, nullptr, RDW_INVALIDATE | RDW_UPDATENOW | RDW_ALLCHILDREN | RDW_NOERASE);

  if (flutter_controller_) {
    flutter_controller_->ForceRedraw();
  }

  return 0;
}
```

Do **not** use `RDW_ERASE`.

- [ ] **Step 3: Rebuild and rerun the Windows repro matrix**

Run:

```bash
cd /mnt/c/dev/Wox/wox.ui.flutter/wox
flutter build windows --debug
```

Manual verification:
- the original `175%` DPI repro no longer leaves a persistent stale result area
- no new white flash or transparent background erase appears during repeated resizes

Only keep Task 4 if Task 2 alone is insufficient.

### Task 5: Final verification and handoff

**Files:**
- No additional repo files required unless diagnostic logs need cleanup

- [ ] **Step 1: Reduce noisy reproduction-only logging before handoff**

Keep only logs that are useful for future native resize debugging. Remove or downgrade the rest so normal launcher usage does not spam the log channel.

- [ ] **Step 2: Run the repository checks required for this change**

Run:

```bash
cd /mnt/c/dev/Wox/wox.ui.flutter/wox
flutter analyze
```

Run:

```bash
cd /mnt/c/dev/Wox/wox.core
make build
```

Expected:
- Flutter analysis passes
- `wox.core` build remains green

- [ ] **Step 3: Complete the Windows validation matrix**

Manual checklist:
- `100%` DPI
- `175%` DPI
- list layout
- grid layout
- preview hidden
- preview visible
- toolbar hidden
- toolbar visible
- at least `50-100` consecutive query transitions without the stale result viewport reappearing

- [ ] **Step 4: Leave follow-up cleanup work out of this change**

Do **not** fold these into the fix commit:
- removing `SWP_FRAMECHANGED`
- removing Dart `forceDwmRecomposition`
- Flutter-side viewport remount experiments

If the native-first fix is successful, record those as a separate cleanup follow-up.
