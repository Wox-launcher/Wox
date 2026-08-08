---
name: wox-align-go-ui
description: Align Wox Go UI screens, behavior, and interactions with the Flutter implementation through source-level contract tracing by default, then implement focused Go UI changes and run targeted tests. Use for Wox Flutter-to-Go UI migrations, Settings or launcher synchronization, visual or interaction parity work, and requests to make Go UI match the former or current Flutter product. Request a serial runtime UI comparison only for complex flows, native lifecycle behavior, ambiguous source contracts, or special visual effects that source inspection cannot establish reliably.
---

# Align Wox Go UI

Align one named screen, component, state, or flow at a time. Use Flutter source as the default product contract and Go UI source as the implementation target. Source-level alignment is sufficient unless the user explicitly asks for runtime visual validation or the escalation criteria below apply.

## Choose the alignment level

### Default: source-level alignment

Do not launch Flutter Wox, launch Go UI, capture screenshots, or operate desktop UI merely because this skill was invoked.

Use source-level alignment when Flutter code can establish the contract for:

- routes, hierarchy, sections, ordering, and responsive structure
- constraints, padding, gaps, typography, colors, borders, radii, icons, and shared tokens
- controller state, API calls, subscriptions, validation, and asynchronous transitions
- keyboard, focus, hover, pointer, scroll, semantics, and lifecycle intent
- initial, loading, empty, populated, error, disabled, and long-content states

Complete the implementation with focused formatting and tests. Report the result as **source-level aligned** and state that runtime visual comparison was not performed.

### Escalate: runtime UI alignment

Recommend a runtime UI comparison when one or more of these conditions make source inspection insufficient:

- exact animation, blur, shadow, clipping, compositing, rasterization, or other special visual effects matter
- platform-native focus, IME, window ownership, resize, close/reopen, or secondary-window behavior is part of acceptance
- the feature has a complex multi-step or timing-sensitive flow whose observable behavior cannot be inferred confidently
- installed Flutter behavior may have drifted from the available Flutter source
- layout depends on runtime measurement, DPI, fonts, data, or platform appearance and the source leaves a meaningful ambiguity
- the user explicitly requests screenshots, pixel-level parity, live comparison, or native UI validation

If the user did not explicitly request runtime UI work, first finish any safe source investigation, then ask whether to upgrade to runtime UI alignment. Briefly identify the uncertain behavior and the extra work required. Do not launch or terminate Wox until the user agrees. If the user declines, finish at source level and record the limitation.

## Establish scope and repository state

1. Resolve the repository root, normally `/Users/qianlifeng/Projects/Wox`.
2. Read applicable `AGENTS.md`, relevant `README.md` files, and `Wox.code-workspace`.
3. Inspect the working tree and references without modifying user work:

   ```bash
   git status --short --branch
   git worktree list --porcelain
   git branch --list master feature-go-ui
   ```

4. Identify the smallest requested surface and concrete acceptance states. If the request is broad, cover one coherent surface at a time while scanning related production implementations for shared contracts.
5. Never switch branches in a dirty checkout, stash user changes, discard changes, or overwrite unrelated work. Do not edit Flutter code unless explicitly requested.

## Trace the Flutter contract from source

Prefer focused history inspection over switching the implementation checkout:

```bash
git grep -n '<visible text, widget, route, controller, or setting>' master -- wox.ui.flutter/wox
git show master:<path>
```

If Flutter no longer exists on `master`, locate the last revision that contains the relevant path and inspect its parent or file history with `git log`, `git rev-list`, and `git show`. Record the exact reference used rather than assuming current `master` still contains Flutter.

Trace the complete contract, not only the leaf widget:

| Contract | Inspect |
| --- | --- |
| Structure | route, window, hierarchy, sections, ordering, responsive branches |
| Geometry and style | constraints, padding, gaps, alignment, theme tokens, type, colors, borders, radius, shadow, icons |
| State | controller ownership, API calls, subscriptions, async transitions, loading, empty, error, disabled, long content |
| Interaction | click, hover, keyboard, focus, IME, scrolling, shortcuts, resize |
| Semantics and lifecycle | roles, labels, focusability, show/hide, close/reopen, secondary windows, focus restoration |

Build a concise Flutter-to-Go mapping that names:

- Flutter route, view/widget, controller, and shared component owners
- Go UI window/controller, adapter, view, widget, and theme owners
- each state or interaction that must be preserved
- deliberate platform differences or contracts that cannot be established from source

Do not copy Flutter code line for line. Preserve its product behavior in the correct Go UI layer.

