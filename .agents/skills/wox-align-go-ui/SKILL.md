---
name: wox-align-go-ui
description: Align Wox Go UI screens and interactions with the Flutter UI reference by running the installed Flutter app and the feature-go-ui implementation serially, inspecting Flutter source on master, comparing controlled screenshots and behavior, implementing focused Go UI fixes, and validating parity. Use for Wox Flutter-to-Go UI migrations, visual parity work, interaction parity, Settings or launcher screen synchronization, and requests to make the Go UI match the existing Flutter product.
---

# Align Wox Go UI

Align one named screen, component, or user flow at a time. Treat the running Flutter app as the product reference, Flutter source on `master` as explanatory evidence, and Go UI source on `feature-go-ui` as the implementation target.

## Non-negotiable constraints

- Never run Flutter Wox and Go UI Wox at the same time. Wox is single-instance.
- Enforce the runtime sequence `stopped -> Flutter -> stopped -> Go UI -> stopped`.
- Verify the process is gone at each `stopped` boundary. Inspect full command paths with `ps -Ao pid=,command=` and ensure both a `go run` wrapper and its compiled child are gone. Do not assume closing a window ended Wox.
- Prefer graceful quit. Force-terminate only the exact verified Wox PID when graceful quit fails and the action is authorized.
- Never switch branches in a dirty checkout, stash user changes, discard changes, or overwrite unrelated work.
- Do not edit Flutter code unless the user explicitly requests it. The normal deliverable changes Go UI only.
- Do not claim parity from compilation or tests alone. Complete a serial runtime comparison after the change.

## Establish scope and repository state

1. Resolve the repository root, normally `/Users/qianlifeng/Projects/Wox`.
2. Read applicable `AGENTS.md`, relevant `README.md` files, and `Wox.code-workspace`.
3. Inspect:

   ```bash
   git status --short --branch
   git worktree list --porcelain
   git branch --list master feature-go-ui
   ```

4. Identify the smallest requested screen, component, state, or flow and its acceptance criteria. If the request is broad, start with one coherent surface and report the remaining surfaces instead of silently expanding scope.
5. Record the `master` and `feature-go-ui` commit IDs used for the comparison.
6. Preserve the user's current worktree. Use `git show master:<path>` and `git grep <pattern> master -- wox.ui.flutter/wox` for focused Flutter inspection. For broad source navigation, create a temporary detached read-only worktree from `master`; do not switch the implementation checkout away from `feature-go-ui`.
7. Implement only in a cleanly identified `feature-go-ui` checkout. If relevant target files already contain user changes, inspect and preserve them. Stop for direction only when the requested change cannot be separated safely.

## Define a reproducible comparison case

Before launching either UI, write down one comparison case containing:

- screen or route and exact navigation steps
- monitor, scale factor, window position, and logical window size
- light or dark theme, locale, text scale, and platform appearance
- account, plugin, settings, and data state that affect the surface
- query text, selection, scroll offset, expanded sections, hover or focus target
- expected loading, empty, populated, error, disabled, or validation state

Use the same case for both implementations. If shared Wox data changes during the Flutter run, restore the intended state before launching Go UI.

Create a temporary capture directory such as `/tmp/wox-ui-parity/<timestamp>/` with separate `flutter/` and `go/` subdirectories. Screenshots may be compared side by side or with an offline image diff after both application processes are stopped.

## Capture the Flutter reference

1. Confirm no Wox process is running.
2. Confirm the installed reference exists at `/Applications/Wox.app`. If only `/Application/Wox.app` was supplied, correct the path rather than creating a second assumption.
3. Load and follow the available Computer Use skill before any GUI operation. If GUI control is unavailable, ask for user-provided captures instead of claiming runtime comparison.
4. Launch `/Applications/Wox.app` with `/usr/bin/open /Applications/Wox.app`, then wait for the exact bundle process and window to become ready.
5. Reproduce the comparison case without changing unrelated settings.
6. Capture:

   - the full window at the agreed geometry
   - focused component crops when small differences matter
   - interaction states relevant to the request, including keyboard focus, hover, pressed, disabled, scrolling, validation, loading, empty, and error states
   - observable window behavior such as resizing, secondary-window ownership, close, reopen, and focus restoration

7. Record measured or directly observable facts rather than impressions: hierarchy, bounds, gaps, padding, alignment, typography, colors, borders, radii, shadows, icons, clipping, scroll behavior, focus order, shortcuts, and accessibility semantics.
8. Quit Flutter Wox gracefully, wait for exit, and verify that no Wox process remains before continuing.

