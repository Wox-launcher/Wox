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

- Can be Bash, Python, or any executable.
- Stateless and short-lived.

## Development Workflow

1. **Scaffold**: Generate a project structure using `wpm create <your_plugin_name>`.
2. **Configure**: Edit `plugin.json` to define metadata, keywords, and permissions.
3. **Implement**:
   - `init()`: Initialize API clients and load settings.
   - `query()`: Handle user input and return `Result[]`.
4. **Internationalize**: Use the `I18n` field in `plugin.json` (recommended) or `lang/` files. See `plugin_i18n`.

## Helper Prompts & Tools

- `get_plugin_json_schema`: Schema specification for `plugin.json`.
- `get_plugin_sdk_docs`: Detailed API documentation for Node.js and Python.
- `get_plugin_i18n`: Guidelines for implementing multi-language support.