## Compare and implement in Go UI

1. Inspect the current Go UI execution path before editing, including controller state, adapters, pure views, shared components, runtime/window ownership, and tests.
2. Classify each difference as shared-contract, page-specific, intentional platform behavior, or unresolved ambiguity.
3. Fix contract and interaction defects first, structural/layout differences second, and cosmetic constants last.
4. Prefer shared widgets, theme tokens, layout primitives, and lifecycle abstractions when multiple production surfaces share the behavior. Avoid a page-local workaround for a shared defect.
5. Keep build/render callbacks pure. Perform I/O, asynchronous loading, native window work, and WebView lifecycle work outside rendering, returning UI updates through the repository's UI-thread boundary.
6. Keep text input, caret, selection, focus, and IME ownership in retained text-field state. Keep launcher result rows out of the keyboard focus chain while preserving pointer and accessibility activation.
7. Preserve independent window instances, synchronization, and complete teardown for secondary-window behavior.
8. Keep control flow straightforward. Add English intent comments only for non-obvious reasons, state transitions, or constraints. Add short comments for non-trivial new functions.
9. Use `apply_patch` for edits and format according to repository rules.

## Validate source-level alignment

Use a risk-proportional validation ladder:

1. Format all changed files. Use `gofmt` for Go; use the repository-configured Dart line length when Flutter edits were explicitly requested.
2. Run the narrowest affected Go packages from `wox.core`, for example:

   ```bash
   GOCACHE=/tmp/wox-go-cache go test ./ui/launcher/...
   ```

3. When shared widgets, runtime, automation, or launcher behavior changed, run from the repository root:

   ```bash
   GOCACHE=/tmp/wox-go-cache make test-go-ui-unit
   ```

4. Add or update focused tests for meaningful state, interaction, semantics, or layout contracts that can be asserted without a desktop runtime.
5. Review the final diff against the Flutter-to-Go mapping. Confirm every scoped contract is implemented or explicitly listed as unresolved.

Compilation and tests validate the source implementation, not pixel-level or native runtime parity. Do not imply that runtime UI behavior was observed when it was not.

## Run runtime UI alignment only after approval

When runtime alignment is explicitly requested or approved, use the following additional workflow.

### Preserve single-instance isolation

- Never run Flutter Wox and Go UI Wox at the same time.
- Before every launch, inspect full command paths with `ps -Ao pid=,command=`, collect exact verified Wox-related PIDs, terminate only those processes, and repeat the check until it is empty.
- Prefer graceful quit for a responsive window. Never kill by broad process name, pattern, glob, or unresolved variable.
- Enforce `clean -> stopped -> Flutter -> clean -> stopped -> Go UI -> clean -> stopped`.
- Load and follow the available Computer Use skill before GUI operations. Request required sandbox approval for GUI launch or loopback automation.

### Compare one reproducible case

Define the exact route, navigation, window geometry, display scale, theme, locale, data state, query, selection, scroll, focus/hover target, and expected state. Use the same case for both implementations.

Capture Flutter first and Go UI second into separate directories under `/tmp/wox-ui-parity/<timestamp>/`. Record observable geometry and behavior rather than impressions. If the installed Flutter bundle disagrees with the selected source revision, report the drift and treat the installed product as the runtime reference for that comparison.

For reproducible Go UI Settings capture, build the automation target from the repository root:

```bash
GOCACHE=/tmp/wox-go-cache make build-go-ui-smoke
```

Then run `.agents/skills/wox-align-go-ui/scripts/capture_go_ui_settings.go` from `wox.core` with the target route, logical width/height, stable `AutomationID` waits, and capture path. Use the driver's activation, hover, input, and key options only for scoped, non-destructive states. Inspect its semantics tree for roles, values, and logical bounds instead of relying on desktop coordinates.

After implementation, repeat the same serial case. Validate relevant geometry, visual output, keyboard/pointer behavior, focus/IME, scrolling, and lifecycle states. Run `make test-go-ui-smoke` when the change affects an existing smoke contract. Leave both implementations stopped unless the user requested another final state.

## Completion report

Report:

- aligned screen, flow, and states
- Flutter source revision and Flutter-to-Go ownership mapping
- files changed and differences fixed
- formatting and focused tests actually completed
- unresolved or intentional differences
- alignment level: source-level, or approved runtime UI comparison
- for runtime comparison only: installed bundle identity, comparison conditions, capture directory, runtime/smoke evidence, and final process state

Use **source-level aligned** when only source and tests were checked. Reserve claims of visual or runtime parity for cases actually launched and compared after the change.
