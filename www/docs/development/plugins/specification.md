# Plugin Specification

`plugin.json` sits at the root of every full-featured plugin (and the same schema is embedded in script-plugin comments). Wox reads it to decide whether the plugin can load on the current platform, which runtime/entry file to run, and how to register trigger keywords and commands.

## plugin.json fields

| Field                | Required | Description                                                                                 | Example                                                   |
| -------------------- | -------- | ------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| `Id`                 | ✅       | Stable unique id (UUID recommended)                                                         | `"cea0f...28855"`                                         |
| `Name`               | ✅       | Display name                                                                                | `"Calculator"`                                            |
| `Description`        | ✅       | Short summary for the store and settings UI                                                 | `"Calculate simple expressions"`                          |
| `Author`             | ✅       | Author name                                                                                 | `"Wox Team"`                                              |
| `Version`            | ✅       | Plugin semantic version (`MAJOR.MINOR.PATCH`)                                               | `"1.0.0"`                                                 |
| `MinWoxVersion`      | ✅       | Minimum Wox version required                                                                | `"2.0.0"`                                                 |
| `Website`            | ⭕       | Homepage/repo link                                                                          | `"https://github.com/Wox-launcher/Wox"`                   |
| `Runtime`            | ✅       | `PYTHON`, `NODEJS`, `SCRIPT` (Go is reserved for system plugins)                            | `"PYTHON"`                                                |
| `Entry`              | ✅       | Entry file relative to plugin root. For script plugins this is filled automatically by Wox. | `"main.py"`                                               |
| `Icon`               | ✅       | [WoxImage](#icon-formats) string (emoji/base64/relative path)                               | `"emoji:🧮"`                                              |
| `TriggerKeywords`    | ✅       | One or more trigger keywords. Use `"*"` for global trigger.                                 | `["calc"]`                                                |
| `Commands`           | ⭕       | Optional commands (see [Query Model](./query-model.md))                                     | `[{"Command":"install","Description":"Install plugins"}]` |
| `SupportedOS`        | ✅       | Any of `Windows`, `Linux`, `Darwin`. Empty defaults to all for script plugins.              | `["Windows","Darwin"]`                                    |
| `Features`           | ⭕       | Optional feature flags with parameters (see below)                                          | `[{"Name":"debounce","Params":{"IntervalMs":"200"}}]`     |
| `SettingDefinitions` | ⭕       | Settings schema rendered in Wox settings                                                    | `[...]`                                                   |
| `I18n`               | ⭕       | Inline translations (see [Internationalization](#internationalization))                     | `{"en_US":{"key":"value"}}`                               |

### Icon formats

`Icon` uses the `WoxImage` string format:

- `emoji:🧮`
- `data:image/png;base64,<...>` or bare base64 (png assumed)
- `relative/path/to/icon.png` (resolved relative to the plugin folder)
- Absolute paths are accepted but avoid them for portable plugins.

### Example plugin.json

```json
{
  "Id": "cea0fdfc6d3b4085823d60dc76f28855",
  "Name": "Calculator",
  "Description": "Quick math in the launcher",
  "Author": "Wox Team",
  "Version": "1.0.0",
  "MinWoxVersion": "2.0.0",
  "Runtime": "PYTHON",
  "Entry": "main.py",
  "Icon": "emoji:🧮",
  "TriggerKeywords": ["calc"],
  "SupportedOS": ["Windows", "Darwin", "Linux"],
  "Features": [{ "Name": "debounce", "Params": { "IntervalMs": "250" } }, { "Name": "ai" }],
  "SettingDefinitions": [
    {
      "Type": "textbox",
      "Value": {
        "Key": "api_key",
        "Label": "API Key",
        "Tooltip": "Get it from your provider",
        "DefaultValue": ""
      }
    }
  ]
}
```

## Feature flags

Add items to `Features` when your plugin needs extra capabilities:

- `querySelection` – receive selection/drag/drop queries (`QueryTypeSelection`).
- `debounce` – avoid flooding `query` while the user types. Params: `IntervalMs` (string ms).
- `ignoreAutoScore` – opt out of Wox frequency-based auto scoring.
- `queryEnv` – request query environment data. Params: `requireActiveWindowName`, `requireActiveWindowPid`, `requireActiveWindowIcon`, `requireActiveBrowserUrl` (`"true"`/`"false"`).
- `ai` – allow usage of AI APIs from plugins.
- `deepLink` – enables custom deep links exposed by the plugin.
- `resultPreviewWidthRatio` – deprecated. Use `QueryResponse.Layout.ResultPreviewWidthRatio` instead for query-scoped preview width control.
- `mru` – enable Most Recently Used support; implement `OnMRURestore` in your plugin.
- `gridLayout` – deprecated. Use `QueryResponse.Layout.GridLayout` instead for query-scoped grid presentation. See [Grid Layout](#grid-layout) for compatibility details.

## SettingDefinitions

Settings are rendered in the Wox settings UI and passed to the plugin host:

| Type            | Description                                                    | Keys                                                                             |
| --------------- | -------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `head`          | Section header                                                 | `Content`                                                                        |
| `label`         | Read-only text                                                 | `Content`, `Tooltip`, optional `Style`                                           |
| `textbox`       | Single/multi-line text                                         | `Key`, `Label`, `Suffix`, `DefaultValue`, `Tooltip`, `MaxLines`, `Style`         |
| `checkbox`      | Boolean                                                        | `Key`, `Label`, `DefaultValue`, `Tooltip`, `Style`                               |
| `select`        | Dropdown                                                       | `Key`, `Label`, `DefaultValue`, `Options[] { Label, Value }`, `Tooltip`, `Style` |
| `selectAIModel` | Dropdown of available AI models (populated dynamically by Wox) | `Key`, `Label`, `DefaultValue`, `Tooltip`, `Style`                               |
| `table`         | Editable table rows                                            | `Key`, `Columns`, `DefaultValue`, `Tooltip`, `Style`                             |
| `dynamic`       | Placeholder that will be filled by the plugin via API          | `Key` only                                                                       |
| `newline`       | Visual separator                                               | (no value)                                                                       |

`Style` supports `PaddingLeft/Top/Right/Bottom` and `Width`. Settings are provided to plugins as part of init parameters and (for script plugins) as `WOX_SETTING_<KEY>` environment variables.

### SettingDefinitions examples

Minimal layout with AI model pick:

```json
{
  "SettingDefinitions": [
    { "Type": "head", "Value": "API" },
    {
      "Type": "textbox",
      "Value": {
        "Key": "api_key",
        "Label": "API Key",
        "Tooltip": "Get it from your provider",
        "DefaultValue": "",
        "Style": { "Width": 320 }
      }
    },
    {
      "Type": "selectAIModel",
      "Value": {
        "Key": "model",
        "Label": "Model",
        "DefaultValue": "",
        "Tooltip": "Use a configured AI provider"
      }
    },
    { "Type": "newline" }
  ]
}
```

Table + dynamic setting populated at runtime:

```json
{
  "SettingDefinitions": [
    { "Type": "head", "Value": "Mappings" },
    {
      "Type": "table",
      "Value": {
        "Key": "rules",
        "Tooltip": "Key/value rules",
        "Columns": [
          { "Title": "Key", "Width": 150 },
          { "Title": "Value", "Width": 240 }
        ],
        "DefaultValue": [
          ["foo", "bar"],
          ["hello", "world"]
        ]
      }
    },
    {
      "Type": "dynamic",
      "Value": {
        "Key": "runtime_options"
      }
    }
  ]
}
```

How values reach your plugin:

- Full-featured plugins: read/write with `GetSetting` + `SaveSetting` in the host SDK. Provide `dynamic` content via the SDK’s dynamic setting callback.
- Script plugins: Wox exports each key as `WOX_SETTING_<UPPER_SNAKE_KEY>` environment variables.

#### Dynamic setting callback (backend wiring)

Python (wox-plugin):

```python
from wox_plugin import Plugin, Context, PluginInitParams
from wox_plugin.models.setting import PluginSettingDefinitionItem, PluginSettingDefinitionType, PluginSettingValueSelect

class MyPlugin(Plugin):
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api

        async def get_dynamic(key: str) -> PluginSettingDefinitionItem:
            if key == "runtime_options":
                return PluginSettingDefinitionItem(
                    type=PluginSettingDefinitionType.SELECT,
                    value=PluginSettingValueSelect(
                        key="runtime_options",
                        label="Runtime Options",
                        default_value="a",
                        options=[
                            {"Label": "Option A", "Value": "a"},
                            {"Label": "Option B", "Value": "b"},
                        ],
                    ),
                )
            return None  # Unknown key

        await self.api.on_get_dynamic_setting(ctx, get_dynamic)
```

Node.js (SDK types):

```typescript
import { Plugin, Context, PluginInitParams, PluginSettingDefinitionItem } from "@wox-launcher/wox-plugin";

class MyPlugin implements Plugin {
  private api: any;

  async init(ctx: Context, params: PluginInitParams): Promise<void> {
    this.api = params.API;

    await this.api.OnGetDynamicSetting(ctx, (key: string): PluginSettingDefinitionItem | null => {
      if (key !== "runtime_options") return null;
      return {
        Type: "select",
        Value: {
          Key: "runtime_options",
          Label: "Runtime Options",
          DefaultValue: "a",
          Options: [
            { Label: "Option A", Value: "a" },
            { Label: "Option B", Value: "b" },
          ],
        },
      };
    });
  }
}
```

> Heads-up: dynamic settings are fetched on demand when the settings page is opened. Keep callbacks fast and deterministic; cache remote data if needed to avoid slowing the UI.

## Internationalization

Wox supports plugin internationalization (i18n) so your plugin can display text in the user's preferred language. There are two ways to provide translations:

### Method 1: Inline I18n in plugin.json (Recommended for Script Plugins)

Define translations directly in `plugin.json` using the `I18n` field. This is especially useful for script plugins that don't have a directory structure:

```json
{
  "Id": "my-plugin-id",
  "Name": "My Plugin",
  "Description": "i18n:plugin_description",
  "TriggerKeywords": ["mp"],
  "I18n": {
    "en_US": {
      "plugin_description": "A useful plugin",
      "result_title": "Result: {0}",
      "action_copy": "Copy to clipboard"
    },
    "zh_CN": {
      "plugin_description": "一个有用的插件",
      "result_title": "结果: {0}",
      "action_copy": "复制到剪贴板"
    }
  }
}
```

### Method 2: Language Files (Recommended for Full-Featured Plugins)

Create a `lang/` directory in your plugin root with JSON files named by language code:

```
my-plugin/
├── plugin.json
├── main.py
└── lang/
    ├── en_US.json
    └── zh_CN.json
```

Each language file contains a flat key-value map:

```json
// lang/en_US.json
{
  "plugin_description": "A useful plugin",
  "result_title": "Result: {0}",
  "action_copy": "Copy to clipboard"
}
```

```json
// lang/zh_CN.json
{
  "plugin_description": "一个有用的插件",
  "result_title": "结果: {0}",
  "action_copy": "复制到剪贴板"
}
```

### Using Translations

To use a translation, prefix your text with `i18n:` followed by the key:

```python
# Python example
result = Result(
    title="i18n:result_title",
    sub_title="i18n:action_copy"
)

# Or use the API to get translated text programmatically
translated = await api.get_translation(ctx, "i18n:result_title")
```

```typescript
// Node.js example
const result: Result = {
  Title: "i18n:result_title",
  SubTitle: "i18n:action_copy",
};

// Or use the API
const translated = await api.GetTranslation(ctx, "i18n:result_title");
```

### Translation Priority

Wox looks up translations in this order:

1. Inline `I18n` in plugin.json (current language)
2. `lang/{current_lang}.json` file
3. Inline `I18n` in plugin.json (en_US fallback)
4. `lang/en_US.json` file (fallback)
5. Return the original key if no translation found

### Supported Languages

| Code    | Language             |
| ------- | -------------------- |
| `en_US` | English (US)         |
| `zh_CN` | Chinese (Simplified) |
| `pt_BR` | Portuguese (Brazil)  |
| `ru_RU` | Russian              |

> Tip: Always provide `en_US` translations as the fallback language.

## Result Preview Width Ratio

> Deprecated: prefer `QueryResponse.Layout.ResultPreviewWidthRatio`. The
> metadata feature is still supported for existing plugins, but it can only
> express static plugin or command defaults. QueryResponse can choose a preview
> width for each query result set.

The `resultPreviewWidthRatio` feature controls how much of the launcher width is reserved for the result list when a preview panel is visible. `WidthRatio` must be between `0` and `1`; for example, `0.3` gives 30% to results and 70% to preview.

Use `Commands` when only some commands need a different layout. Empty `Commands` applies the ratio to every command, `["preview"]` applies it only to that command, and `["!preview"]` applies it to every command except `preview`. `WidthRatio: 0.0` hides the result list and is useful for preview-only commands such as Quick Look style file previews.

```json
{
  "Features": [
    {
      "Name": "resultPreviewWidthRatio",
      "Params": {
        "WidthRatio": 0.0,
        "Commands": ["preview"]
      }
    }
  ]
}
```

## Grid Layout

> Deprecated: prefer `QueryResponse.Layout.GridLayout`. The metadata feature is
> still supported for existing plugins, but it can only express static plugin or
> command defaults. QueryResponse can choose grid presentation for each query
> result set.

The `gridLayout` feature enables displaying results in a grid format instead of the default vertical list. This is ideal for plugins that display visual items such as emoji, icons, colors, or image thumbnails.

### Compatibility Configuration

Existing plugins may still add the `gridLayout` feature to `plugin.json`:

```json
{
  "Features": [
    {
      "Name": "gridLayout",
      "Params": {
        "Columns": "8",
        "ShowTitle": "false",
        "ItemPadding": "12",
        "ItemMargin": "6"
      }
    }
  ]
}
```

### Parameters

| Parameter     | Type   | Default   | Description                                                        |
| ------------- | ------ | --------- | ------------------------------------------------------------------ |
| `Columns`     | string | `"8"`     | Number of columns per row                                          |
| `ShowTitle`   | string | `"false"` | Whether to show title text below each icon (`"true"` or `"false"`) |
| `ItemPadding` | string | `"12"`    | Padding inside each grid item (in pixels)                          |
| `ItemMargin`  | string | `"6"`     | Margin around each grid item (in pixels)                           |

### Result Structure

When using grid layout, each result should have:

- **Icon**: The main visual element displayed in the grid cell (required)
- **Title**: Shown below the icon if `ShowTitle` is `"true"` (truncated with ellipsis if too long)
- **Group**: Optional grouping to organize items into sections with headers

### Example: Emoji Picker Plugin

```json
{
  "Id": "emoji-picker-plugin",
  "Name": "Emoji Picker",
  "MinWoxVersion": "2.0.4",
  "TriggerKeywords": ["emoji"],
  "Features": [
    {
      "Name": "gridLayout",
      "Params": {
        "Columns": "12",
        "ShowTitle": "false",
        "ItemPadding": "12",
        "ItemMargin": "6"
      }
    }
  ]
}
```

```python
from wox_plugin import Plugin, Context, Query, QueryResponse, Result

class EmojiPlugin(Plugin):
    async def query(self, ctx: Context, query: Query) -> QueryResponse:
        emojis = ["😀", "😃", "😄", "😁", "😅", "😂", "🤣", "😊"]
        return QueryResponse(results=[
            Result(
                title=emoji,
                icon=f"emoji:{emoji}",
                group="Smileys"
            )
            for emoji in emojis
        ])
```

Returning `list[Result]` is deprecated. The Python host still accepts it for compatibility with older Wox releases. Use `QueryResponse` only when `plugin.json` declares `MinWoxVersion` >= `2.0.4`.

### Grouping Items

Use the `group` field to organize grid items into sections. Items with the same group value will be displayed together under a group header:

```python
results = [
    Result(title="😀", icon="emoji:😀", group="Smileys"),
    Result(title="😃", icon="emoji:😃", group="Smileys"),
    Result(title="❤️", icon="emoji:❤️", group="Hearts"),
    Result(title="💙", icon="emoji:💙", group="Hearts"),
]
```

This produces a layout with "Smileys" and "Hearts" section headers, each followed by their respective emoji in a grid.

### Layout Calculation

The grid automatically calculates item sizes based on:

1. Available width divided by number of columns = cell width
2. Icon size = cell width - (ItemPadding + ItemMargin) × 2
3. Cell height = cell width + title height (if ShowTitle is enabled)

Adjust `ItemPadding` and `ItemMargin` to control spacing between items. Larger values create more breathing room; smaller values fit more items on screen.
