## Architecture

- `wox.core/`: Go backend and app core. It also owns the embedded Go UI under `wox.core/ui`, including native windows, focus lifecycle, widgets, and GPU rendering. Tests live under `wox.core/test/`.
- `wox.plugin.host.*/`: Runtime hosts for plugins (`wox.plugin.host.python`, `wox.plugin.host.nodejs`). They connect to `wox.core` (WebSocket/JSON-RPC), load plugin processes, and proxy plugin API calls.
- `wox.plugin.*/`: SDKs for third‑party plugins (`wox.plugin.python`, `wox.plugin.nodejs`) – provide typed APIs, models, and helper logic for plugin authors.

## Rules

- **Comments**: English only. Add intent-level comments only where they are necessary, such as complex logic, counterintuitive behavior, important state transitions, or code whose purpose is not obvious from the implementation.
- **Logging**: Use `util.GetLogger()` for new runtime and diagnostic logs so entries use Wox's configured output and formatting. Do not add `fmt.Print*`, standard-library `log.Print*`, or native stderr logging for application diagnostics.
- **Icons**: When adding or using UI icons, prefer an existing categorized SVG from `wox.core/common/icons.go` over font glyphs. Add reusable icons there before introducing local assets.
- **System Plugin I18n**: Put system-plugin and other Wox-owned user-facing strings in `wox.core/resource/lang/*.json` (`en_US`, `zh_CN`, `ru_RU`, `pt_BR`, `ko_KR`, `ja_JP`) and reference them with `i18n:` keys. Follow neighboring plugins such as Folder and Clipboard: use `plugin_<id>_plugin_name`, `plugin_<id>_plugin_description`, and `plugin_<id>_*` for commands, actions, and settings. Do not embed translation maps in Go, including `Metadata.I18n` or package-level `map[string]string`. `Metadata.I18n` is only for third-party plugins that ship their own `plugin.json` or `lang/` translations.
- **New Functions**: Add a short comment for new functions unless the function is trivial, such as 2-4 straightforward lines whose purpose is obvious from the name and body.
- **Change Comments**: For optimizations, bug fixes, and new features, add comments near the relevant code only when they clarify a non-obvious reason, previous limitation, or implementation choice. Avoid boilerplate comments for obvious changes.
- **Readability First**: Favor the simplest control flow that keeps behavior correct. Avoid clever abstractions, layered state handling, or indirection that make the execution path harder to follow.
- **Layout Alignment**: Prefer built-in horizontal and vertical alignment primitives for centering. Do not manually calculate padding or offsets when the layout system can express the intended alignment directly.
- **DPI And Scaling**: UI layout and screen-coordinate code must explicitly distinguish logical units from physical pixels. Scale visual chrome from the active display, convert coordinates at platform boundaries, and map captured images using their actual pixel dimensions. Do not assume window, pointer, desktop, or image coordinates share the same scale. Cover non-100% scaling, mixed-DPI multi-monitor layouts, display transitions, and negative desktop origins in implementation and focused tests.
- **Inline Small Logic**: Prefer keeping very small, single-use logic inline. Do not extract a 3-4 line block into a helper unless it is reused, clarifies a meaningful boundary, or clearly reduces complexity.
- **Explain Structures And Logic**: Add comments for complex structs, state transitions, control-flow branches, and non-obvious or counterintuitive logic. Do not comment obvious code just to satisfy a rule.
- **Refactors**: Scan `AGENTS.md` and `README.md` files first
- **Verification**: After code changes, run code formatting according to the project style. Go build may be run for Go/backend changes.
- **Format**: When formatting code, you must adhere to the coding style guidelines specified in Wox.code-workspace file.
- **Boundary Purity**: `widget.Boundary.Build` must derive its widget tree only from `Props` and stable callbacks carried by `Props`. It must not capture mutable application, controller, collection, or view state outside `Props`, because cache hits intentionally skip `Build`.
- **Linux Desktop Environments**: Linux desktop sessions differ enough that environment-specific behavior must not accumulate as branches inside a shared `*_linux.go` or `native_linux.c` file. Follow the clipboard package: keep a shared Linux interface and a selector that uses session helpers such as `util.IsKDEDesktopSession()`, `util.IsGnomeDesktopSession()`, `util.IsHyprlandSession()`, and `util.IsLinuxWaylandSession()`, then put each environment's implementation in its own file (`clipboard_linux_gnome.go`, `clipboard_linux_kde.go`, `clipboard_linux_hyprland.go`, `clipboard_linux_x11.go`). A GNOME, KDE/Plasma, Hyprland, or X11 change must stay in that environment's file so it cannot affect the others. When a new desktop needs different behavior, add a new file rather than extending a shared switch with more special cases.

## User Coding Style Preferences

- **Favor clarity and maintainability**: Prefer designs that reduce duplication and make intent obvious.
- **Keep flows easy to read**: Optimize for straightforward execution paths that can be understood quickly during review and debugging.
- **Prioritize consistency**: Keep implementation style and user-facing behavior coherent across related modules.
- **Respect boundaries**: Place responsibilities in the most appropriate layer to keep modules cohesive.
- **Align with existing conventions**: Follow established project patterns unless there is a strong reason to change them.
- **Preserve existing semantics**: Avoid accidental behavior changes during refactor and optimization.
- **Prefer extensible abstractions**: Choose approaches that support future evolution with minimal rework.
- **Document non-obvious change points**: Complex or counterintuitive optimization points, bug fixes, and feature additions should carry local comments that explain the reason for the change, the behavior being introduced or corrected, and the rationale behind the chosen solution. Obvious small changes do not need comments.

## Debug

- When troubleshooting an issue, if you cannot pinpoint the exact cause with 100% certainty, you can start by adding log statements to the relevant code and reviewing the logs to identify the problem. The log output should contain sufficient information to help understand the program’s state and behavior.
