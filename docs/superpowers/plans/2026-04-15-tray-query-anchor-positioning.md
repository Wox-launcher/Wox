# Tray Query Anchor Positioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Windows tray queries position from a tray anchor computed by the backend but resolved to a final top-left in Flutter using the real target height.

**Architecture:** Extend the show payload with optional tray anchor metadata for tray-query flows. Keep backend monitor and anchor selection logic, then let Flutter compute the final position immediately before `setBounds` using its actual layout-derived height. Non-tray flows continue using their existing position handling.

**Tech Stack:** Go backend, Flutter/Dart desktop UI, existing Wox show payload entities

---

### Task 1: Carry tray anchor metadata through the show payload

**Files:**
- Modify: `wox.core/common/ui.go`
- Modify: `wox.core/ui/manager.go`
- Modify: `wox.ui.flutter/wox/lib/entity/wox_query.dart`

- [ ] Add an optional typed tray anchor field to the shared show payload model.
- [ ] Populate the tray anchor only for tray-query flows on Windows.
- [ ] Preserve existing explicit `WindowPosition` behavior for non-tray flows.

### Task 2: Compute tray-query top-left in Flutter from final target height

**Files:**
- Modify: `wox.ui.flutter/wox/lib/controllers/wox_launcher_controller.dart`

- [ ] Add a small helper that resolves the final `Offset` for tray-query windows from tray anchor data, target width, and target height.
- [ ] Update `showApp` to use that helper before `windowManager.setBounds`.
- [ ] Leave non-tray show flows unchanged.

### Task 3: Verify the original failure path

**Files:**
- No repo file changes required unless extra diagnostics are needed

- [ ] Run targeted checks against the tray-query preview-only scenario to confirm the payload and resize path now keep a consistent tray anchor.
- [ ] Run a backend build check for `wox.core`.
