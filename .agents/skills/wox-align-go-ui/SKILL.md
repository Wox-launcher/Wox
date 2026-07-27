---
name: wox-align-go-ui
description: Align Wox Go UI screens and interactions with the Flutter UI reference by running the installed Flutter app and the feature-go-ui implementation serially, inspecting Flutter source on master, comparing controlled screenshots and behavior, implementing focused Go UI fixes, and validating parity with the native automation driver. Use for Wox Flutter-to-Go UI migrations, visual parity work, interaction parity, Settings or launcher screen synchronization, and requests to make the Go UI match the existing Flutter product.
---

# Align Wox Go UI

Align one named screen, component, or user flow at a time. Treat the running Flutter app as the product reference, Flutter source on `master` as explanatory evidence, and Go UI source on `feature-go-ui` as the implementation target.

## Non-negotiable constraints

- Never run Flutter Wox and Go UI Wox at the same time. Wox is single-instance.
- Before every Flutter or Go UI launch, terminate all existing Wox instances and verify a clean `stopped` boundary. A request that invokes this skill and requires launching Wox authorizes terminating exact verified Wox processes for runtime isolation, including installed/nested Flutter apps, `go run` children, `__debug_bin*`/Delve sessions, automation binaries, plugin hosts, and their Wox-owned debugger process.
- Enforce the runtime sequence `clean -> stopped -> Flutter -> clean -> stopped -> Go UI -> clean -> stopped`. Repeat the cleanup even when the previous launch appeared to exit normally.
- Inspect full command paths with `ps -Ao pid=,command=` before and after cleanup. Never kill by process name, a broad pattern, or an unresolved variable; collect exact PIDs first and exclude the process-check command itself. Do not assume closing a window ended Wox.
- Prefer graceful quit when a responsive Wox window is available. Otherwise terminate only the exact verified Wox-related PIDs, then repeat the process check until it is empty.
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

1. Find and terminate every existing Wox-related process, including IDE/Delve sessions and plugin hosts, then confirm the process check is empty.
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

1. Repeat the full Wox process cleanup and confirm the process check is empty, even if Flutter appeared to quit cleanly.
2. Choose the appropriate launch path:

   - Use the ordinary development entry for exploratory manual inspection:

     ```bash
     cd wox.core
     CGO_ENABLED=1 GOCACHE=/tmp/wox-go-cache go run -tags sqlite_fts5 .
     ```

   - Use the automation-enabled binary and bundled capture driver for reproducible Settings navigation, logical geometry, semantics, and screenshots. Prefer this path for before/after evidence.

3. Reproduce the exact comparison case and capture the same window and component states.
4. Quit Go UI gracefully and verify the Wox process is gone.
5. Compare the saved artifacts only after the serial captures are complete.

Do not treat a GUI-control timeout as proof that Go UI failed to launch. A `go run` child is bundleless on macOS and may not be discoverable by bundle-based GUI tools even when its native window is visible. Check the exact process, terminal output, Wox log, or automation endpoint before diagnosing launch failure.

### Reproducible Go UI Settings capture

Build the repository-owned automation binary from the repository root:

```bash
GOCACHE=/tmp/wox-go-cache make build-go-ui-smoke
```

Then run the skill's driver from `wox.core`. It uses `wox_automation`, `test/automationdriver`, the real settings window lifecycle, and an authenticated loopback endpoint:

```bash
GOCACHE=/tmp/wox-go-cache go run \
  ../.agents/skills/wox-align-go-ui/scripts/capture_go_ui_settings.go \
  -binary ./.tmp/wox-go-ui-smoke \
  -route /plugins/installed \
  -capture /tmp/wox-ui-parity/go/plugin-settings.png \
  -width 1152 \
  -height 768 \
  -wait-id settings.page.plugins \
  -wait-id plugin-search \
  -set-value 'plugin-search=剪贴板历史' \
  -key arrow-down
```

Use `-activate <automation-id>` with `-activate-capture <path.png>` to capture a non-destructive interaction state such as an opened dropdown:

```bash
  -activate plugin-settings-field-7 \
  -activate-capture /tmp/wox-ui-parity/go/plugin-settings-dropdown.png
```

Driver rules:

- Run it only after the Flutter process is confirmed stopped.
- By default it uses the active Wox user data so the captured state can match the installed Flutter app. Do not activate mutating controls merely to obtain a screenshot.
- For isolated behavior tests, pass both `-data-dir <temp-dir>` and `-user-dir <temp-dir>` and seed the required state explicitly.
- Use stable `AutomationID` values and inspect the printed semantics tree for roles, values, and logical bounds. If the needed shared control lacks semantics, add an appropriate generic semantic contract instead of relying on screen coordinates.
- `-width` and `-height` are logical pixels. Native captures on Retina are normally 2x physical pixels; compare normalized images or visual proportions, and use semantic bounds for geometry assertions.
- The driver owns and terminates the exact automation process group on every normal or error return. Still perform the stopped-boundary process check afterward.
- A sandbox may require approval for the native GUI launch and loopback listener. Request the required approval; do not replace the real runtime check with a mocked result.

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

Use this validation ladder:

1. Format every changed Go file with `gofmt`.
2. Run the narrowest changed packages from `wox.core`, for example:

   ```bash
   GOCACHE=/tmp/wox-go-cache go test ./ui/launcher/...
   ```

3. When shared widgets, runtime, automation, or launcher behavior changed, run from the repository root:

   ```bash
   GOCACHE=/tmp/wox-go-cache make test-go-ui-unit
   ```

4. Build the automation binary and run the bundled driver against the exact target route and state. Capture the initial state and each relevant interaction state. Confirm expected semantic roles and logical bounds in the driver's output.
5. Run `make test-go-ui-smoke` when the change affects an existing smoke contract or broader launcher/settings lifecycle. Treat a smoke assertion only as evidence for that exact contract. If an unrelated existing assertion fails before reaching the target, report it separately and continue with a focused automation case.
6. Repeat the same serial Flutter and Go UI capture case after implementation. Run the full cleanup and empty-process verification before each launch, including repeated launches of the same implementation.
7. Verify window geometry, rapid input where relevant, focus semantics, pointer and keyboard activation, scrolling, and visual output. A successful build is not runtime acceptance.
8. Re-run the process check before each launch and after each shutdown:

   ```bash
   ps -Ao pid=,command= | rg '(/Applications/[W]ox\.app|/\.wox[^/]*/ui/flutter/[w]ox-ui\.app|[w]ox-go-ui-smoke|/go-build.*/exe/[w]ox\.core|/wox\.core/(wox\.core|__debug_bin)|/\.wox[^/]*/hosts/(python-host|node-host)|[w]ox\.plugin\.host)'
   ```

   An empty result is the required `stopped` boundary. The character-class patterns avoid matching the process-check command itself. Because launching through this skill requires an isolated single instance, terminate exact matching IDE/Delve Wox sessions as part of cleanup.
9. Leave at most one Wox implementation running. Prefer restoring the initial runtime state; otherwise leave both stopped and report it.

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
