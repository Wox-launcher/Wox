# AI Instructions for Wox

This document guides AI coding agents working in this repository.

## Big Picture Architecture

- `wox.core/`: Go backend and app core. Provides HTTP/WebSocket bridge to the UI, manages settings, plugins, database, i18n, and updates. Tests live under `wox.core/test/`.
- `wox.ui.flutter/wox/`: Flutter desktop UI (macOS/Linux/Windows). Talks to `wox.core` via WebSocket/HTTP. Build output is embedded under `wox.core/resource/ui/flutter/`.
- `wox.plugin.host.*/`: Runtime hosts for plugins (`wox.plugin.host.python`, `wox.plugin.host.nodejs`). They connect to `wox.core` (WebSocket/JSON-RPC), load plugin processes, and proxy plugin API calls.
- `wox.plugin.*/`: SDKs for third‑party plugins (`wox.plugin.python`, `wox.plugin.nodejs`) – provide typed APIs, models, and helper logic for plugin authors.
- Other top‑level folders: `assets/`, `docs/`, `ci/`, `release/`, `screenshots/`, plus repo‑level `Makefile` orchestrating builds/tests.

## Key Development Workflows

- **Core build & tests (Go):**
  - Run all checks: `make dev` (verifies Go, Flutter, Node, pnpm, uv, etc.).
  - Run Go tests: `make test` (see variants `make test-offline`, `make test-verbose`, `make test-debug`).
  - Direct Go tests: `cd wox.core && go test ./test -v`.
  - Build app: `make build` (platform-aware; bundles UI, produces executables).
- **Flutter UI:**
  - Project root: `wox.ui.flutter/wox/`.
  - Build UI only: `make -C wox.ui.flutter/wox build`.
  - Follow `flutter_lints` and `analysis_options.yaml` (2‑space indent, `PascalCase` widgets, etc.).
- **Plugin hosts & SDKs:**
  - Node.js host: `make -C wox.plugin.host.nodejs build`.
  - Python host: `make -C wox.plugin.host.python build` (lint/format via `make -C wox.plugin.host.python lint format`).
  - Node SDK: lint with `pnpm -C wox.plugin.nodejs lint`.
  - Python SDK (`wox.plugin.python`): type-hinted, distributed as a library (`pip install wox-plugin` or `uv add wox-plugin`).

## Project-Specific Conventions

- **General style:**
  - Go (`wox.core`): `gofmt`, packages in `lower_snake`, files `snake_case.go`. Exported symbols start with uppercase.
  - Dart: camelCase for members, `PascalCase` for classes/widgets; avoid comments except for non-obvious logic.
  - TypeScript: files `kebab-case.ts`, types/interfaces `PascalCase`.
  - Python: `snake_case` for functions, `CamelCase` for classes, prefer type hints and explicit returns.
- **Internationalization:**
  - All user-facing text should be i18n-friendly. Prefer existing i18n facilities under `wox.core/i18n/` and UI-level localization rather than hardcoding text.
  - When adding new i18n keys, **always translate for ALL supported languages**: `en_US.json`, `zh_CN.json`, `pt_BR.json`, and `ru_RU.json`.
  - Organize i18n keys by prefix in language files: keep `ui_*` keys together, `plugin_*` keys together, etc. for maintainability.
- **Comments:**
  - Use concise English comments only when logic is non-trivial. Do not add redundant or narrative comments.

## Patterns & Data Flow

- **Core ↔ UI:**
  - `wox.core` exposes WebSocket/HTTP endpoints; Flutter UI consumes them via the API layer under `wox.ui.flutter/wox/lib/api/` and updates state controllers under `lib/controllers/`.
  - UI components (e.g., `lib/components/wox_list_view.dart`) are usually driven by GetX controllers (`lib/controllers/*`) and theme utilities (`lib/utils/wox_theme_util.dart`). When adding UI features, follow this pattern: controller owns state, widgets are dumb views.
- **Core plugins:**
  - Plugin definitions live under `wox.core/plugin/`. System plugins (e.g., calculator, media player) have their own folders and often ship a README with usage and implementation notes.
  - When touching plugin APIs, keep Go definitions (`wox.core/plugin/*`) in sync with SDKs (`wox.plugin.python/src/wox_plugin`, `wox.plugin.nodejs/src` and `types/`).
- **Python plugins:**
  - Use models from `wox.plugin.python/src/wox_plugin/models/` (`Query`, `Result`, `Context`, etc.).
  - Plugins subclass `BasePlugin` and expose a `plugin = MyPlugin()` instance; see the README example in `wox.plugin.python/`.
- **Node.js plugins:**
  - Use TypeScript definitions under `wox.plugin.nodejs/types/` and helpers in `wox.plugin.nodejs/src/`. Keep type declarations and runtime code in sync when changing plugin contracts.

## Safe Changes & Gotchas for AI Agents

- Prefer small, focused edits aligned to existing patterns: e.g., when changing a UI interaction, update the corresponding controller and widget together (`lib/controllers/*`, `lib/components/*`).
- When modifying plugin APIs or core contracts, check and update **all** relevant places:
  - Go core (`wox.core/plugin`, `wox.core/ai`, `wox.core/common`).
  - Python/Node hosts (`wox.plugin.host.*`).
  - Python/Node SDKs (`wox.plugin.*`).
- Do not introduce new build tools; reuse `make` targets defined in the root `Makefile` and subproject `Makefile`s.
- Never commit credentials or hardcode keys. AI/MCP provider keys are configured at runtime via app settings.

## Repo-Specific Rules

- Use English for all code comments.
- Runtime logs are written under `~/.wox/log/log`; check this path when you need to inspect logs.
- Do not try to compile or run the app yourself; the human maintainer will build and run Wox to verify changes.

## How AI Agents Should Work Here

- Respect existing coding standards from `.github/instructions/*.instructions.md` (Go, Python, Dart, etc.).
- Before large refactors, scan `AGENTS.md` and relevant `README.md` files for context on intended architecture.
- After editing code that can be tested, prefer running the narrowest relevant tests (e.g., `make test` for core, or project-specific build/lint commands) and avoid changing unrelated failing tests.
