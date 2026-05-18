## Architecture

- `wox.core/`: Go backend and app core. Provides HTTP/WebSocket bridge to the UI, manages settings, plugins, database, i18n, and updates. Tests live under `wox.core/test/`.
- `wox.ui.flutter/wox/`: Flutter desktop UI (macOS/Linux/Windows). Talks to `wox.core` via WebSocket/HTTP. Build output is embedded under `wox.core/resource/ui/flutter/`.
- `wox.plugin.host.*/`: Runtime hosts for plugins (`wox.plugin.host.python`, `wox.plugin.host.nodejs`). They connect to `wox.core` (WebSocket/JSON-RPC), load plugin processes, and proxy plugin API calls.
- `wox.plugin.*/`: SDKs for third‑party plugins (`wox.plugin.python`, `wox.plugin.nodejs`) – provide typed APIs, models, and helper logic for plugin authors.

## Rules

- **Comments**: English only. Add clear intent-level comments where appropriate.
- **Change Comments Required**: Every optimization, bug fix, and new feature must include comments near the relevant code that explain what changed, why the previous behavior or structure was not enough, and why the chosen implementation is used.
- **Readability First**: Favor the simplest control flow that keeps behavior correct. Avoid clever abstractions, layered state handling, or indirection that make the execution path harder to follow.
- **Inline Small Logic**: Prefer keeping very small, single-use logic inline. Do not extract a 3-4 line block into a helper unless it is reused, clarifies a meaningful boundary, or clearly reduces complexity.
- **Explain Structures And Logic**: Add necessary comments for structs, state transitions, control-flow branches, and non-obvious logic so readers can understand the intent without reverse-engineering the code.
- **Refactors**: Scan `AGENTS.md` and `README.md` files first
- **Build**: Verify with `make build` in wox.core (you should skip Go or UI build for small changes)
- **Unit Tests**: Do not write unit tests unless the user requests them
- **Smoke Tests**: For major fixes or feature additions, corresponding smoke tests must be added
- **Format**: When formatting code, you must adhere to the coding style guidelines specified in Wox.code-workspace file.

## User Coding Style Preferences

- **Favor clarity and maintainability**: Prefer designs that reduce duplication and make intent obvious.
- **Keep flows easy to read**: Optimize for straightforward execution paths that can be understood quickly during review and debugging.
- **Prioritize consistency**: Keep implementation style and user-facing behavior coherent across related modules.
- **Respect boundaries**: Place responsibilities in the most appropriate layer to keep modules cohesive.
- **Align with existing conventions**: Follow established project patterns unless there is a strong reason to change them.
- **Preserve existing semantics**: Avoid accidental behavior changes during refactor and optimization.
- **Prefer extensible abstractions**: Choose approaches that support future evolution with minimal rework.
- **Document each change point**: Optimization points, bug fixes, and feature additions should carry local comments that explain the reason for the change, the behavior being introduced or corrected, and the rationale behind the chosen solution.


## Debug
- When troubleshooting an issue, if you cannot pinpoint the exact cause with 100% certainty, you can start by adding log statements to the relevant code and reviewing the logs to identify the problem. The log output should contain sufficient information to help understand the program’s state and behavior.
