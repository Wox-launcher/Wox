# Repository Guidelines

## Project Structure & Module Organization
- `wox.core/`: Go core, HTTP/UI bridge, resources under `wox.core/resource/`. Tests live in `wox.core/test/`.
- `wox.ui.flutter/wox/`: Flutter desktop UI (macOS/Linux/Windows). Builds into `wox.core/resource/ui/flutter/`.
- `wox.plugin.host.nodejs/`, `wox.plugin.host.python/`: Plugin host runtimes for Node.js and Python.
- `wox.plugin.nodejs/`, `wox.plugin.python/`: Plugin SDKs/templates for third‑party plugins.
- Other: `assets/`, `docs/`, `ci/`, `release/`, repo‑level `Makefile`.

## Build, Test, and Development Commands
- `make dev`: Verify deps (go, flutter, node, pnpm, uv; plus create-dmg on macOS).
- `make test`: Run Go tests in isolation. Variants: `make test-offline`, `make test-verbose`, `make test-debug`.
- `make build`: Build core binary and package per platform (bundles app on macOS).
- `make plugins`: Regenerate the plugin store metadata.
- UI only: `make -C wox.ui.flutter/wox build`.
- Hosts: `make -C wox.plugin.host.nodejs build`, `make -C wox.plugin.host.python build`.

## Coding Style & Naming Conventions
- Go (`wox.core`): Format with `gofmt`; keep packages short, lower_snake; files `snake_case.go`.
- Dart/Flutter: Follows `flutter_lints` (see `analysis_options.yaml`); 2‑space indent; widgets/classes `PascalCase`.
- Node (TypeScript): ESLint + Prettier in `wox.plugin.nodejs`; run `pnpm -C wox.plugin.nodejs lint`; types/interfaces `PascalCase`, files `kebab-case.ts`.
- Python Host: Ruff + MyPy; run `make -C wox.plugin.host.python lint format`; prefer type hints and explicit returns.

## Testing Guidelines
- Location: `wox.core/test/*.go`; name tests `*_test.go` with focused cases.
- Run: `make test` or `cd wox.core && go test ./test -v`.
- Envs: no network `make test-offline`; custom dirs `WOX_TEST_DATA_DIR=/tmp/wox-test go test ./test -v`.

## Commit & Pull Request Guidelines
- Conventional commits: `feat(core): …`, `fix(ui): …`, `refactor(mediaplayer): …` (see `git log`). Use clear scope: `core`, `ui`, `plugin`, `store`, `build`.
- PRs: include description, rationale, test plan, affected platforms, linked issues; add screenshots for UI changes.

## Security & Configuration Tips
- Never commit credentials. Configure AI/MCP keys via the app Settings at runtime.
- macOS signing/notarization is required for release (`create-dmg`, codesign). Local development does not require signing.
