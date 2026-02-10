# AI Instructions for Wox

This document guides AI coding agents working in this repository.

## Big Picture Architecture

- `wox.core/`: Go backend and app core. Provides HTTP/WebSocket bridge to the UI, manages settings, plugins, database, i18n, and updates. Tests live under `wox.core/test/`.
- `wox.ui.flutter/wox/`: Flutter desktop UI (macOS/Linux/Windows). Talks to `wox.core` via WebSocket/HTTP. Build output is embedded under `wox.core/resource/ui/flutter/`.
- `wox.plugin.host.*/`: Runtime hosts for plugins (`wox.plugin.host.python`, `wox.plugin.host.nodejs`). They connect to `wox.core` (WebSocket/JSON-RPC), load plugin processes, and proxy plugin API calls.
- `wox.plugin.*/`: SDKs for third‑party plugins (`wox.plugin.python`, `wox.plugin.nodejs`) – provide typed APIs, models, and helper logic for plugin authors.

## Build & Test Commands

### Core (Go)

- **All checks**: `make dev` (verifies Go, Flutter, Node, pnpm, uv, etc.)
- **Build app**: `make build` (platform-aware; bundles UI, produces executables)
- **Run all tests**: `make test` or `make test-isolated`
- **Run single test**: `cd wox.core && go test ./test -v -run TestCalculatorBasic`
- **Test without network**: `make test-offline` (set WOX_TEST_ENABLE_NETWORK=false)
- **Verbose test logging**: `make test-verbose` (set WOX_TEST_VERBOSE=true)
- **Debug mode tests**: `make test-debug` (no cleanup, logs to /tmp/wox-test-debug)

### Flutter UI

- **Project root**: `wox.ui.flutter/wox/`
- **Build UI only**: `make -C wox.ui.flutter/wox build`
- **Analysis**: `cd wox.ui.flutter/wox && flutter analyze`

### Plugin Hosts & SDKs

- **Node.js host**: `make -C wox.plugin.host.nodejs build`
- **Python host**: `make -C wox.plugin.host.python build`
- **Python lint/format**: `make -C wox.plugin.host.python lint` `make -C wox.plugin.host.python format`
- **Node SDK**: `pnpm -C wox.plugin.nodejs lint`

## Code Style Guidelines

### Go (wox.core)

- **Formatting**: Use `gofmt` (auto-formatted on save)
- **Naming**: Packages in `lower_snake`, files `snake_case.go`, exported symbols `PascalCase`
- **Imports**: Use standard library first, then third-party, then local (blank line between groups)
- **Error handling**: Return `error` as last return value, check errors immediately with `if err != nil`
- **Context**: Pass `context.Context` as first parameter to most functions
- **Interfaces**: Define behavior contracts, prefer small focused interfaces
- **Testing**: Use table-driven tests with `[]QueryTest` struct slices and `NewTestSuite(t)`

### Dart (wox.ui.flutter)

- **Formatting**: 2-space indent, camelCase members, PascalCase classes/widgets
- **Private members**: Do NOT use leading underscore for private members/methods
- **State**: Use GetX controllers (`extends GetxController`) for state management
- **Widgets**: Keep widgets dumb, delegate state/logic to controllers
- **Imports**: Follow `flutter_lints` rules from `analysis_options.yaml`

### TypeScript (hosts/SDKs)

- **Formatting**: Files `kebab-case.ts`, types/interfaces `PascalCase`, max line 180
- **Linting**: ESLint with `@typescript-eslint` rules
- **Style**: Semi-colons required, arrow parens avoided, trailing commas none
- **Types**: Strict mode enabled, use TypeScript types for all plugin APIs

### Python (hosts/SDKs)

- **Formatting**: snake_case functions, CamelCase classes, max line 140
- **Type hints**: Use type hints for all function parameters and returns
- **Linting**: Ruff for linting/formatting, mypy for type checking
- **Async**: Use `async/await` for I/O operations, prefer explicit returns

### All Languages

- **Comments**: Concise English only for non-obvious logic; no narrative comments
- **Imports**: Organize alphabetically, remove unused imports
- **Error handling**: Wrap errors with context, log with trace IDs

## Internationalization

- **All user-facing text** must use i18n keys from `wox.core/resource/lang/*.json`
- **Supported languages**: `en_US.json`, `zh_CN.json`, `pt_BR.json`, `ru_RU.json`
- **When adding keys**: Translate to ALL 4 languages, organize by prefix (e.g., `ui_*`, `plugin_*`)
- **UI translations**: Use `wox.core/i18n` or Flutter localization facilities

## Architecture Patterns

### Core ↔ UI Communication

- `wox.core` exposes WebSocket/HTTP endpoints
- Flutter UI uses API layer under `wox.ui.flutter/wox/lib/api/`
- State managed by GetX controllers in `lib/controllers/*`
- Components in `lib/components/*` are dumb views driven by controllers

### Plugin System

- **Core definitions**: `wox.core/plugin/` (Go interfaces: `Plugin`, `SystemPlugin`, `API`)
- **Hosts**: Load plugins via WebSocket/JSON-RPC, proxy API calls
- **SDKs**: Provide typed APIs matching Go interfaces
- **Communication**: JSON-RPC over WebSocket between core → host → plugin
- **Sync**: Keep Go core, Python/Node hosts, and SDKs in sync when changing APIs

### Python Plugins

- Use models from `wox.plugin.python/src/wox_plugin/` (`Query`, `Result`, `Context`)
- Subclass `BasePlugin`, expose `plugin = MyPlugin()` instance
- See `wox.plugin.python/README.md` for examples

### Node.js Plugins

- Use TypeScript definitions from `wox.plugin.nodejs/types/`
- Implement `Plugin` interface with `init()` and `query()` methods
- See `wox.plugin.nodejs/README.md` for examples

## Testing Patterns

### Go Tests

- Create `NewTestSuite(t)` for setup
- Use table-driven tests: `tests := []QueryTest{{...}}`
- Run with `suite.RunQueryTests(tests)`
- Test names: `Test<Feature><Scenario>` (e.g., `TestCalculatorBasic`)
- Verify with `suite.RunQuery()` and check results

### Environment Isolation

- Tests run in isolated directories under `/tmp/wox-test-*`
- Set `WOX_TEST_DATA_DIR` for custom test directories
- `WOX_TEST_CLEANUP=false` to preserve test artifacts for debugging
- `WOX_TEST_VERBOSE=true` for detailed logging

## Safe Changes & Gotchas

- **Plugin API changes**: Update ALL three places: Go core, Python/Node hosts, SDKs
- **UI changes**: Update controller and widget together
- **No new build tools**: Use existing `make` targets
- **No backward compatibility**: Break old formats freely
- **Credentials**: Never commit keys; AI/MCP providers configured at runtime
- **Logs**: Check `~/.wox/log/log` for runtime logs

## Repo Rules

- **Comments**: English only, concise, for complex logic only
- **Refactors**: Scan `AGENTS.md` and `README.md` files first
- **Build**: Verify with `make build` in wox.core (you can skip UI build for small changes)
- **Tests**: Run narrowest relevant tests after changes, avoid breaking unrelated tests
- **Format**: When formatting your code, you must adhere to the coding style guidelines specified in Wox.code-workspace file.
