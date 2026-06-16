# Launcher Fixed Query Height Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Change launcher query-time height behavior to two fixed modes so the window no longer resizes with result-count changes.

**Architecture:** Keep the existing Flutter and native resize pipeline, but collapse height calculation into a controller-owned `compact` vs `expanded` policy. Validate the behavior through launcher resize smoke tests that assert state-based height transitions instead of result-count-driven window growth and shrink.

**Tech Stack:** Flutter desktop, GetX controller state, Dart integration smoke tests

---

### Task 1: Capture the new two-state behavior in smoke tests

**Files:**
- Modify: `wox.ui.flutter/wox/integration_test/launcher_resize_smoke_test.dart`
- Use: `wox.ui.flutter/wox/integration_test/smoke_test_helper.dart`

- [ ] **Step 1: Rewrite the Windows resize smoke expectations around `compact` and `expanded` states**

Update `launcher_resize_smoke_test.dart` so the suite asserts:
- blank start page opens at compact height
- typing any non-empty query moves to expanded height
- changing result counts within a non-empty query does not change expanded height
- clearing the query on blank start page returns to compact height

- [ ] **Step 2: Add MRU-specific smoke coverage**

Add a test that sets `StartPage` to `MRU`, shows the launcher with an empty query, and verifies:
- the launcher uses expanded height immediately
- clearing a previously non-empty query keeps the launcher at expanded height

- [ ] **Step 3: Run the targeted resize smoke test to confirm it fails against current behavior**

Run:

```bash
cd /mnt/c/dev/Wox/wox.ui.flutter/wox
flutter test integration_test/launcher_resize_smoke_test.dart
```

Expected before implementation:
- at least one assertion fails because the current controller still derives height from result count and other dynamic UI factors

### Task 2: Simplify launcher height calculation to the approved two-state policy

**Files:**
- Modify: `wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart`

- [ ] **Step 1: Add explicit controller helpers for height mode and fixed target heights**

Introduce focused helpers in `WoxLauncherController` for:
- determining whether the launcher should use `expanded` height
- computing compact height
- computing expanded height

Keep naming aligned with existing controller style and avoid introducing a new public enum unless the implementation truly needs it.

- [ ] **Step 2: Replace dynamic result-count-driven logic in `calculateWindowHeight(...)`**

Update `calculateWindowHeight(...)` so it:
- returns compact height when the query is empty and start page is not `MRU`
- returns expanded height otherwise
- no longer derives native height from:
  - current result count
  - grid height
  - preview/action panel visibility
  - query box line count

- [ ] **Step 3: Remove or neutralize redundant height-preservation code paths**

Review the controller fields and methods tied to transient visible-height preservation, including:
- `pendingVisibleQueryWindowHeight`
- `pendingVisibleQueryWindowHeightQueryId`
- `visibleQueryHeightPreservationTimer`
- `preserveVisibleQueryWindowHeight(...)`
- related release logic in result handling

Delete or simplify them when they no longer affect behavior under the new fixed-height policy.

- [ ] **Step 4: Keep unrelated window lifecycle behavior unchanged**

Do not change:
- show/hide positioning behavior
- Windows DWM recomposition workaround
- query preservation across temporary queries
- preview/result width behavior

This task is only about native height target policy.

### Task 3: Verify the new behavior end to end

**Files:**
- Modify as needed: `wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart`
- Modify as needed: `wox.ui.flutter/wox/integration_test/launcher_resize_smoke_test.dart`

- [ ] **Step 1: Run the targeted resize smoke test again and make it pass**

Run:

```bash
cd /mnt/c/dev/Wox/wox.ui.flutter/wox
flutter test integration_test/launcher_resize_smoke_test.dart
```

Expected:
- all resize smoke tests pass with the new two-state height behavior

- [ ] **Step 2: Run an additional launcher smoke check that exercises ordinary query flow**

Run:

```bash
cd /mnt/c/dev/Wox/wox.ui.flutter/wox
flutter test integration_test/launcher_core_smoke_test.dart --plain-name "T2-16"
```

Expected:
- continue/fresh show behavior still passes after the height-policy change

- [ ] **Step 3: Run Flutter formatting on touched Dart files**

Run:

```bash
cd /mnt/c/dev/Wox/wox.ui.flutter/wox
dart format lib/controllers/wox_launcher_controller.dart integration_test/launcher_resize_smoke_test.dart
```

Expected:
- formatter succeeds with no syntax errors

- [ ] **Step 4: Run the required backend build verification from the repo instructions**

Run:

```bash
cd /mnt/c/dev/Wox/wox.core
make build
```

Expected:
- backend build succeeds, confirming the repo remains in a valid state
