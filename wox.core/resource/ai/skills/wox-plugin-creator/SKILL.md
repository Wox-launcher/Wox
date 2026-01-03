---
name: wox-plugin-creator
description: Create and scaffold Wox plugins (nodejs, python, script-nodejs, script-python). Use when cloning official SDK templates, generating script plugin templates, or preparing plugins for publish.
---

# Wox Plugin Creator

## Quick Start

- Scaffold a Node.js plugin (clones template repo):
  - `python3 scripts/scaffold_wox_plugin.py --type nodejs --output-dir ./MyPlugin`
- Scaffold a Python plugin (clones template repo):
  - `python3 scripts/scaffold_wox_plugin.py --type python --output-dir ./MyPlugin`
- Scaffold a script plugin (uses local templates; plugin-id auto-generated):
  - `python3 scripts/scaffold_wox_plugin.py --type script-nodejs --output-dir ./MyScript --name "My Script" --trigger-keywords my`

## Workflow

### 1) Scaffold plugin files

- Use `scripts/scaffold_wox_plugin.py` for `nodejs`, `python`, `script-nodejs`, or `script-python`.
- For Node.js and Python, the scaffold clones the official template repos and replaces placeholders like `{{.ID}}`, `{{.Name}}`, `{{.Description}}`, `{{.TriggerKeywordsJSON}}`, `{{.Author}}`.
- For script plugins, the scaffold copies Wox script templates from `~/.wox/ai/skills/wox-plugin-creator/assets/script_plugin_templates/` and fills metadata placeholders.
- For SDK usage and API details, read `references/sdk_nodejs.md` or `references/sdk_python.md`.

### 2) Package and publish

- For SDK plugins cloned from templates, run `make publish` inside the template repo.
- Publishing notes: `references/publishing.md`.

## Resources

- scripts: `scripts/scaffold_wox_plugin.py`, `scripts/package_plugin.py`
- references: `references/plugin_overview.md`, `references/scaffold_nodejs.md`, `references/scaffold_python.md`, `references/sdk_nodejs.md`, `references/sdk_python.md`, `references/plugin_json_schema.md`, `references/plugin_i18n.md`, `references/publishing.md`
- assets: `assets/script_plugin_templates/`
