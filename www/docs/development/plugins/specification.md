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
| `SupportedOS`        | ✅       | Any of `Windows`, `Linux`, `Macos`. Empty defaults to all for script plugins.               | `["Windows","Macos"]`                                     |
| `Features`           | ⭕       | Optional feature flags with parameters (see below)                                          | `[{"Name":"debounce","Params":{"IntervalMs":"200"}}]`     |
| `SettingDefinitions` | ⭕       | Settings schema rendered in Wox settings                                                    | `[...]`                                                   |

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
  "SupportedOS": ["Windows", "Linux", "Macos"],
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
- `resultPreviewWidthRatio` – control result list vs preview width. Params: `WidthRatio` between 0 and 1.
- `mru` – enable Most Recently Used support; implement `OnMRURestore` in your plugin.

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

`Style` supports `PaddingLeft/Top/Right/Bottom`, `Width`, and `LabelWidth`. Settings are provided to plugins as part of init parameters and (for script plugins) as `WOX_SETTING_<KEY>` environment variables.

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
        "Style": { "Width": 320, "LabelWidth": 90 }
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
import { Plugin, Context, PluginInitParams, PluginSettingDefinitionItem } from "@wox-launcher/wox-plugin"

class MyPlugin implements Plugin {
  private api: any

  async init(ctx: Context, params: PluginInitParams): Promise<void> {
    this.api = params.API

    await this.api.OnGetDynamicSetting(ctx, (key: string): PluginSettingDefinitionItem | null => {
      if (key !== "runtime_options") return null
      return {
        Type: "select",
        Value: {
          Key: "runtime_options",
          Label: "Runtime Options",
          DefaultValue: "a",
          Options: [
            { Label: "Option A", Value: "a" },
            { Label: "Option B", Value: "b" }
          ]
        }
      }
    })
  }
}
```

> Heads-up: dynamic settings are fetched on demand when the settings page is opened. Keep callbacks fast and deterministic; cache remote data if needed to avoid slowing the UI.
