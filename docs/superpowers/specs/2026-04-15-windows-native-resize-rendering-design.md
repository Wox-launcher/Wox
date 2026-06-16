# Windows Native Resize Rendering Design

## Problem
On Windows, query-result-driven auto-resize can leave the launcher result area visually stale after the window height changes. The top-level window and toolbar move to the new bounds, but the result list can remain clipped, offset, or partially painted until a later resize happens.

The issue is most visible on high DPI displays. The reported repro machine uses `175%` scaling.

## Scope
This document covers **approach 1 only**: fix the problem from the Windows native runner side first.

Out of scope for this document:
- Flutter-side viewport remount or widget tree restructuring
- backend query timing changes
- general redesign of the Windows window manager channel

## Observed Facts
- Flutter schedules `resizeHeight(...)` after query results update, using a post-frame callback.
- On Windows, `setSize` and `setBounds` call `SetWindowPos(...)`, and only after that call returns do they call `flutter_controller_->ForceRedraw()`.
- `FlutterWindow::MessageHandler(...)` calls `flutter_controller_->HandleTopLevelWindowProc(...)` before delegating to `Win32Window::MessageHandler(...)`.
- `Win32Window::MessageHandler(...)` handles `WM_SIZE` by resizing the hosted child window with `MoveWindow(child_content_, ...)`.
- If `flutter_controller_->HandleTopLevelWindowProc(...)` returns a handled result for `WM_SIZE`, the function returns early and `Win32Window::MessageHandler(...)` never receives that `WM_SIZE`.
- The launcher window uses a custom transparent client area and DWM material effects. `WM_NCCALCSIZE` returns `0`, and the runner extends the frame into the client area.
- The failure is not only a background-material problem. In the reported screenshots, the toolbar already reflects the new window height while the result list still behaves like the old viewport.
- A later query-triggered height change usually restores correct rendering.

## Root Cause Hypothesis
The highest-priority native-side hypothesis is that **the root window is resized, but the hosted Flutter child window is not always resized along with it**.

More specifically:
- `SetWindowPos(...)` updates the top-level window size.
- `WM_SIZE` is intended to resize the hosted child window.
- However, the current dispatch order allows `flutter_controller_->HandleTopLevelWindowProc(...)` to short-circuit `WM_SIZE` before `Win32Window::MessageHandler(...)` runs.

If that happens in the failing path:
- the root window reaches the new size
- Flutter engine receives the size message
- but the hosted child HWND can remain at the old size because `MoveWindow(child_content_, ...)` is skipped

That would directly explain the observed symptom: outer window geometry and toolbar placement are already correct, while the result content still behaves like the old viewport.

This hypothesis is consistent with the current runner code. What is still unverified is whether `HandleTopLevelWindowProc(...)` actually returns a handled result for `WM_SIZE` in the failing case. That must be measured, not assumed.

## Secondary Hypothesis
Even when the child window is resized correctly, high DPI transparent-window composition may still leave a short-lived stale frame because Flutter rendering and DWM composition are asynchronous. That is a secondary hardening concern, not the first suspected root cause.

## Why This Fits The Evidence
- The toolbar position already matches the new window height, which means the root window resize itself is not the missing step.
- The stale region is concentrated in the result content area, which is exactly where a stale Flutter child viewport or stale child HWND size would show up.
- The problem becomes more visible at `175%` scaling, where logical-to-physical rounding increases sensitivity to any mismatch between root bounds, client rect, and child bounds.
- The code already contains a separate Windows-only workaround for DWM recomposition after showing the window. That strongly suggests this runner already has timing-sensitive interactions between DWM, Flutter, and transparent window composition.

## Proposed Native Fix
Keep the Flutter API contract unchanged and fix the native resize chain in two phases:
- first, guarantee that `WM_SIZE` always synchronizes the hosted Flutter child window, even if the engine handles the message first
- second, only if needed, add a deferred reconciliation pass for high DPI/DWM edge cases

## Design
### 1. Make child-window resize unconditional for `WM_SIZE`
Move child-window synchronization into `FlutterWindow::MessageHandler(...)` so it cannot be skipped when `HandleTopLevelWindowProc(...)` returns a handled result.

Responsibilities:
- let `HandleTopLevelWindowProc(...)` observe `WM_SIZE`
- independently read `GetClientRect(hwnd, ...)`
- explicitly `MoveWindow(child_window_, ...)` or equivalent against that final client rect before returning
- keep `Win32Window::MessageHandler(...)` behavior as a fallback, but do not rely on it as the only path

