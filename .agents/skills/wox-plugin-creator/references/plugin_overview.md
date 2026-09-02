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
- Saving the file reloads the plugin.
- Python can `import wox_plugin`. Node.js first version must use `module.exports.plugin` and `params.API`; it cannot import `@wox-launcher/wox-plugin`.
- No pip/npm dependencies, relative images, or extra files.

### 3. Script Plugins (Unmanaged)

Designed for simple, one-off tasks or shell scripts.

- Can be Python 3.10+ or Node.js 20+.
- Stateless and short-lived: Wox starts a process per query over stdin/stdout JSON-RPC.
- Store install and query execution reject older interpreters. Script plugins run with the user's system Python or Node.js, not a bundled runtime.

Script plugins are not deprecated. Use them when a one-shot command wrapper is enough.

## Development Workflow

1. **Scaffold**:
   - **Node.js/Python packages**: Clone the official template repos.
     - https://github.com/Wox-launcher/Wox.Plugin.Template.Nodejs
     - https://github.com/Wox-launcher/Wox.Plugin.Template.Python
   - **Single-file SDK plugins**: Use the templates under `assets/single_file_plugin_templates/`, or `wpm create`.
   - **Script plugins**: Use the script templates under `assets/script_plugin_templates/`.
2. **Configure**:
   - SDK plugins: edit `plugin.json` to define metadata, trigger keywords, supported OS, features, i18n, and `SettingDefinitions`.
   - Single-file SDK plugins and script plugins: edit the JSON metadata block in the file header comments.
3. **Implement**:
   - `init()`: Initialize API clients and load settings. Called on every load/reload for SDK and single-file SDK plugins.
   - `query()`: Handle user input and return `Result[]`.
   - Register unload callbacks if you create timers, watchers, or sockets.
4. **Internationalize**: Use the `I18n` field in `plugin.json` or the file header (recommended) or `lang/` files for packaged plugins. Single-file plugins only support inline `I18n`. See `plugin_i18n`.
5. **Validate settings-related work**:
   - Read `references/plugin_json_schema.md` before authoring `SettingDefinitions`.
   - For validator syntax and advanced controls, read `references/settings_patterns.md`.

## Minimal Single-file SDK Plugin (Quick Start)

Single-file SDK plugins are the fastest way to get a Python or Node.js plugin that can call the full Wox API.

1. **Create**: `wpm create <name>` and choose Python or Node.js single-file, or start from `assets/single_file_plugin_templates/`.
2. **Edit**: Open the generated `.py` or `.js` file and update the JSON metadata block in comments.
3. **Implement**: Modify `query` in the same file. Saving reloads the plugin.
4. **Run**: Trigger the plugin by typing its `TriggerKeywords` in Wox.

## Minimal Script Plugin (Quick Start)

Script plugins are the fastest way to wrap a command with no SDK host.

1. **Create**: Start from the script templates under `assets/script_plugin_templates/`.
2. **Edit**: Open the generated `.py` or `.js` file and update the JSON metadata block in comments.
3. **Implement**: Modify the `query` handler in the same file to return results.
4. **Run**: Trigger your plugin by typing its `TriggerKeywords` in Wox.

## Helper Prompts & Tools

- `get_plugin_json_schema`: Schema specification for `plugin.json`.
- `get_plugin_sdk_docs`: Detailed API documentation for Node.js and Python.
- `get_plugin_i18n`: Guidelines for implementing multi-language support.
