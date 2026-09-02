---
name: wox-plugin-creator
description: Create, scaffold, implement, and package Wox plugins (nodejs, python, script-nodejs, script-python, singlefile-python, singlefile-nodejs). Use when cloning official SDK templates, generating script or single-file SDK plugin templates, editing plugin.json metadata, defining SettingDefinitions and validators, wiring i18n, implementing plugin APIs, or preparing plugin repositories for local packaging. If the user wants to publish a plugin to the official Wox store or check whether it is already listed, prefer wox-plugin-submit2store.
---

# Wox Plugin Creator

## Quick Start

- Scaffold a Node.js plugin (clones template repo):
  - `python3 scripts/scaffold_wox_plugin.py --type nodejs --output-dir ./MyPlugin --name "My Plugin" --trigger-keywords my`
- Scaffold a Python plugin (clones template repo):
  - `python3 scripts/scaffold_wox_plugin.py --type python --output-dir ./MyPlugin --name "My Plugin" --trigger-keywords my`
- Scaffold a single-file SDK plugin (uses local templates; plugin-id auto-generated; single file output):
  - `python3 scripts/scaffold_wox_plugin.py --type singlefile-python --output-dir ./Wox.Plugin.Weather.py --name "Weather" --trigger-keywords weather`
  - `python3 scripts/scaffold_wox_plugin.py --type singlefile-nodejs --output-dir ./Wox.Plugin.Weather.js --name "Weather" --trigger-keywords weather`
- Scaffold a script plugin (uses local templates; plugin-id auto-generated; single file output):
  - `python3 scripts/scaffold_wox_plugin.py --type script-nodejs --output-dir ./Wox.Plugin.Script.MyScript.js --name "My Script" --trigger-keywords my`

## Choose a plugin type

- **Script plugin**: one-shot shell/command wrapper. Wox starts a process per query over stdin/stdout JSON-RPC. Limited Public API.
- **Single-file SDK plugin**: one `.py` or CommonJS `.js` file with full Public API, loaded into the existing Python/Node runtime host. No extra process per query. Save reloads the plugin. Requires Wox 2.4.2+; header `MinWoxVersion` must be `"2.4.2"`.
- **SDK plugin (`.wox`)**: multi-file package with dependencies, resources, TypeScript, and `plugin.json`.

Single-file Python is the fastest path to a Python SDK plugin. Node.js first version must stay CommonJS (`module.exports.plugin`) and must not import `@wox-launcher/wox-plugin`.

## Workflow

### 1) Scaffold plugin files

- Use `scripts/scaffold_wox_plugin.py` for `nodejs`, `python`, `script-nodejs`, `script-python`, `singlefile-python`, or `singlefile-nodejs`.
- Pass `--name` and `--trigger-keywords` for every runtime. The scaffold exits without them.
- For Node.js and Python packages, the scaffold clones the official template repos and replaces placeholders like `{{.ID}}`, `{{.Name}}`, `{{.Description}}`, `{{.TriggerKeywordsJSON}}`, `{{.Author}}`.
- Before starting work in a new SDK plugin project, run `make init` in the project root when the project has not been initialized yet.
- Script plugins are **single-file** process-per-query plugins. Prefer filenames like `Wox.Plugin.Script.<Name>.<ext>` (e.g., `Wox.Plugin.Script.Memos.py`).
- Single-file SDK plugins are **single-file** host-loaded plugins. Prefer filenames like `Wox.Plugin.<Name>.py` or `Wox.Plugin.<Name>.js`.
- Single-file SDK plugins must set header `MinWoxVersion` to `"2.4.2"`. The scaffold applies this default when `--min-wox-version` is omitted. Do not lower it; Wox 2.4.2 is the first release that can load this plugin type, and store/CI reject older floors.
- For script plugins, the scaffold copies Wox script templates from `~/.wox/ai/skills/wox-plugin-creator/assets/script_plugin_templates/` and fills metadata placeholders.
- For single-file SDK plugins, the scaffold copies templates from `~/.wox/ai/skills/wox-plugin-creator/assets/single_file_plugin_templates/` (or the repo `.agents/skills/wox-plugin-creator/assets/single_file_plugin_templates/` fallback).
- Prefer standard library features; avoid third-party dependencies unless absolutely necessary. Single-file SDK plugins cannot use pip/npm packages.
- For SDK usage and API details, read `references/sdk_nodejs.md` or `references/sdk_python.md`.
- For `plugin.json`, `SettingDefinitions`, `QueryRequirements`, validators, dynamic settings, and feature flags, read `references/plugin_json_schema.md` first.
- SDK and single-file SDK plugins must persist and read settings through the Public API setting methods (`GetSetting` / `SaveSetting` / `OnSettingChanged`, or Python `get_setting` / `save_setting` / `on_setting_changed`). These values participate in Wox cloud sync and can follow the user across machines. Do not store plugin settings in local files, custom JSON, or other side storage unless the value is truly machine-local and cannot live in settings.