The installed bundle is the visual and behavioral authority. If it disagrees with current `master`, report the drift and use source only to explain what can be confirmed.

## Trace the Flutter contract on master

Locate the relevant code under `wox.ui.flutter/wox` on `master`:

```bash
git grep -n '<visible text or symbol>' master -- wox.ui.flutter/wox
git grep -n '<widget, route, controller, or setting>' master -- wox.ui.flutter/wox
```

Trace the complete contract, not only the leaf widget:

- route and window orchestration
- widget hierarchy and layout constraints
- shared theme, spacing, typography, icons, and reusable components
- controller state, API calls, subscriptions, and asynchronous transitions
- keyboard, focus, hover, pointer, scroll, and accessibility behavior
- empty, loading, error, disabled, and long-content states

Keep a mapping from Flutter source and observed behavior to the corresponding Go UI owner. Do not copy Flutter code line for line; preserve the product contract in the appropriate Go UI layer.

## Capture and compare Go UI

1. Reconfirm the Flutter process is gone.
2. Launch Go UI from the `feature-go-ui` implementation checkout, using the repository launch contract. The normal terminal entry is:

   ```bash
   cd wox.core
   CGO_ENABLED=1 go run -tags sqlite_fts5 .
   ```

3. Reproduce the exact comparison case and capture the same window and component states.
4. Quit Go UI gracefully and verify the Wox process is gone.
5. Compare the saved artifacts only after the serial captures are complete.

Build a concise delta matrix with these categories:

| Category | Compare |
| --- | --- |
| Structure | window, route, hierarchy, sections, ordering |
| Geometry | size, position, constraints, padding, gaps, alignment |
| Styling | theme tokens, typography, colors, borders, radius, shadow, icons |
| State | initial, loading, empty, populated, error, disabled, long content |
| Interaction | click, hover, keyboard, focus, scrolling, shortcuts, resize |
| Semantics | accessibility role, label, focusability, activation |
| Lifecycle | show, hide, close, reopen, secondary windows, focus restoration |

Prioritize contract and interaction defects first, major layout differences second, and cosmetic polish last. Distinguish real defects from expected platform font rasterization or timing differences.

## Implement in the correct Go UI layer

- Follow the repository instructions and preserve existing semantics outside the scoped surface.
- Prefer shared Go UI widgets, theme tokens, layout primitives, and lifecycle abstractions when the Flutter contract is shared. Avoid page-specific constants or workarounds for a shared defect.
- Keep rendering pure: render prepared state and perform I/O, asynchronous loading, native window work, and WebView lifecycle work outside build/render callbacks.
- Keep text input, focus, IME, and caret ownership in retained text-field state. Investigate shared focus lifecycle before patching one page.
- Keep launcher result rows out of the keyboard focus chain while preserving pointer and accessibility activation.
- Preserve independent window instances, message synchronization, and complete resource teardown for true secondary-window behavior.
- Keep control flow straightforward. Add English intent comments only for non-obvious reasons, state transitions, or constraints. Add short comments for non-trivial new functions.
- Use `apply_patch` for edits and `gofmt` for changed Go files.

## Validate and repeat the runtime comparison

1. Run focused tests for the changed packages from `wox.core`. If the default cache is unavailable, use `GOCACHE=/tmp/wox-go-cache`.
2. Run `make test-go-ui-unit` when the change crosses shared widget, runtime, automation, or launcher boundaries.
3. Run native smoke only when the request or repository guidance authorizes it. Treat smoke assertions as evidence for their exact contract, not as proof of visual parity.
4. Repeat the same serial Flutter and Go UI capture case after implementation.
5. Verify window geometry, rapid input where relevant, focus semantics, interaction behavior, and visual output. A successful build is not runtime acceptance.
6. Leave at most one Wox implementation running. Prefer restoring the initial runtime state; otherwise leave both stopped and report it.

## Completion report

Report:

- aligned screen, flow, and states
- installed Flutter bundle identity when available, plus `master` and `feature-go-ui` commit IDs
- Flutter-to-Go source mapping and files changed
- capture directory and comparison conditions
- differences fixed and any intentional or unresolved differences
- formatting, focused tests, unit suite, smoke, and serial runtime validation actually completed
- final Wox process state

State limitations plainly. Do not say the interfaces are aligned if the post-change Go UI was not launched and compared against the saved Flutter reference.
