# Wox Python Plugin SDK Reference

Wox requires **Python 3.10 or later**.

## Installation

`uv add wox-plugin`

## Key Classes

### Plugin Base Class

```python
from wox_plugin import Plugin, Query, QueryResponse, Result, Context, PluginInitParams

class MyPlugin(Plugin):
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api

    async def query(self, ctx: Context, query: Query) -> QueryResponse:
        return QueryResponse(results=[])
```

Return `QueryResponse` when `plugin.json` declares `MinWoxVersion` >= `2.0.4`.
Use `QueryResponse.layout.result_preview_width_ratio` and
`QueryResponse.layout.grid_layout` for query-scoped layout. The older
`resultPreviewWidthRatio` and `gridLayout` metadata features are deprecated
because they can only describe static plugin or command defaults.

### Data Models

```python
class Query:
    type: str  # "input" or "selection"
    raw_query: str
    trigger_keyword: str
    command: str
    search: str
    refinements: dict[str, str]  # selected refinement values

class QueryRefinement:
    id: str
    title: str
    type: QueryRefinementType  # singleSelect | multiSelect | toggle | sort
    hotkey: str  # cmd+t on macOS, ctrl+t on Windows/Linux
    options: list[QueryRefinementOption]
    default_value: list[str] = []
    persist: bool = False

class Result:
    title: str # Supports "i18n:key" prefix for auto-translation
    icon: WoxImage
    sub_title: str = "" # Supports "i18n:key" prefix
    actions: List[ResultAction] = []
    score: float = 0.0
    context_data: Any = None

class WoxImage:
    # Factory methods
    @classmethod
    def new_emoji(cls, char: str) -> "WoxImage"
    @classmethod
    def new_absolute(cls, path: str) -> "WoxImage"
    @classmethod
    def new_relative(cls, path: str) -> "WoxImage"
```

Action icons should be theme-adaptive SVG (`svg:` / inline markup with `var(--wox-theme-icon-color)`), not emoji. See `references/icons.md`.

Return refinements on `QueryResponse.refinements`. Read selected values from `query.refinements` on the next query. See `references/refinements.md`.

`hotkey` must be a real platform chord: `cmd+<key>` on macOS and `ctrl+<key>` on Windows/Linux. Detect `sys.platform == "darwin"` and emit the matching string. Do not write a literal `ctrl/cmd+t` token.

## Public API Methods

All methods are async and require `ctx`.

### General

- `change_query(ctx, query: PlainQuery)`: Update search bar.
- `hide_app(ctx)`: Hide Wox.
- `show_app(ctx)`: Show Wox.
- `notify(ctx, message)`: Show notification.
- `log(ctx, level, msg)`: Write log. Levels: `"Info"`, `"Error"`.
- `copy(ctx, params: CopyParams)`: Copy text/image.
- `is_visible(ctx)`: Check visibility.

### Cache

If the plugin needs on-disk cache, prefer `get_cache_folder` over any custom directory.

- `get_cache_folder(ctx)`: Return `~/.wox/cache/plugins/<plugin-id>/`. Wox creates it if needed and deletes it on uninstall.
- Call it once in `init()`, keep the path, and write downloads, thumbnails, and search-result files under it.
- Do not invent `cache/`, `tmp/`, or `downloads/` next to the plugin file, under user data, or under a hardcoded folder name.
- User preferences and favorites are settings, not cache. Use `get_setting` / `set_setting` for those.

### Theme

Requires Wox >= 2.4.5.

- `get_theme_colors(ctx, option=None)`: Opaque `#RRGGBB` launcher colors for HTML/webview previews: `background`, `text`, `secondary_text`, `border`, `accent`, `accent_text`, `selection`, plus `dark`. Call this only when building HTML previews so light and dark Wox themes stay in sync. Markdown previews follow the launcher theme without this API.

### Settings

Prefer these APIs for all plugin settings. Values stored here can sync across machines through Wox cloud sync. Do not persist ordinary settings in local files or a custom store.

