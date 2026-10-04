---
name: wox-plugin-creator
description: Create, scaffold, implement, and package Wox plugins (nodejs, python, singlefile-python, singlefile-nodejs). Use when cloning official SDK templates, generating single-file SDK plugin templates, editing plugin.json metadata, defining SettingDefinitions and validators, wiring i18n, implementing plugin APIs, QueryResponse refinements, refinement hotkeys, structured query slots, Plugin Tools (RegisterPluginTool / InvokePluginTool), or preparing plugin repositories for local packaging. If the user wants to publish a plugin to the official Wox store or check whether it is already listed, prefer wox-plugin-submit2store.
---

# Wox Plugin Creator

## Quick Start

- Scaffold a Node.js plugin (clones template repo):
  - `python3 scripts/scaffold_wox_plugin.py --type nodejs --output-dir ./MyPlugin --name "My Plugin" --trigger-keywords my`
- Scaffold a Python plugin (clones template repo):
  - `python3 scripts/scaffold_wox_plugin.py --type python --output-dir ./MyPlugin --name "My Plugin" --trigger-keywords my`
- Scaffold a single-file SDK plugin (uses local templates; plugin-id auto-generated; writes **one** file into the live user plugin directory so a running Wox instance can load it immediately). Omit `--output-dir`; default is `~/.wox/wox-user/plugins/single-file/` (Windows: `%USERPROFILE%\.wox\wox-user\plugins\single-file\`):
  - Auto-detect this machine: `python3 scripts/scaffold_wox_plugin.py --type singlefile --name "Weather" --trigger-keywords weather`
  - Explicit Node.js: `python3 scripts/scaffold_wox_plugin.py --type singlefile-nodejs --name "Weather" --trigger-keywords weather`
  - Explicit Python: `python3 scripts/scaffold_wox_plugin.py --type singlefile-python --name "Weather" --trigger-keywords weather`

## Choose a plugin type

- **Single-file SDK plugin**: one `.py` or CommonJS `.js` file with full Public API, loaded into the existing Python/Node runtime host. No extra process per query. Create and edit **only** `~/.wox/wox-user/plugins/single-file/Wox.Plugin.<Name>.js` (or `.py`); Wox watches that directory and reloads on save. Requires Wox 2.4.2+; header `MinWoxVersion` must be `"2.4.2"`.
- **SDK plugin (`.wox`)**: multi-file package with dependencies, resources, TypeScript, and `plugin.json`.

Node.js first version must stay CommonJS (`module.exports.plugin`) and must not import `@wox-launcher/wox-plugin`.

## Choose a runtime language

Honor an explicit Python or Node.js request from the user, including a `.py` or `.js` output filename.

When the user does not specify a language, detect this machine before scaffolding. Do not default to Python.

1. Run `python3 scripts/detect_local_runtime.py` (use `python` if `python3` is missing). It prints `nodejs`, `python`, or `none`.
2. If that script cannot run, check the floors yourself: `node --version` for Node.js 20+, and `python3 --version` for Python 3.10+. On Windows also try `py -3 --version` and `python --version`.
3. Decision:
   - Only one runtime meets the floor → use that runtime
   - Both meet the floor → use Node.js
   - Neither meets the floor → ask the user which language they want
4. Map the choice to `--type`: `singlefile-nodejs` / `singlefile-python`, or `nodejs` / `python`. `--type singlefile` applies the same detection inside the scaffold.

## Workflow

### 1) Scaffold plugin files

- Use `scripts/scaffold_wox_plugin.py` for `nodejs`, `python`, `singlefile`, `singlefile-python`, or `singlefile-nodejs`.
- Pass `--name` and `--trigger-keywords` for every runtime. The scaffold exits without them. `--output-dir` is required for packaged SDK plugins; omit it for single-file SDK plugins so the file lands in `~/.wox/wox-user/plugins/single-file/`.
- For Node.js and Python packages, the scaffold clones the official template repos and replaces placeholders like `{{.ID}}`, `{{.Name}}`, `{{.Description}}`, `{{.TriggerKeywordsJSON}}`, `{{.Author}}`.
- Before starting work in a new SDK plugin project, run `make init` in the project root when the project has not been initialized yet.
- Single-file SDK plugins are **single-file** host-loaded plugins. Prefer filenames like `Wox.Plugin.<Name>.js` or `Wox.Plugin.<Name>.py`.
- **Single-file SDK destination**: create and implement the plugin as one file in `~/.wox/wox-user/plugins/single-file/` (Windows: `%USERPROFILE%\.wox\wox-user\plugins\single-file\`). That directory is what a running Wox instance loads; saving reloads after about 500ms, so the user can query the trigger keyword immediately. Omit `--output-dir` when using the scaffold, or write the file there directly from `assets/single_file_plugin_templates/`. Do not also create a copy under the current repository (`plugins/`, workspace root, or a new folder). Pass `--output-dir` only when the user asks for a different path.
- Do not put companion files (`*.test.js`, README, extra modules) in the live directory. Wox loads every `.js` and `.py` there as a plugin.
- Single-file SDK plugins must set header `MinWoxVersion` to `"2.4.2"`. The scaffold applies this default when `--min-wox-version` is omitted. Do not lower it; Wox 2.4.2 is the first release that can load this plugin type, and store/CI reject older floors.
- For single-file SDK plugins, the scaffold copies templates from `~/.wox/ai/skills/wox-plugin-creator/assets/single_file_plugin_templates/` (or the repo `.agents/skills/wox-plugin-creator/assets/single_file_plugin_templates/` fallback).
- Prefer standard library features; avoid third-party dependencies unless absolutely necessary. Single-file SDK plugins cannot use pip/npm packages.
- For SDK usage and API details, read `references/sdk_nodejs.md` or `references/sdk_python.md`.
- For plugins declaring `querySelection`, return results only for selection types and content the plugin can process. Return an empty results list for unsupported, missing, or empty selection data instead of showing usage or help rows for unrelated selections.
- Keep a result on the row. `Title` is the name to scan. `SubTitle` is one short identity line, such as a code, place, or source. `Tails` are the few facts that must stay visible, such as a price and its change. Use at most three tail tags on one result. A fourth tag can be clipped, so part of a tag is not shown. Put any further fact in the subtitle, the copied text, or a preview. Do not add `Preview` for a quote, status, short record, or anything that fits on that row.
- A text tail already has a theme-aware rounded container. Use an SVG image tail only when that tag must also contain an icon. Match the launcher metrics below, and see [Simulated tail tags](#simulated-tail-tags).
- Refresh a visible row in place with `UpdateResult` / `update_result`. Remember the results returned by the latest `query()`. Replace that list on the next query. When a background refresh or an action changes a row that is still on screen, call `UpdateResult` with the same result `Id` and only the fields that changed (`Title`, `SubTitle`, `Icon`, `Tails`, `Actions`). The query text stays put and the list does not reload. Use `RefreshQuery` / `refresh_query` only when rows must be added or removed and `UpdateResult` cannot express that. Do not use `ChangeQuery` to redraw results.
- Add `Preview` only for a large body that cannot fit the row: a long document, many fields, a chart, a gallery, or syntax highlighting. When a preview is required, use `markdown` for prose, lists, links, and images. Use `text` or `image` when that is the whole preview. Use `webview` HTML only after markdown cannot express it, such as syntax highlighting, folding, or an interactive layout. HTML is the last option because the webview can steal query focus, miss launcher theme colors, and hit layout bugs. There is no separate `html` preview type; HTML uses `webview` with a JSON-encoded `html` field and no local HTTP server. See the HTML preview examples in the SDK references.
- Do not rasterize documents as SVG/`image` previews; those scale as pictures, cannot select text, and do not follow theme colors.
- When HTML is required, follow the current Wox theme. Call `GetThemeColors` / `get_theme_colors` (Wox >= 2.4.5) when building the HTML and paint opaque `Background`, `Text`, `SecondaryText`, `Border`, `Accent`, `AccentText`, and `Selection`. Use `Dark` to choose a light or dark syntax palette. Do not hardcode only a dark page, and do not rely on `transparent` or `prefers-color-scheme` as a substitute for the launcher palette. Include a theme color in `cacheKey` so the preview refreshes after a theme change. If the API is missing, fall back to a dark and a light default.
- For inline command arguments or atomic query blocks, read [QueryHint](#queryhint). Command declarations contain suffix templates; `ChangeQuery` contains a complete instance. Keep legacy text parsing when structure is absent.
- For query-scoped filters or sort controls, return `QueryResponse.Refinements` and read `references/refinements.md` before assigning hotkeys.
- For `plugin.json`, `SettingDefinitions`, `QueryRequirements`, validators, dynamic settings, and feature flags, read `references/plugin_json_schema.md` first.
- When implementing an SDK or single-file SDK plugin, also register Plugin Tools in `init()` for capabilities other plugins should be able to call. Query results and actions stay for the user; tools expose the same work as structured operations. Read `references/plugin_tools.md` first. Requires Wox >= 2.4.5 (`MinWoxVersion` `"2.4.5"` or newer). Do not declare tools in `plugin.json`.
- SDK and single-file SDK plugins should support MRU unless the plugin is clearly unsuitable. Declare the `mru` feature, put restore identity on action `ContextData`, and register `OnMRURestore` / `on_mru_restore` in `init()`. The restore callback must return immediately from memory or local cache. Wox waits 300ms for each start-page MRU restore, then discards that item, logs the timeout, and shows the next MRU item. Do not fetch, scan disk, or call host APIs inside the callback. Skip MRU only for context-dependent, one-shot, diagnostic, or inbox-style plugins, and say why in the implementation notes.
- Persist user settings through the Public API (`GetSetting` / `SaveSetting` / `SetSetting` / `OnSettingChanged`, or Python `get_setting` / `save_setting` / `set_setting` / `on_setting_changed`). A normal setting write is cloud-synced and follows the user across machines. Use these APIs for preferences, API keys, favorites, and account configuration.

### Cache stays out of the settings API

Anything the plugin can rebuild is cache: fetched JSON, subject or entity snapshots, search indexes, downloaded files, and thumbnails. Write that data under the plugin cache folder. Do not pass it to `SaveSetting`, `SetSetting`, `save_setting`, or `set_setting`.

Plugin settings are cloud-synced. Each settings write becomes a sync record and is copied to the user's other devices, including a hidden key that is absent from `SettingDefinitions`. A cache blob stored this way is uploaded on every refresh and fills sync history. `IsPlatformSpecific` still syncs; it only keeps a separate value per operating system. `IsLocal` / `is_local` is for a small machine-local preference that must remain in the settings store, such as a window position. It is not a place for cache.

- Call `GetCacheFolder(ctx)` / `get_cache_folder(ctx)` once in `init()`, keep the path, and write cache files under it.
- The folder is `~/.wox/cache/plugins/<plugin-id>/`. Wox creates it and deletes it when the plugin is uninstalled.
- Do not invent a `cache/`, `tmp/`, `downloads/`, or `data/` directory beside the plugin file, under user data, or under a hardcoded name.
- When authoring `SettingDefinitions`, always decide whether each setting is platform-specific before shipping it. Wox cloud sync replicates normal plugin settings across devices, so local paths, executable paths, shell commands, hotkeys, system integrations, browser profiles, and application paths should usually set `IsPlatformSpecific: true`. Account IDs, API keys, remote service hosts, and cross-platform user preferences should usually keep `IsPlatformSpecific: false`.
- Use `DisabledInPlatforms` only to disable a setting on selected platforms. It does not isolate stored values; use `IsPlatformSpecific` when the value must differ per platform after cloud sync.
- When a plugin cannot run a query without required settings such as access keys, declare those requirements in metadata `QueryRequirements` instead of returning ad hoc setup results from `query()`.
- Query refinements (type filters, sort modes, and similar query-scoped chips) belong on `QueryResponse.Refinements`, not in command syntax. Every refinement `Hotkey` must use the platform primary modifier: `cmd+<key>` on macOS and `ctrl+<key>` on Windows/Linux (for example `cmd+t` / `ctrl+t`). Detect the OS at runtime and emit the matching string.
- For ready-to-copy patterns such as validated textbox/select fields, editable tables, AI model selectors, and dynamic preview settings, read `references/settings_patterns.md`.
- For Python settings APIs, note that helper builders are limited; advanced settings are often created by constructing `PluginSettingDefinitionItem` and value objects directly.

### 2) Author result and action icons

- Read `references/icons.md` before choosing any glyph. Result-row and plugin-identity icons may be colorful; Action Panel leading icons must not.
- Scaffold templates use inline SVGs for the default plugin and result icons. Keep those defaults as SVG; use emoji or other formats only when deliberately choosing a result identity icon.
- Prefer a bundled monochrome verb from `assets/iconify/action/` (copy, open, execute/lightning, delete, edit, paste, add, search, settings). These SVGs already use `var(--wox-theme-icon-color)` so the Action Panel can tint them to the row label.
- Do not use emoji, brand logos, the plugin mark, or mixed-color result art as the leading action icon. The panel only tints SVGs that contain the theme variable; anything else stays authored and looks inconsistent next to system actions.
- Execute actions use the lightning verb (`action/execute.svg`), not a gear or play triangle. Settings actions use the gear.
- For a new action metaphor, fetch a monochrome Iconify outline with `scripts/search_iconify.py` (it rewrites `currentColor` to the theme variable by default). Use `--no-wox-theme` only for colorful **result** icons.
- Single-file SDK plugins cannot use relative image paths. Inline the SVG for actions; emoji/URL/base64 are acceptable for result identity only.

### 3) Package and submit plugin

- For SDK plugins cloned from templates, run `make package` inside the template repo.
- Single-file SDK plugins embed JSON metadata in the file header. Keep `MinWoxVersion` as `"2.4.2"`.
- **Publish a single-file SDK plugin** on a public GitHub gist. That is the simplest store host. Reference: https://gist.github.com/qianlifeng/04a9609de66eaa582a473f5852450ede
  1. Create one public gist file named `Wox.Plugin.<Name>.js` or `Wox.Plugin.<Name>.py`. The store `DownloadUrl` path must end with `.js` or `.py` (a gist URL without that suffix will not install). Example: `gh gist create --public --desc "Wox.Plugin.YouTube" Wox.Plugin.YouTube.js`
  2. Upload the screenshot and icon to GitHub (drag them onto the gist page or a gist comment). Copy the resulting `https://gist.github.com/user-attachments/assets/<id>` URLs, for example `https://gist.github.com/user-attachments/assets/7502acdc-1ea5-4ef6-a3fb-31875353dabe`.
  3. Then follow `wox-plugin-submit2store`: clone Wox and add a `store-plugin.json` entry. `Website` is the gist HTML URL, `DownloadUrl` is the gist raw URL including the filename (`https://gist.githubusercontent.com/<user>/<gist-id>/raw/Wox.Plugin.<Name>.js`), `IconUrl` and `ScreenshotUrls` are the user-attachments URLs. Runtime is `nodejs` or `python`. Do not ship a `.wox` for this plugin type.
- For packaged SDK plugins, use `wox-plugin-submit2store` with a GitHub repository and `.wox` release. Ask the user before submitting to the store.

## Simulated tail tags

Prefer a native `text` tail whenever the label is only text. Wox draws compact rounded rectangles and owns theme, selected-row colors, font rendering, and density scaling. Use at most three tails on one result; additional facts belong in the subtitle, copied text, or a preview.

Use an SVG `image` tail only when an icon and label must share one tag. Match the current native text-tail geometry; do not reuse the old pill shape or outlined capsule examples. At normal density, the logical dimensions are:

| Property | Value |
| --- | --- |
| Height | 22 |
| Corner radius | 4 (`rx="4"`), not half the height |
| Side inset | 7 on each side; text-only width is measured text width plus 14 |
| Font | 11, regular weight |
| Border | None; omit `stroke` |
| Default | Row foreground text; background uses the same RGB at alpha 13/255, or 24/255 when selected |
| Success | Fill `#027A48`, label `#FFFFFF` |
| Danger | Fill `#B42318`, label `#FFFFFF` |
| Warning | Fill `#B54708`, label `#FFFFFF` |

Use native `TextCategory` / `text_category` for semantic text tails. Status fills remain solid; ordinary metadata has a subtle background. Do not make every secondary fact a status badge just to add color.

For an icon-and-label SVG, width is `7 + icon width + gap + measured label width + 7`. Set `ImageWidth` to that width and `ImageHeight` to 22, or the launcher defaults to a 20px image slot. For example, a 90-unit tag starts with `<svg viewBox="0 0 90 22" ...><rect width="90" height="22" rx="4" fill="#027A48"/>...`, with no stroke. Keep the image-tail corners consistent with adjacent native tags.

Image tails keep their authored colors; Wox does not automatically apply text-tail selection or theme styling to them. For neutral metadata, prefer native text instead of hardcoding white text on a transparent SVG. If a custom image is necessary, use `GetThemeColors` / `get_theme_colors` to choose readable light/dark colors and include those colors in its cache key. Avoid claiming an SVG matches selected-row styling when the plugin has no selection state.

Draw side-by-side icon labels as filled glyph paths inside the SVG. The SVG rasterizer does not draw general `<text>`. One `<text text-anchor="middle">` whose `x` is the viewBox center is extracted and painted centered on the whole image; it is unsuitable for a label beside an icon because that shifts the intended spacing.

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
- Use SDK or single-file SDK APIs. Verify the first supporting Wox/SDK release before setting
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

Wox enforces the same interpreter floors for SDK plugins and single-file SDK plugins. Store install fails, and queries show a setup result, when the machine is below these versions:

- **Python**: 3.10 or later
- **Node.js**: 20 or later

Do not target older interpreters. Single-file SDK plugins run inside Wox's existing Python/Node runtime host and require **Wox 2.4.2 or later** (`MinWoxVersion`: `"2.4.2"`).

## Resources

- scripts: `scripts/scaffold_wox_plugin.py`, `scripts/detect_local_runtime.py`, `scripts/search_iconify.py`
- references: `references/plugin_overview.md`, `references/scaffold_nodejs.md`, `references/scaffold_python.md`, `references/sdk_nodejs.md`, `references/sdk_python.md`, `references/plugin_json_schema.md`, `references/settings_patterns.md`, `references/plugin_i18n.md`, `references/icons.md`, `references/refinements.md`, `references/plugin_tools.md`
- assets: `assets/single_file_plugin_templates/`, `assets/iconify/action/`
