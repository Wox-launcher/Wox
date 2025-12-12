# Wox Plugin Overview (for Plugin Developers)

This is a developer-focused overview to help you choose a plugin type and get started quickly.

## What you build

A Wox plugin is a small integration that:

- Receives a user query.
- Returns a list of results (items/actions) for Wox to display.

You mainly decide **which plugin type** to use, and then implement the query → results behavior.

## Plugin types (which one should I choose?)

### 1) SDK plugins (recommended)

Use this when you want a “real” plugin with richer capabilities, maintainability, and a clear structure.

- **Python SDK plugin**: best if you prefer Python.
- **Node.js SDK plugin**: best if you prefer JavaScript/TypeScript.

What you typically ship:

- A `plugin.json` manifest (metadata like id/name/keywords/entrypoint).
- Your plugin source code (Python or Node.js).

### 2) Script plugins (lightweight)

Use this for quick utilities and small automations.

- Usually a single script (Python/Node.js/Bash) based on the provided templates.
- Stored under the user scripts plugin directory (see tool `get_wox_directories`).

Tradeoff: fastest to start, but generally less suitable for larger projects.

### 3) Built-in/system plugins

Some features are shipped as part of Wox itself. As a third-party developer, you usually don’t implement these.

## Recommended workflow

1. Decide runtime: Python or Node.js or Script plugin
2. Generate a starter scaffold.
3. Adjust `plugin.json` (id/name/keywords/entrypoint).
4. Implement query handling and return results.

## Helpful tools (for an AI agent)

- `plugin_overview`: quick orientation (this document).
- `get_plugin_json_schema`: generate/validate a correct `plugin.json`.
- `get_plugin_sdk_docs`: learn the runtime-specific API.
- `generate_plugin_scaffold`: create a starter plugin skeleton.
