---
name: wox-plugin-creator
description: Create, scaffold, implement, and package Wox plugins (nodejs, python, script-nodejs, script-python). Use when cloning official SDK templates, generating script plugin templates, editing plugin.json metadata, defining SettingDefinitions and validators, wiring i18n, implementing plugin APIs, or preparing plugin repositories for local packaging. If the user wants to publish a plugin to the official Wox store or check whether it is already listed, prefer wox-plugin-submit2store.
---

# Wox Plugin Creator

## Quick Start

- Scaffold a Node.js plugin (clones template repo):
  - `python3 scripts/scaffold_wox_plugin.py --type nodejs --output-dir ./MyPlugin --name "My Plugin" --trigger-keywords my`
- Scaffold a Python plugin (clones template repo):
  - `python3 scripts/scaffold_wox_plugin.py --type python --output-dir ./MyPlugin --name "My Plugin" --trigger-keywords my`
- Scaffold a script plugin (uses local templates; plugin-id auto-generated; single file output):
  - `python3 scripts/scaffold_wox_plugin.py --type script-nodejs --output-dir ./Wox.Plugin.Script.MyScript.js --name "My Script" --trigger-keywords my`

## Workflow

### 1) Scaffold plugin files

- Use `scripts/scaffold_wox_plugin.py` for `nodejs`, `python`, `script-nodejs`, or `script-python`.
- Pass `--name` and `--trigger-keywords` for every runtime. The scaffold exits without them.
- For Node.js and Python, the scaffold clones the official template repos and replaces placeholders like `{{.ID}}`, `{{.Name}}`, `{{.Description}}`, `{{.TriggerKeywordsJSON}}`, `{{.Author}}`.
- Before starting work in a new SDK plugin project, run `make init` in the project root when the project has not been initialized yet.
- Script plugins are **single-file** plugins. Prefer filenames like `Wox.Plugin.Script.<Name>.<ext>` (e.g., `Wox.Plugin.Script.Memos.py`).
- For script plugins, the scaffold copies Wox script templates from `~/.wox/ai/skills/wox-plugin-creator/assets/script_plugin_templates/` and fills metadata placeholders.
- Prefer standard library features; avoid third-party dependencies unless absolutely necessary.
- For SDK usage and API details, read `references/sdk_nodejs.md` or `references/sdk_python.md`.
- For `plugin.json`, `SettingDefinitions`, `QueryRequirements`, validators, dynamic settings, and feature flags, read `references/plugin_json_schema.md` first.
- When a plugin cannot run a query without required settings such as access keys, declare those requirements in metadata `QueryRequirements` instead of returning ad hoc setup results from `query()`.
- For ready-to-copy patterns such as validated textbox/select fields, editable tables, AI model selectors, and dynamic preview settings, read `references/settings_patterns.md`.
- For Python settings APIs, note that helper builders are limited; advanced settings are often created by constructing `PluginSettingDefinitionItem` and value objects directly.

### 2) Author result and action icons

- Read `references/icons.md` for icon selection, inline SVG patterns, and placement rules.
- When the requested icon semantics already match a bundled generic icon under `assets/iconify/`, prefer reusing that local reference before searching for a new one.
- Use `scripts/search_iconify.py` to search Iconify collections and fetch ready-to-inline SVG constants for `icons.ts` or `icons.py`.

### 3) Package and submit plugin

- For SDK plugins cloned from templates, run `make package` inside the template repo.
- For submitting a plugin to the official Wox store, prefer `wox-plugin-submit2store` skill.
- Script plugins do not use `plugin.json`; they embed a JSON metadata block in the script header comments.

## Resources

- scripts: `scripts/scaffold_wox_plugin.py`, `scripts/search_iconify.py`
- references: `references/plugin_overview.md`, `references/scaffold_nodejs.md`, `references/scaffold_python.md`, `references/sdk_nodejs.md`, `references/sdk_python.md`, `references/plugin_json_schema.md`, `references/settings_patterns.md`, `references/plugin_i18n.md`, `references/icons.md`
- assets: `assets/script_plugin_templates/`, `assets/iconify/`