This is the primary fix because it addresses a concrete control-flow hole in the current runner.

### 2. Keep `SWP_FRAMECHANGED` in the first native fix
Do not remove `SWP_FRAMECHANGED` in the first attempt.

Reasoning:
- the window uses a custom transparent client area and DWM material effects
- current behavior may depend on that message forcing non-client recalculation and DWM recomposition
- removing it before the real resize bug is fixed would mix two variables and make regression analysis harder

Once the child-window sync issue is fixed and verified, `SWP_FRAMECHANGED` can be re-evaluated separately.

### 3. Add targeted diagnostics before changing redraw policy
Instrument the runner with concise logs for:
- requested logical width/height from Dart
- physical width/height passed to `SetWindowPos`
- whether `HandleTopLevelWindowProc(...)` returned a handled result for `WM_SIZE`
- client rect after the root window resize
- child window rect after child sync
- timing of `WM_SIZE`

These logs are needed to validate that:
- root window size is correct
- child window matches the client rect in both handled and unhandled `WM_SIZE` paths

### 4. Add deferred resize reconciliation only if stale rendering remains
If unconditional child-window resize fixes most cases but high DPI stale frames still remain, add a deferred post-resize sync message such as `WM_APP + N`.

Requirements for that deferred pass:
- coalesce multiple pending posts so only the last resize state is reconciled
- store a token or generation counter so stale deferred work is ignored
- re-read `GetClientRect(hwnd, ...)`
- re-apply child window bounds if needed
- invalidate without background erase
- call `flutter_controller_->ForceRedraw()` only after the reconciliation step

Recommended redraw policy:
- avoid `RDW_ERASE`
- prefer `RedrawWindow(..., nullptr, nullptr, RDW_INVALIDATE | RDW_UPDATENOW | RDW_ALLCHILDREN | RDW_NOERASE)` or equivalent non-erasing invalidation

This deferred step is a secondary hardening layer, not the first fix.

### 5. Keep the existing Dart-side DWM workaround during the native-first attempt
Do not remove the Dart-side `forceDwmRecomposition` and `+1px` workaround in the same change.

Reasoning:
- it addresses a known show-time DWM issue
- removing it while changing native resize behavior would combine unrelated variables
- if the native fix solves query-result-driven resize corruption, the Dart workaround can later be re-evaluated in a separate cleanup pass

## Files To Change
- `wox.ui.flutter/wox/windows/runner/flutter_window.cpp`
- `wox.ui.flutter/wox/windows/runner/flutter_window.h`
- `wox.ui.flutter/wox/windows/runner/win32_window.cpp`
- `wox.ui.flutter/wox/windows/runner/win32_window.h`

## Expected Result
After the native-first fix:
- query-result-driven auto-resize should no longer leave the result list clipped or visually stale
- high DPI resizing should produce one stable result after each resize request
- the existing transparent/DWM window appearance should remain unchanged

## Validation Plan
### Manual repro
- Use Windows with `100%` scaling as a control case.
- Use Windows with `175%` scaling.
- Reproduce result-count growth and shrink cycles with the launcher visible.
- Confirm that the result list remains aligned after each automatic height change.

### Targeted cases
- list layout
- grid layout
- result count increases and window grows taller
- result count decreases and window shrinks
- toolbar visible vs hidden
- preview hidden vs visible
- repeated fast query changes
- at least 50-100 consecutive query transitions without a stale result viewport

### Native verification
- Confirm logs show:
  - whether `HandleTopLevelWindowProc(...)` handled `WM_SIZE`
  - final root client rect equals the child window rect in both handled and unhandled paths
  - if deferred sync is enabled, only the last pending resize token executes
  - `ForceRedraw()` happens after child geometry is reconciled

## Risks
- If the engine already resizes the child HWND internally, an unconditional extra `MoveWindow(...)` must remain idempotent and not introduce redundant churn.
- Extra redraw or invalidation could introduce flicker if the deferred sync is too aggressive.
- A native-only fix may improve but not fully eliminate the issue if Flutter-side viewport retention also contributes.

## Fallback If Native-Only Fix Is Incomplete
If this design reduces but does not eliminate the issue, the next step should be a mixed fix:
- keep the native deferred sync
- add a small Flutter-side viewport reset or remount hook tied to result-area height changes

That fallback is intentionally not part of this document. This document is the native-first attempt.
