# Full-featured Plugin Development Guide

Full-featured plugins run inside dedicated hosts (Python/Node.js) and talk to the Go core over WebSocket. They stay loaded, can keep state, and can use the full API surface (AI, previews, MRU, settings UI, deep links).

## Quickstart

- Create a folder under `~/.wox/plugins/<your-plugin-id>/`.
- Add `plugin.json` (see [Specification](./specification.md)) and your entry file (`main.py`, `index.js`, etc.).
- Install the SDK: `uv add wox-plugin` (Python ≥ 3.8) or `pnpm add @wox-launcher/wox-plugin` (Node.js ≥ 16).
- Restart Wox or disable/enable the plugin from settings to reload changes.

## Minimal examples

### Python

```python
from wox_plugin import Plugin, Query, Result, Context, PluginInitParams
from wox_plugin.models.image import WoxImage

class MyPlugin(Plugin):
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api
        self.plugin_dir = params.plugin_directory

    async def query(self, ctx: Context, query: Query) -> list[Result]:
        return [
            Result(
                title="Hello Wox",
                sub_title="This is a sample result",
                icon=WoxImage.new_emoji("👋"),
                score=100,
            )
        ]

plugin = MyPlugin()
```

### Node.js

```typescript
import { Plugin, Query, Result, Context, PluginInitParams } from "@wox-launcher/wox-plugin"

class MyPlugin implements Plugin {
  private api!: any
  private pluginDir = ""

  async init(ctx: Context, params: PluginInitParams): Promise<void> {
    this.api = params.API
    this.pluginDir = params.PluginDirectory
  }

  async query(ctx: Context, query: Query): Promise<Result[]> {
    return [
      {
        Title: "Hello Wox",
        SubTitle: "This is a sample result",
        Icon: { ImageType: "emoji", ImageData: "👋" },
        Score: 100,
      },
    ]
  }
}

export const plugin = new MyPlugin()
```

## plugin.json essentials

- Follow the [Specification](./specification.md) for fields, feature flags, and settings.
- Use `Runtime` = `PYTHON` or `NODEJS` and point `Entry` to your built file (TypeScript users build to JS).
- Add `Features` when you need selection queries, query env, AI, MRU, preview width control, or deep links.

Example:

```json
{
  "Id": "my-awesome-plugin",
  "Name": "My Awesome Plugin",
  "Description": "Do awesome things",
  "Author": "You",
  "Version": "1.0.0",
  "MinWoxVersion": "2.0.0",
  "Runtime": "NODEJS",
  "Entry": "dist/index.js",
  "TriggerKeywords": ["awesome", "ap"],
  "Features": [{ "Name": "querySelection" }, { "Name": "ai" }],
  "SettingDefinitions": [
    {
      "Type": "textbox",
      "Value": { "Key": "api_key", "Label": "API Key", "DefaultValue": "" }
    }
  ]
}
```

## Handling queries

- `Query.Type` can be `input` or `selection`. `selection` is delivered only when `querySelection` is enabled.
- `Query.Env` (active window title/pid/icon, active browser URL) is filled when `queryEnv` is enabled.
- Refer to [Query Model](./query-model.md) for how Wox splits `TriggerKeyword`, `Command`, and `Search`.

## Building results

- Use `Result` with optional `Preview` (markdown/text/image/url/file/remote), `Tails` (text or image badges), `Group`/`GroupScore`, and `Actions`.
- `ResultAction` supports `Hotkey`, `IsDefault`, `PreventHideAfterAction`, and custom `ContextData`.
- You can update existing items via `UpdateResult`/`UpdateResultAction` using `ActionContext` ids.
- Enable `resultPreviewWidthRatio` feature if you want a wider preview area for your plugin results.

## Settings

- Define UI with `SettingDefinitions` (textbox/checkbox/select/selectAIModel/table/dynamic/head/label/newline) in `plugin.json`.
- Values are provided in init params and can be read/written with `GetSetting`/`SaveSetting` (supports platform-specific storage).
- Dynamic settings can be replaced at runtime via API when you need to populate options from the plugin.

## AI, deep links, and MRU

- AI APIs require the `ai` feature; requests go through Wox-configured providers.
- Implement deep links only after adding the `deepLink` feature to metadata.
- For recency-based experiences, add the `mru` feature and implement `OnMRURestore` to hydrate results from stored MRU data.

## Testing locally

- Keep your plugin folder under `~/.wox/plugins/` (or symlink your development directory there).
- Toggle the plugin off/on in Wox settings or restart Wox after changing `plugin.json` or rebuilding your entry file.
- Use the SDK types to mock APIs in unit tests; avoid long-running operations in `query` (keep them async and cache results).
