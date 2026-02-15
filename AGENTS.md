## Architecture

- `wox.core/`: Go backend and app core. Provides HTTP/WebSocket bridge to the UI, manages settings, plugins, database, i18n, and updates. Tests live under `wox.core/test/`.
- `wox.ui.flutter/wox/`: Flutter desktop UI (macOS/Linux/Windows). Talks to `wox.core` via WebSocket/HTTP. Build output is embedded under `wox.core/resource/ui/flutter/`.
- `wox.plugin.host.*/`: Runtime hosts for plugins (`wox.plugin.host.python`, `wox.plugin.host.nodejs`). They connect to `wox.core` (WebSocket/JSON-RPC), load plugin processes, and proxy plugin API calls.
- `wox.plugin.*/`: SDKs for third‑party plugins (`wox.plugin.python`, `wox.plugin.nodejs`) – provide typed APIs, models, and helper logic for plugin authors.

## Rules

- **Comments**: English only, concise
- **Refactors**: Scan `AGENTS.md` and `README.md` files first
- **Build**: Verify with `make build` in wox.core (you can skip UI build for small changes)
- **Tests**: Run narrowest relevant tests after changes, avoid breaking unrelated tests
- **Format**: When formatting code, you must adhere to the coding style guidelines specified in Wox.code-workspace file.