- `get_setting(ctx, key)`: Get setting.
- `save_setting(ctx, key, value, is_platform_specific)`: Save setting. Normal plugin settings are eligible for cloud sync, so pass `True` for platform-only values such as local paths, executable paths, shell commands, hotkeys, browser profiles, application paths, and system integrations.
- `on_setting_changed(ctx, callback)`: Listen for changes.
- `on_get_dynamic_setting(ctx, callback)`: Provide runtime-generated setting definitions for `dynamic` settings.
- `on_mru_restore(ctx, callback)`: Rebuild a start-page result from stored `MRUData`. Declare the `mru` feature first. Return `None` when the item is stale. Put restore identity on action `ContextData` when building results. Return immediately from memory or local cache. Wox waits 300ms, then discards that start-page restore and shows the next MRU item. Do not fetch or call host APIs inside the callback.

### UI Updates

- `update_result(ctx, result: UpdatableResult)`: Update one visible result in place. Keep the same id. See `SKILL.md`.
- `push_results(ctx, query, results)`: Append results.
- `refresh_query(ctx, param)`: Re-run the current query. Use only when rows must be added or removed. See `SKILL.md`.
- `get_updatable_result(ctx, result_id)`: Get current result state.

### Plugin Tools

Runtime-registered callable operations. Requires Wox >= 2.4.5. See `references/plugin_tools.md`.

- `register_plugin_tool(ctx, option)`: Publish a tool owned by this plugin.
- `unregister_plugin_tool(ctx, option)`: Remove one of this plugin's tools.
- `list_plugin_tools(ctx, option)`: Snapshot of callable tools.
- `invoke_plugin_tool(ctx, option)`: Validate arguments, run another plugin's tool, validate output.

### AI

- `ai_chat_stream(ctx, model, convs, options, callback)`: Stream LLM response.

### Internationalization (i18n)

- `get_translation(ctx, key)`: Get raw translated string.
  > **Note**: Returns raw string. Use f-strings or `.format()` for parameter substitution.

## Settings Authoring Notes

- Prefer `get_setting`, `save_setting`, and `on_setting_changed` for plugin settings. These APIs participate in Wox cloud sync across machines. Avoid local files or custom persistence for values the user would expect to follow them to another device.
- If the plugin caches files, put them under `get_cache_folder(ctx)` first. Do not invent a cache directory under the plugin folder or user-data tree.
- The Python SDK exports helper builders for:
  - `create_textbox_setting()`
  - `create_checkbox_setting()`
  - `create_label_setting()`
- There is no built-in `create_select_setting()` helper today.
- For advanced settings such as `select`, `table`, validators, or `dynamic`, construct `PluginSettingDefinitionItem` and the corresponding value objects directly, or emit the expected JSON shape manually.
- For the exact `plugin.json` and validator shape, read `references/plugin_json_schema.md`.
- For ready-to-copy advanced settings examples, read `references/settings_patterns.md`.
- Match runtime `save_setting(ctx, key, value, is_platform_specific)` calls to the setting metadata. Do not hardcode `False` for dynamically saved settings if their `SettingDefinitions` entry uses `IsPlatformSpecific: true`.
- `DisabledInPlatforms` only controls where the setting is disabled; it does not isolate cloud-synced values.
- Use static `QueryRequirements` in `plugin.json` when a query requires settings such as API keys. Wox blocks the query before calling `query()` and shows only the built-in `query_requirement_settings` setup preview.
- There is no runtime `register_query_requirements` API. Declare query requirements in metadata.

## QueryRequirements Dataclasses

```python
from dataclasses import dataclass, field

@dataclass
class PluginQueryRequirement:
    setting_key: str
    validators: list[dict] = field(default_factory=list)
    message: str = ""

@dataclass
class PluginQueryRequirements:
    any_query: list[PluginQueryRequirement] = field(default_factory=list)
    query_without_command: list[PluginQueryRequirement] = field(default_factory=list)
    query_with_command: dict[str, list[PluginQueryRequirement]] = field(default_factory=dict)
```