### Cache files first: use the plugin cache folder

If a plugin needs to cache anything on disk, put it under the Wox plugin cache folder. Do this before inventing a local `cache/`, `tmp/`, `downloads/`, or `data/` directory.

- SDK and single-file SDK: call `GetCacheFolder(ctx)` / `get_cache_folder(ctx)` in `init()`, keep the path, then write files under it (`downloads/`, `thumbs/`, query JSON, and so on).
- Script plugins: use `WOX_DIRECTORY_PLUGIN_CACHE`. It is the same `~/.wox/cache/plugins/<plugin-id>/` folder.
- Wox creates the folder if needed and deletes it when the plugin is uninstalled. That is why cache must live here, not beside the plugin file, not under user data, and not under a hardcoded name such as `gifbox-script-plugin`.
- Settings are not cache. User preferences, API keys, and favorites go through the setting APIs so they can sync. Downloads, thumbnails, and search-result files go in the cache folder.
- When authoring `SettingDefinitions`, always decide whether each setting is platform-specific before shipping it. Wox cloud sync replicates normal plugin settings across devices, so local paths, executable paths, shell commands, hotkeys, system integrations, browser profiles, and application paths should usually set `IsPlatformSpecific: true`. Account IDs, API keys, remote service hosts, and cross-platform user preferences should usually keep `IsPlatformSpecific: false`.
- Use `DisabledInPlatforms` only to disable a setting on selected platforms. It does not isolate stored values; use `IsPlatformSpecific` when the value must differ per platform after cloud sync.
- When a plugin cannot run a query without required settings such as access keys, declare those requirements in metadata `QueryRequirements` instead of returning ad hoc setup results from `query()`.
- For ready-to-copy patterns such as validated textbox/select fields, editable tables, AI model selectors, and dynamic preview settings, read `references/settings_patterns.md`.
- For Python settings APIs, note that helper builders are limited; advanced settings are often created by constructing `PluginSettingDefinitionItem` and value objects directly.

### 2) Author result and action icons

- Read `references/icons.md` for icon selection, inline SVG patterns, and placement rules.
- When the requested icon semantics already match a bundled generic icon under `assets/iconify/`, prefer reusing that local reference before searching for a new one.
- Use `scripts/search_iconify.py` to search Iconify collections and fetch ready-to-inline SVG constants for `icons.ts` or `icons.py`.
- Single-file SDK plugins cannot use relative image paths. Use emoji, URL, SVG, base64, or an absolute path.

### 3) Package and submit plugin

- For SDK plugins cloned from templates, run `make package` inside the template repo.
- For submitting a plugin to the official Wox store, prefer `wox-plugin-submit2store` skill.
- Script plugins do not use `plugin.json`; they embed a JSON metadata block in the script header comments.
- Single-file SDK plugins also embed JSON metadata in the file header. Keep `MinWoxVersion` as `"2.4.2"`. Store delivery uses a `.py` or `.js` download URL with `Runtime` `PYTHON` or `NODEJS`. Do not mix those suffixes with `SCRIPT` or `.wox`.

## Runtime Requirements

Wox enforces the same interpreter floors for SDK plugins, single-file SDK plugins, and script plugins. Store install fails, and queries show a setup result, when the machine is below these versions:

- **Python**: 3.10 or later
- **Node.js**: 20 or later

Do not target older interpreters. Script plugins still use the user's system Python or Node.js; they do not use a bundled runtime. Single-file SDK plugins run inside Wox's existing Python/Node runtime host and require **Wox 2.4.2 or later** (`MinWoxVersion`: `"2.4.2"`).

## Resources

- scripts: `scripts/scaffold_wox_plugin.py`, `scripts/search_iconify.py`
- references: `references/plugin_overview.md`, `references/scaffold_nodejs.md`, `references/scaffold_python.md`, `references/sdk_nodejs.md`, `references/sdk_python.md`, `references/plugin_json_schema.md`, `references/settings_patterns.md`, `references/plugin_i18n.md`, `references/icons.md`
- assets: `assets/script_plugin_templates/`, `assets/single_file_plugin_templates/`, `assets/iconify/`
