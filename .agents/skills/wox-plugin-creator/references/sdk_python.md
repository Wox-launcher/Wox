# Wox Python Plugin SDK Reference

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

### Settings

- `get_setting(ctx, key)`: Get setting.
- `save_setting(ctx, key, value, is_platform_specific)`: Save setting.
- `on_setting_changed(ctx, callback)`: Listen for changes.
- `on_get_dynamic_setting(ctx, callback)`: Provide runtime-generated setting definitions for `dynamic` settings.

### UI Updates

- `update_result(ctx, result: UpdatableResult)`: Real-time update.
- `push_results(ctx, query, results)`: Append results.
- `refresh_query(ctx, param)`: Re-run query.
- `get_updatable_result(ctx, result_id)`: Get current result state.

### AI

- `ai_chat_stream(ctx, model, convs, options, callback)`: Stream LLM response.

### Internationalization (i18n)

- `get_translation(ctx, key)`: Get raw translated string.
  > **Note**: Returns raw string. Use f-strings or `.format()` for parameter substitution.

## Settings Authoring Notes

- The Python SDK exports helper builders for:
  - `create_textbox_setting()`
  - `create_checkbox_setting()`
  - `create_label_setting()`
- There is no built-in `create_select_setting()` helper today.
- For advanced settings such as `select`, `table`, validators, or `dynamic`, construct `PluginSettingDefinitionItem` and the corresponding value objects directly, or emit the expected JSON shape manually.
- For the exact `plugin.json` and validator shape, read `references/plugin_json_schema.md`.
- For ready-to-copy advanced settings examples, read `references/settings_patterns.md`.
- Use static `QueryRequirements` in `plugin.json` when a query requires settings such as API keys. Wox blocks the query before calling `query()` and shows the built-in `query_requirement_settings` setup preview.
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
    async def init(self, ctx, params): self.api = params.api

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