Metadata example:

```json
{
  "SettingDefinitions": [
    {
      "Type": "textbox",
      "Value": {
        "Key": "accessKey",
        "Label": "i18n:access_key",
        "DefaultValue": "",
        "Validators": [{ "Type": "not_empty", "Value": {} }]
      }
    }
  ],
  "QueryRequirements": {
    "AnyQuery": [
      {
        "SettingKey": "accessKey",
        "Message": "i18n:access_key_required"
      }
    ],
    "QueryWithoutCommand": [],
    "QueryWithCommand": {}
  }
}
```

## Dynamic Setting Example

```python
from wox_plugin import (
    PluginSettingDefinitionItem,
    PluginSettingDefinitionType,
    PluginSettingValueLabel,
)

async def _on_get_dynamic_setting(ctx, key):
    if key == "separator_preview":
        return PluginSettingDefinitionItem(
            type=PluginSettingDefinitionType.LABEL,
            value=PluginSettingValueLabel(content="Preview: 1,234.56"),
        )

    return PluginSettingDefinitionItem(
        type=PluginSettingDefinitionType.LABEL,
        value=PluginSettingValueLabel(content="Unknown setting"),
    )
```

## Usage Example

```python
from wox_plugin import Plugin, Query, Result, WoxImage

class HelloPlugin(Plugin):
    async def init(self, ctx, params):
        self.api = params.api
        await self.api.on_mru_restore(ctx, self._on_mru_restore)

    async def _on_mru_restore(self, ctx, mru_data):
        item_id = (mru_data.context_data or {}).get("id")
        if not item_id:
            return None
        return Result(title=item_id, actions=[])

    async def query(self, ctx, query):
        # I18n with formatting
        raw_fmt = await self.api.get_translation(ctx, "hello_format") # "Hello {name}"
        title = raw_fmt.format(name=query.search)

        return [Result(
            title=title,
            icon=WoxImage.new_emoji("👋"),
            actions=[]
        )]

plugin = HelloPlugin()
```

## Static HTML preview

A result preview is only for a large body that does not fit the row. See `SKILL.md`. When a preview is required, prose, lists, links, and images belong in `markdown`. Use `WoxPreviewType.WEBVIEW` with a JSON-encoded `html` field only after markdown cannot express that preview. HTML is the last option because the webview can steal query focus, miss theme colors, and hit layout bugs. No HTTP server or temporary HTML file is needed; there is no separate `html` preview type.

Do not rasterize documents as SVG/`image` previews.

When HTML is required, paint the page from `get_theme_colors` so light and dark launcher themes stay in sync. Put a theme color in `cache_key`.

```python
import json
from wox_plugin import WoxPreview, WoxPreviewType

colors = await self.api.get_theme_colors(ctx)
preview = WoxPreview(
    preview_type=WoxPreviewType.WEBVIEW,
    preview_data=json.dumps({
        "html": f'<!doctype html><html><body style="background:{colors.background};color:{colors.text}">Hello Wox</body></html>',
        "cacheKey": f"hello:{colors.background}",
    }),
)
# Assign preview to Result(preview=preview, ...).
```

Set either `html` or `url`. Optional JSON fields are `injectCss`, `userAgent`, `cacheDisabled`, and `cacheKey` (defaults to the URL or HTML). Inline HTML has no plugin-relative base URL: embed CSS/images or use absolute resource URLs. This is browser content, not sanitized Markdown; use `html.escape` for untrusted text before interpolation.

## Structured query arguments and blocks

See [QueryHint](../SKILL.md#queryhint) for `Commands[].Aliases`, suffix
templates in `Commands[].QueryHint`, complete `ChangeQuery` instances, and
SDK usage. `QueryHint` remains optional for `input` queries; do not
embed markup in `QueryText` or infer argument boundaries from pasted text.
