---
name: wox-multiple-window-merge
description: Merge or rebase Wox master into the Flutter multiple-window branch with conflict-resolution rules. Use when Codex is resolving Wox conflicts involving feature-flutter-multiple-windows, WoxAppRuntime, WoxMultipleWindow, WoxWindowDriver, macOS multiview, launcher/settings/onboarding/screenshot windows, or repeated master merge/rebase conflicts in /Users/qianlifeng/Projects/Wox.
---

# Wox Multiple Window Merge

## Goal

Resolve master updates into Wox's multiple-window branch with low churn:

- Prefer `master` behavior for normal feature code.
- Preserve multiple-window infrastructure and per-window routing.
- Avoid re-litigating the same conflict patterns across repeated merges.

## First Decision

Inspect the current Git operation before editing:

```bash
git status --short --branch
git status
git diff --name-only --diff-filter=U
find .git -maxdepth 2 \( -name rebase-merge -o -name rebase-apply -o -name MERGE_HEAD -o -name CHERRY_PICK_HEAD \) -print
```

Prefer `git merge master` for future master syncs into the multiple-window branch. This branch is architectural and touches shared UI/runtime seams, so rebase tends to replay conflicts commit-by-commit and repeat the same files many times.

If a rebase is already in progress, continue the rebase instead of switching strategies unless the user explicitly asks to abort. Explain that new conflicts after `git rebase --continue` usually come from later replayed commits, not from the previous batch being unresolved.

## Conflict Policy

Use this rule of thumb:

1. Take `master` implementation for product features, UI layout, settings forms, preview renderers, tooltip APIs, release notes, plugin installer behavior, and language/resource updates.
2. Keep multiple-window code when it controls instance identity, native window routing, per-window focus, or engine startup.
3. If both sides are needed, compose them instead of choosing one side.

Do not blindly prefer `HEAD` during rebase. In rebase conflict labels, `HEAD` is the already-replayed result on top of master, while the incoming side is the commit currently being replayed.

## Preserve These Multiple-Window Pieces

Keep these patterns unless master has an equivalent multiple-window implementation:

- `WoxAppRuntime.initializePrimary(...)`, `WoxAppRuntime.instance`, `primaryInstance`, and per-instance controllers.
- `WoxMultipleWindowHost`, `WoxMultipleWindow`, `WoxMultipleWindowStyle`, and multiple-window IDs.
- Explicit widget/controller injection such as `WoxLauncherView(controller: controller)`, `WoxQueryBoxView(controller: controller)`, `WoxPreviewView(..., launcherController: controller, aiChatController: controller.activeAIChatController)`.
- `WoxWindowDriver` and `controller.windowDriver` for `show`, `hide`, `focus`, `isVisible`, `setBounds`, `startDragging`, and native handle lookup.
- `sessionId` forwarding in websocket/http UI lifecycle calls: `onShow`, `onHide`, `onSetting`, `onOnboarding`, `onHotkeyRecording`, `onQueryBoxFocus`, `onInstanceDestroyed`.
- macOS multiview startup and AppKit bridge changes required for secondary Flutter windows.
- Secondary-window-safe WebView handling: use `launcherController.windowDriver.getNativeHandle()` and filter native WebView events by source window handle.
- Separate settings/onboarding/screenshot/window flows when the multiple-window branch moved them out of the inline main launcher view.

Avoid reintroducing global `Get.find<WoxLauncherController>()` or global `windowManager` in code that should act on the current window instance. Use the injected controller and `windowDriver`.

## Recurring Resolutions

### `main.dart`

Keep the multiple-window runtime and host:

- Keep `flutter_windowing.WindowManager`.
- Keep `WoxMultipleWindowHost`.
- Keep `WoxAppRuntime` initialization and websocket routing through runtime.
- Keep `WoxLauncherView(controller: launcherController)`.

If master adds inline settings/onboarding/screenshot branching in `WoxApp.build`, usually do not copy that into the primary launcher body if the multiple-window branch already opens those as separate windows.

### `wox_launcher_controller.dart`

Keep constructor and fields:

```dart
WoxLauncherController({required this.sessionId, required this.windowDriver, required this.isPrimaryInstance});
final String sessionId;
final WoxWindowDriver windowDriver;
final bool isPrimaryInstance;
```

When master adds behavior inside methods, keep the behavior but route window operations through `windowDriver`, not global `windowManager`.

### Preview Files

For `WoxPreviewView`, `WoxWebViewPreview`, terminal preview, AI chat preview, update preview, query-requirement preview, and trigger-keyword conflict preview:

- Keep explicit `launcherController` and `aiChatController` parameters.
- If master adds new constructor options, combine them with controller parameters. Example: keep both `launcherController` and `showToolbar`.
- For file-preview renderers that create a WebView, pass the current launcher controller through the preview context rather than using a global lookup.
- Use `launcherController.isPreviewOnlyLayout` for preview-only padding checks if that is the current branch API.

### Query Box Drag Areas

If master adds draggable blank areas or tap regions, keep the new interaction structure but set drag start explicitly:

```dart
WoxDragMoveArea(
  onDragStart: controller.windowDriver.startDragging,
  ...
)
```

This avoids dragging the wrong native window from a secondary instance.

### Wox API

When master adds new endpoints, keep them. Also preserve `sessionId` parameters on lifecycle endpoints.

If a later master commit evolves an API shape, let the later commit apply it. For example, tooltip overlay may first add `x/y` endpoints and later change to `side`; resolve each commit in the order Git presents it unless doing a merge commit.

### Settings Tables And Dialogs

Prefer master's settings form behavior, including custom dialog builders and dedicated query hotkey dialogs. Preserve only multiple-window-specific focus restoration or routing when present.

## Rebase Loop

When the user asks to finish an in-progress rebase:

1. Run `GIT_EDITOR=true git rebase --continue`.
2. On conflict, inspect only unmerged files first.
3. Resolve using this skill's policy.
4. Format touched Dart files with `dart format --line-length 180`.
5. Stage resolved files.
6. Run:

```bash
git diff --name-only --diff-filter=U
git diff --cached --check
```

7. Continue until rebase completes.

Run `flutter analyze --no-fatal-infos` after meaningful Flutter conflict batches or before claiming the merge is clean. Do not run Flutter build or smoke tests unless the user explicitly asks.

## Verification

Use the repo rules:

- Dart formatting: `dart format --line-length 180 <files>`.
- Flutter changes: `flutter analyze --no-fatal-infos` from `wox.ui.flutter/wox`.
- Go/backend changes: run focused Go build/tests when useful and low risk.
- macOS native changes: prefer `xcodebuild -project Runner.xcodeproj -scheme Runner -showBuildSettings` for lightweight syntax/project checks.
- Always check no conflict markers remain:

```bash
rg -n '<<<<<<<|=======|>>>>>>>' <resolved-files>
git diff --name-only --diff-filter=U
```

Never run smoke tests unless the user explicitly asks.
