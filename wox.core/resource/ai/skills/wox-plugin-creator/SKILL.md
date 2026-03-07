---
name: wox-plugin-creator
description: Create, scaffold, implement, and publish Wox plugins (nodejs, python, script-nodejs, script-python). Use when cloning official SDK templates, generating script plugin templates, editing plugin.json metadata, defining SettingDefinitions and validators, wiring i18n, implementing plugin APIs, or preparing plugins for publish.
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
- Script plugins are **single-file** plugins. Prefer filenames like `Wox.Plugin.Script.<Name>.<ext>` (e.g., `Wox.Plugin.Script.Memos.py`).
- For script plugins, the scaffold copies Wox script templates from `~/.wox/ai/skills/wox-plugin-creator/assets/script_plugin_templates/` and fills metadata placeholders.
- Prefer standard library features; avoid third-party dependencies unless absolutely necessary.
- For SDK usage and API details, read `references/sdk_nodejs.md` or `references/sdk_python.md`.
- For `plugin.json`, `SettingDefinitions`, validators, dynamic settings, and feature flags, read `references/plugin_json_schema.md` first.
- For ready-to-copy patterns such as validated textbox/select fields, editable tables, AI model selectors, and dynamic preview settings, read `references/settings_patterns.md`.
- For Python settings APIs, note that helper builders are limited; advanced settings are often created by constructing `PluginSettingDefinitionItem` and value objects directly.

### 2) Package and publish

- For SDK plugins cloned from templates, run `make publish` inside the template repo.
- Publishing notes: `references/publishing.md`.
- For publishing a plugin into the Wox store, read `references/store_publishing.md`.
- Script plugins do not use `plugin.json`; they embed a JSON metadata block in the script header comments.

## Resources

- scripts: `scripts/scaffold_wox_plugin.py`
- references: `references/plugin_overview.md`, `references/scaffold_nodejs.md`, `references/scaffold_python.md`, `references/sdk_nodejs.md`, `references/sdk_python.md`, `references/plugin_json_schema.md`, `references/settings_patterns.md`, `references/plugin_i18n.md`, `references/publishing.md`, `references/store_publishing.md`
- assets: `assets/script_plugin_templates/`
