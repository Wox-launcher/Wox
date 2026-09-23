# Wox Plugin Architecture Overview

This document provides a high-level overview of the Wox plugin system for developers and AI agents.

## Core Concepts

A Wox plugin is an event-driven module that interacts with the main application via JSON-RPC.

1. **Trigger**: User types a keyword (e.g., `npm`) or a global query.
2. **Execution**: Wox spawns or calls the plugin process.
3. **Response**: The plugin returns a list of **Results** (Items) to be displayed.
4. **Action**: User selects a result, triggering an **Action** callback in the plugin.

## Plugin Types

### 1. SDK Plugins (Managed)

Designed for complex, production-grade extensions. Wox manages the lifecycle of these plugins.

- **Node.js**: Written in TypeScript/JavaScript. Uses `@wox-launcher/wox-plugin`. Requires Node.js 20+.
- **Python**: Written in Python 3.10+. Uses `wox-plugin`.

**Benefits**:

- Full access to the Wox API (Notifications, Settings, Filesystem, AI, etc.).
- Persistent processes for faster response times.
- Strong typing and better tooling support.
- Multi-file layout, resources, and third-party dependencies inside a `.wox` package.

### 2. Single-file SDK Plugins

One `.py` or CommonJS `.js` file that loads into the existing Python or Node.js runtime host.

- Full Public API, same as a packaged SDK plugin.
- Query/action do not start a new process.
- Metadata lives in a comment JSON header.
- Requires Wox 2.4.2 or later. Header `MinWoxVersion` must be `"2.4.2"`.
- Saving the file reloads the plugin.
- Python can `import wox_plugin`. Node.js first version must use `module.exports.plugin` and `params.API`; it cannot import `@wox-launcher/wox-plugin`.
- No pip/npm dependencies, relative images, or extra files.

## Development Workflow

1. **Scaffold**:
   - **Node.js/Python packages**: Clone the official template repos.
     - https://github.com/Wox-launcher/Wox.Plugin.Template.Nodejs
     - https://github.com/Wox-launcher/Wox.Plugin.Template.Python
   - **Single-file SDK plugins**: Write one `.js` or `.py` file into `~/.wox/wox-user/plugins/single-file/` from `assets/single_file_plugin_templates/`, the scaffold (omit `--output-dir`), or `wpm create`. Do not also create a copy in the current repository.
2. **Configure**:
   - SDK plugins: edit `plugin.json` to define metadata, trigger keywords, supported OS, features, i18n, and `SettingDefinitions`.
   - Single-file SDK plugins: edit the JSON metadata block in the file header comments. Keep `MinWoxVersion` as `"2.4.2"`.
3. **Implement**:
   - `init()`: Initialize API clients and load settings. Called on every load/reload for SDK and single-file SDK plugins. Read and write settings through the Public API setting methods so values can sync across machines. If the plugin will cache files, resolve `get_cache_folder` / `GetCacheFolder` here and write later files under that path. Unless the plugin is clearly unsuitable for MRU, declare the `mru` feature and register `OnMRURestore` / `on_mru_restore` here. The restore callback must return immediately from memory or local cache; Wox waits 300ms, then discards that start-page item and shows the next one.
   - `query()`: Handle user input and return `QueryResponse` (results plus optional refinements and layout). Refinement hotkeys are `cmd+<key>` on macOS and `ctrl+<key>` on Windows/Linux; see `references/refinements.md`.
   - Plugin Tools: register them in `init()` at the same time as query/action features so other plugins can call the same capabilities. See `references/plugin_tools.md`. Requires Wox 2.4.5+.
   - Result rows use title, subtitle, and tails. Add a preview only for a large body. Refresh a visible row with `UpdateResult` / `update_result`. Capsule tail metrics are in `SKILL.md`.
   - Register unload callbacks if you create timers, watchers, or sockets.
4. **Internationalize**: Use the `I18n` field in `plugin.json` or the file header (recommended) or `lang/` files for packaged plugins. Single-file plugins only support inline `I18n`. See `plugin_i18n`.
5. **Validate settings-related work**:
   - Prefer the Public API setting methods over custom files or local storage. Those APIs are what Wox cloud-syncs between machines.
   - If the plugin caches downloads, thumbnails, or search results, put them under `get_cache_folder` / `GetCacheFolder` first (`~/.wox/cache/plugins/<plugin-id>/`). Do not create a sibling `cache/` directory. Wox deletes this folder on uninstall.
   - Read `references/plugin_json_schema.md` before authoring `SettingDefinitions`.
   - For validator syntax and advanced controls, read `references/settings_patterns.md`.

## Minimal Single-file SDK Plugin (Quick Start)

Single-file SDK plugins are the fastest way to get a Python or Node.js plugin that can call the full Wox API. They require Wox 2.4.2 or later.

When the user does not specify a language, detect this machine: only Node.js 20+ → Node.js, only Python 3.10+ → Python, both → Node.js.

1. **Create**: Write `Wox.Plugin.<Name>.js` or `.py` into `~/.wox/wox-user/plugins/single-file/` (or `wpm create`). That is the only file to create; a running Wox instance loads it immediately.
2. **Edit**: Update the JSON metadata block in that file. Keep `MinWoxVersion` as `"2.4.2"`.
3. **Implement**: Modify `query` in the same live file. Saving reloads the plugin.
4. **Run**: Trigger the plugin by typing its `TriggerKeywords` in Wox.

## Helper Prompts & Tools

- `get_plugin_json_schema`: Schema specification for `plugin.json`.
- `get_plugin_sdk_docs`: Detailed API documentation for Node.js and Python.
- `get_plugin_i18n`: Guidelines for implementing multi-language support.
