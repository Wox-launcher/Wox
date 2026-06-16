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

- **Node.js**: Written in TypeScript/JavaScript. Uses `@wox-launcher/wox-plugin`.
- **Python**: Written in Python 3.x. Uses `wox-plugin`.

**Benefits**:

- Full access to the Wox API (Notifications, Settings, Filesystem, AI, etc.).
- Persistent processes (optional) for faster response times.
- Strong typing and better tooling support.

### 2. Script Plugins (Unmanaged)

Designed for simple, one-off tasks or shell scripts.

- Can be Python or Node.js.
- Stateless and short-lived.

## Development Workflow

1. **Scaffold**:
   - **Node.js/Python**: Clone the official template repos.
     - https://github.com/Wox-launcher/Wox.Plugin.Template.Nodejs
     - https://github.com/Wox-launcher/Wox.Plugin.Template.Python
   - **Script plugins**: Use the script templates under `assets/script_plugin_templates/`.
2. **Configure**:
   - SDK plugins: edit `plugin.json` to define metadata, trigger keywords, supported OS, features, i18n, and `SettingDefinitions`.
   - Script plugins: edit the JSON metadata block in the script header comments.
3. **Implement**:
   - `init()`: Initialize API clients and load settings.
   - `query()`: Handle user input and return `Result[]`.
4. **Internationalize**: Use the `I18n` field in `plugin.json` (recommended) or `lang/` files. See `plugin_i18n`.
5. **Validate settings-related work**:
   - Read `references/plugin_json_schema.md` before authoring `SettingDefinitions`.
   - For validator syntax and advanced controls, read `references/settings_patterns.md`.

## Minimal Script Plugin (Quick Start)

Script plugins are the fastest way to get a working plugin with no build step.

1. **Create**: Start from the script templates under `assets/script_plugin_templates/`.
2. **Edit**: Open the generated `.py` or `.js` file and update the JSON metadata block in comments.
3. **Implement**: Modify the `query` handler in the same file to return results.
4. **Run**: Trigger your plugin by typing its `TriggerKeywords` in Wox.

## Helper Prompts & Tools

- `get_plugin_json_schema`: Schema specification for `plugin.json`.
- `get_plugin_sdk_docs`: Detailed API documentation for Node.js and Python.
- `get_plugin_i18n`: Guidelines for implementing multi-language support.
