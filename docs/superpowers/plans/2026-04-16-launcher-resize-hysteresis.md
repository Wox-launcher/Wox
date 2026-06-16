# Launcher Resize Hysteresis Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep launcher window height responsive to result-count changes without input-time flicker or long-lived stale results.

**Architecture:** Split launcher behavior into two independent layers. The result presentation layer keeps current visible rows stable across short query transitions and only swaps to placeholder or final empty state when appropriate. The window management layer tracks desired height separately from committed height so grow operations stay immediate while shrink operations use a short cancellable settle window.

**Tech Stack:** Flutter, GetX, integration_test smoke tests, Windows desktop window manager bridge.

---

### Task 1: Lock the expected behavior in smoke tests

**Files:**
- Modify: `wox.ui.flutter/wox/integration_test/launcher_resize_smoke_test.dart`
- Modify: `wox.ui.flutter/wox/integration_test/smoke_test_helper.dart`

- [ ] **Step 1: Add smoke coverage for stale-result grace and deferred shrink**

Add tests that model three cases:

```dart
testWidgets('T7-04: query change keeps visible height until shrink settle window expires', ...)
testWidgets('T7-05: non-final empty update does not clear visible results', ...)
testWidgets('T7-06: final empty update clears results and shrinks after settle', ...)
```

- [ ] **Step 2: Run the targeted smoke test file to verify current behavior fails**

Run: `flutter test integration_test/launcher_resize_smoke_test.dart`
Expected: at least one new test fails because the launcher currently clears or resizes too early.

- [ ] **Step 3: Commit the failing test shape mentally and do not adjust assertions to match current behavior**

Keep assertions centered on:
- non-final empty updates must not clear visible results
- shrink must be deferred
- final empty must still clear and shrink

### Task 2: Add result-display transition state to the launcher controller

**Files:**
- Modify: `wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart`

- [ ] **Step 1: Introduce explicit transient-result state**

Add controller state for:

```dart
final isShowingResultPlaceholder = false.obs;
String? pendingVisibleQueryId;
Timer staleResultTimer = Timer(const Duration(), () {});
static const Duration staleResultGrace = Duration(milliseconds: 80);
```

Also add helpers to:
- classify whether the current visible snapshot is stale
- decide when to switch from stale visible results to placeholder
- ignore non-final empty snapshots for the active query

- [ ] **Step 2: Route query-change handling through the transient-result state instead of immediate clear**

Update `onQueryChanged(...)` so that visible results are not cleared immediately when the window is visible. Instead:
- keep current visible items during `staleResultGrace`
- keep query-box loading indicator active
- mark the current visible snapshot as stale for the new query
- arm a timer that switches to placeholder only if no replacement snapshot arrives in time

- [ ] **Step 3: Update result-application logic to use `isFinal`**

Change `onReceivedQueryResults(...)` to accept `isFinal`, then enforce:

```dart
if (receivedResults.isEmpty && !isFinal) {
  return;
}
if (receivedResults.isNotEmpty) {
  // replace visible snapshot atomically
}
if (receivedResults.isEmpty && isFinal) {
  // clear visible snapshot
}
```

- [ ] **Step 4: Run the targeted smoke test file and confirm at least the stale-result assertions now pass**

Run: `flutter test integration_test/launcher_resize_smoke_test.dart`
Expected: failures move from stale-result behavior toward resize timing until Task 3 is complete.

### Task 3: Add window shrink hysteresis without blocking grow

**Files:**
- Modify: `wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart`

- [ ] **Step 1: Introduce committed vs desired resize state**

Add fields:

```dart
double? committedWindowHeight;
double? pendingShrinkHeight;
Timer windowShrinkTimer = Timer(const Duration(), () {});
static const Duration windowShrinkSettle = Duration(milliseconds: 96);
```

Add a coordinator helper:

```dart
Future<void> requestResize({
  required String traceId,
  required String reason,
  required double targetHeight,
})
```

- [ ] **Step 2: Make grow immediate and shrink cancellable**

`requestResize(...)` should:
- cancel pending shrink if a larger target arrives
- apply larger target on the next call immediately
- defer smaller target behind `windowShrinkSettle`
- commit final empty shrink immediately only when `isFinal` state allows it

- [ ] **Step 3: Redirect existing height-change call sites through the coordinator**

Replace direct `resizeHeight(...)` paths in result updates, clear-result flows, and other query-result-triggered paths with the coordinator so:
- result-driven grow is immediate
- result-driven shrink is delayed
- non-query UI paths such as multiline query box growth still stay immediate

- [ ] **Step 4: Keep height calculation data-driven**

Do not freeze the launcher at a fixed expanded height. Keep `calculateWindowHeight(...)` based on current result count, but only commit smaller values through the coordinator’s delayed path.

- [ ] **Step 5: Run the targeted smoke test file to verify green**

Run: `flutter test integration_test/launcher_resize_smoke_test.dart`
Expected: all resize smoke tests pass.

### Task 4: Reduce pointless redraw work and verify end-to-end

**Files:**
- Modify: `wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart`
- Optional modify: `wox.ui.flutter/wox/windows/runner/flutter_window.cpp`
- Verify: `wox.core/Makefile`

- [ ] **Step 1: Skip no-op visible snapshot replacements where practical**

Before replacing visible items, compare the new active-query snapshot with the currently visible snapshot. If they are visually equivalent, avoid redundant list updates and resize requests.

- [ ] **Step 2: Only touch the Windows runner if Flutter-side coordination still leaves obvious redraw churn**

If needed, narrow native forced redraw behavior for routine resize paths. Keep this optional unless smoke or manual verification shows Flutter-side fixes are insufficient.

- [ ] **Step 3: Run project verification commands**

Run:
- `flutter test integration_test/launcher_resize_smoke_test.dart`
- `make build` in `wox.core`

Expected:
- smoke tests pass
- core build exits with code 0

- [ ] **Step 4: Review changed files and prepare a focused commit**

Stage only:
- launcher controller changes
- smoke test changes
- any minimal native/window bridge updates required by verification
