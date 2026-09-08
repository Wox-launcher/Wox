---
name: wox-plugin-creator
description: Create, scaffold, implement, and package Wox plugins (nodejs, python, script-nodejs, script-python, singlefile-python, singlefile-nodejs). Use when cloning official SDK templates, generating script or single-file SDK plugin templates, editing plugin.json metadata, defining SettingDefinitions and validators, wiring i18n, implementing plugin APIs, QueryResponse refinements, refinement hotkeys, structured query slots, or preparing plugin repositories for local packaging. If the user wants to publish a plugin to the official Wox store or check whether it is already listed, prefer wox-plugin-submit2store.
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
- For static HTML previews, use `webview` with a JSON-encoded `html` field in `PreviewData`; see the HTML preview examples in those SDK references. There is no separate `html` preview type, and no local HTTP server is needed.
- For inline command arguments or atomic query blocks, read [QueryHint](#queryhint). Command declarations contain suffix templates; `ChangeQuery` contains a complete instance. Keep legacy text parsing when structure is absent.
- For query-scoped filters or sort controls, return `QueryResponse.Refinements` and read `references/refinements.md` before assigning hotkeys.
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
- Query refinements (type filters, sort modes, and similar query-scoped chips) belong on `QueryResponse.Refinements`, not in command syntax. Every refinement `Hotkey` must use the platform primary modifier: `cmd+<key>` on macOS and `ctrl+<key>` on Windows/Linux (for example `cmd+t` / `ctrl+t`). Detect the OS at runtime and emit the matching string.
- For ready-to-copy patterns such as validated textbox/select fields, editable tables, AI model selectors, and dynamic preview settings, read `references/settings_patterns.md`.
- For Python settings APIs, note that helper builders are limited; advanced settings are often created by constructing `PluginSettingDefinitionItem` and value objects directly.

### 2) Author result and action icons

- Read `references/icons.md` for icon selection, inline SVG patterns, and placement rules.
- Use `var(--wox-theme-icon-color)` for theme-adaptive SVG paints; see [SVG Theme Colors](references/icons.md#svg-theme-colors) for the black/white mapping and brand-color preservation rules.
- When the requested icon semantics already match a bundled generic icon under `assets/iconify/`, prefer reusing that local reference before searching for a new one.
- Use `scripts/search_iconify.py` to search Iconify collections and fetch ready-to-inline SVG constants for `icons.ts` or `icons.py`.
- Single-file SDK plugins cannot use relative image paths. Use emoji, URL, SVG, base64, or an absolute path.

### 3) Package and submit plugin

- For SDK plugins cloned from templates, run `make package` inside the template repo.
- For submitting a plugin to the official Wox store, prefer `wox-plugin-submit2store` skill.
- Script plugins do not use `plugin.json`; they embed a JSON metadata block in the script header comments.
- Single-file SDK plugins also embed JSON metadata in the file header. Keep `MinWoxVersion` as `"2.4.2"`. Store delivery uses a `.py` or `.js` download URL with `Runtime` `PYTHON` or `NODEJS`. Do not mix those suffixes with `SCRIPT` or `.wox`.

## QueryHint

`QueryHint` is optional semantic background guidance for `input` queries. It can
carry actual argument values, but must never turn continuous input into a mandatory
form. Preserve normal caret movement, cross-element selection, deletion, clipboard,
undo and IME behavior. Tab is optional. When editing invalidates a semantic boundary,
keep the user's text and discard unreliable metadata rather than blocking input.

- Declare suffix elements in `Commands[].QueryHint`; `Aliases` use the same trigger.
  Wox inserts the matched command prefix with reserved ID `command`. Static metadata
  and `RegisterQueryCommands` / `register_query_commands` use the same model.
- `ChangeQuery` always requires `QueryType` and complete `QueryText` for input,
  or complete `QuerySelection` for selection. `QueryHint` is optional visual
  enhancement, never a replacement for query content. An input hint must describe
  exactly the supplied text, including trigger keyword, command, separators and values.
  An invalid or mismatching hint is ignored; the supplied `QueryText` is used unchanged. Explicit `QueryText: ""` remains valid for clearing.
  Python uses `ChangeQueryParam(query_type=QueryType.INPUT, query_text="set volume 50",
  query_hint=QueryHint(elements=[...]))`; `QueryHint` and `QueryElement` are SDK exports.
- Elements have nonempty, unique `Id` values. `text` uses `Text` (including explicit
  separators); `argument` uses `Value`, optional `Placeholder` and `Required`;
  `block` uses atomic `Value`. A highlighted argument remains freely editable.
- Placeholders support `i18n:` and never enter the query value or clipboard.
  `Required` is descriptive: validate empty values and business constraints before
  offering or executing actions. Querying itself must not perform the action.
- Read values by ID from `query.QueryHint.Elements` (Node.js) or
  `query.query_hint.elements` (Python). Keep legacy `Search` / `search` parsing when
  the hint is absent; an empty argument is not an absent hint. Wox supplies a lossy
  plain-text projection in existing query fields; do not reconstruct boundaries from it.
- Complete commands preview hints; space or Tab activates the template. Whole-command
  paste into ordinary text is not parsed into arguments. Reopening selects the entire
  query for replacement; undo and history preserve hints when possible.
- Keep the list flat; no nested elements, dropdowns, custom rendering or inline markup.
  Hints never carry plugin identity or create a scope. Routing uses the normal query
  text and explicit `QueryScope`; complete instances must include the trigger keyword
  when needed (for example `gh issues `). If both text and a hint are passed to `ChangeQuery`,
  the explicit query text remains authoritative; the hint must match it.
- Use SDK or single-file SDK APIs; do not assume the limited script `change-query`
  action supports hints. Verify the first supporting Wox/SDK release before setting
  distribution requirements; the single-file runtime version floor alone is insufficient.
- Validate blank/valid/invalid arguments, multiple arguments, whole-query replacement
  after reopen, undo and the legacy path. Set Volume is the first built-in example.

Example command suffix (Wox adds the command text):

```json
{
  "Command": "set-volume",
  "Aliases": ["set volume", "volume"],
  "QueryHint": {
    "Elements": [
      { "Id": "volume", "Kind": "argument", "Placeholder": "Volume (0–100)", "Required": true }
    ]
  }
}
```

## Runtime Requirements

Wox enforces the same interpreter floors for SDK plugins, single-file SDK plugins, and script plugins. Store install fails, and queries show a setup result, when the machine is below these versions:

- **Python**: 3.10 or later
- **Node.js**: 20 or later

Do not target older interpreters. Script plugins still use the user's system Python or Node.js; they do not use a bundled runtime. Single-file SDK plugins run inside Wox's existing Python/Node runtime host and require **Wox 2.4.2 or later** (`MinWoxVersion`: `"2.4.2"`).

## Resources

- scripts: `scripts/scaffold_wox_plugin.py`, `scripts/search_iconify.py`
- references: `references/plugin_overview.md`, `references/scaffold_nodejs.md`, `references/scaffold_python.md`, `references/sdk_nodejs.md`, `references/sdk_python.md`, `references/plugin_json_schema.md`, `references/settings_patterns.md`, `references/plugin_i18n.md`, `references/icons.md`, `references/refinements.md`
- assets: `assets/script_plugin_templates/`, `assets/single_file_plugin_templates/`, `assets/iconify/`
