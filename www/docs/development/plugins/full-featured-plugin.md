# Full-featured Plugin Development Guide

Full-featured plugins run inside dedicated hosts (Python or Node.js) and talk to `wox.core` over WebSocket. They stay loaded, can keep state, and can use the richer Wox API surface such as previews, settings UI, toolbar messages, MRU restore, screenshot capture, AI streaming, and deep links.

## When to choose a full-featured plugin

Choose this model when your plugin needs one or more of these:

- persistent state across queries
- async/network-heavy work
- custom settings UI
- richer previews and actions
- plugin-driven screenshot or clipboard workflows
- AI or MRU integration

If your plugin is a small one-file automation script, start with the [Script Plugin](./script-plugin.md) guide instead.

## Quickstart

1. Create a folder under `~/.wox/plugins/<your-plugin-id>/`
2. Add `plugin.json` and your entry file (`main.py`, `index.js`, or built output such as `dist/index.js`)
3. Install the SDK
4. Reload the plugin from Wox settings or restart Wox

SDK install commands:

- Python: `uv add wox-plugin`
- Node.js: `pnpm add @wox-launcher/wox-plugin`

If you use Codex, see [AI Skills For Plugin Development](./ai-skills.md).

## Minimal examples

These examples return `QueryResponse`, so the plugin's `plugin.json` must set
`MinWoxVersion` to `2.0.4` or newer. Return `list[Result]` or `Result[]`
directly if the same plugin build must run on older Wox releases.

### Python

```python
from wox_plugin import Plugin, Query, QueryResponse, Result, Context, PluginInitParams
from wox_plugin.models.image import WoxImage

class MyPlugin(Plugin):
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api
        self.plugin_dir = params.plugin_directory

    async def query(self, ctx: Context, query: Query) -> QueryResponse:
        return QueryResponse(results=[
            Result(
                title="Hello Wox",
                sub_title="This is a sample result",
                icon=WoxImage.new_emoji("👋"),
                score=100,
            )
        ])

plugin = MyPlugin()
```

### Node.js

```typescript
import { Plugin, Query, QueryResponse, Context, PluginInitParams } from "@wox-launcher/wox-plugin"

class MyPlugin implements Plugin {
  private api!: PluginInitParams["API"]
  private pluginDir = ""

  async init(ctx: Context, params: PluginInitParams): Promise<void> {
    this.api = params.API
    this.pluginDir = params.PluginDirectory
  }

  async query(ctx: Context, query: Query): Promise<QueryResponse> {
    return {
      Results: [
        {
          Title: "Hello Wox",
          SubTitle: "This is a sample result",
          Icon: { ImageType: "emoji", ImageData: "👋" },
          Score: 100,
        },
      ],
    }
  }
}

export const plugin = new MyPlugin()
```

Returning `list[Result]` or `Result[]` directly is deprecated. Python and Node.js hosts still accept the old shape for compatibility with older Wox releases. Use `QueryResponse` only when `plugin.json` declares `MinWoxVersion` >= `2.0.4`.

## `plugin.json` essentials

- Follow the full schema in [Specification](./specification.md)
- Use `Runtime` = `PYTHON` or `NODEJS`
- Point `Entry` at the file Wox should execute
- Add `Features` only for capabilities you actually use

Example:

```json
{
  "Id": "my-awesome-plugin",
  "Name": "My Awesome Plugin",
  "Description": "Do awesome things",
  "Author": "You",
  "Version": "1.0.0",
  "MinWoxVersion": "2.0.4",
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

## Query handling

Wox sends a normalized `Query` object into `query()`:

- `Query.Type` is `input` or `selection`
- `Query.RawQuery` keeps the original input
- `Query.TriggerKeyword`, `Query.Command`, and `Query.Search` give you the parsed parts
- `Query.Id` is the identifier you should keep when doing async follow-up updates
- `Query.Env` carries optional environment context when the `queryEnv` feature is enabled

See [Query Model](./query-model.md) for the exact split and feature-dependent fields.

## Building results

Each `Result` can include:

- `Icon`
- `Preview`
- `Tails`
- `Actions`
- `Group` and `GroupScore`

Useful patterns:

- use `Preview` for markdown, text, image, URL, or file previews
- use `Tails` for badges or small metadata
- use `PreventHideAfterAction` when an action continues to update the same result in place

If you need to update a visible result after an action starts, use:

- `GetUpdatableResult`
- `UpdateResult`

If you need to stream or append additional results for the same active query, use:

- `PushResults`

## Settings

Define your settings UI in `plugin.json` with `SettingDefinitions`.

Common setting types:

- `textbox`
- `checkbox`
- `select`
- `selectAIModel`
- `table`
- `dynamic`
- `head`
- `label`
- `newline`

At runtime:

- read values with `GetSetting`
- persist values with `SaveSetting`
- react to changes with `OnSettingChanged`
- provide runtime-generated settings with `OnGetDynamicSetting`

## Feature flags you will likely use

- `querySelection`: receive text/file selection queries
- `queryEnv`: receive active-window or browser context
- `ai`: use Wox-configured AI APIs
- `deepLink`: register plugin deep links
- `mru`: restore items from Wox MRU storage
- `resultPreviewWidthRatio`: deprecated; use `QueryResponse.Layout.ResultPreviewWidthRatio`
- `gridLayout`: deprecated; use `QueryResponse.Layout.GridLayout`

Only enable features you actually need. They change how Wox routes queries and builds plugin context.

## Screenshot API

Wox now exposes a built-in screenshot workflow to full-featured plugins.

Use it when your plugin needs the user to draw a region and then continue processing the resulting PNG path itself, for example:

- OCR
- image upload
- bug reporting
- visual annotation pipelines outside Wox

### What the API returns

`Screenshot()` returns:

- `Success`: whether the capture completed successfully
- `ScreenshotPath`: exported PNG path when successful
- `ErrMsg`: failure reason, or a warning message when the capture completed with caveats

### Options

`ScreenshotOption` supports:

- `HideAnnotationToolbar`: keep the flow focused on raw area selection
- `AutoConfirm`: finish immediately after the user completes a valid selection

### Node.js example

```typescript
const capture = await this.api.Screenshot(ctx, {
  HideAnnotationToolbar: true,
  AutoConfirm: true,
})

if (!capture.Success) {
  await this.api.Notify(ctx, `Screenshot failed: ${capture.ErrMsg}`)
  return
}

await this.api.Notify(ctx, `Saved to ${capture.ScreenshotPath}`)
```

Behavior notes:

- the exported file path is returned to the plugin; clipboard handling is left to the plugin
- third-party plugins automatically show their own plugin icon in the floating screenshot toolbox
- if you need Wox's built-in annotation UI, leave `HideAnnotationToolbar` unset

## AI, deep links, and MRU

- AI APIs require the `ai` feature
- deep-link callbacks require the `deepLink` feature and `OnDeepLink`
- MRU restore requires the `mru` feature and `OnMRURestore`

These are optional capabilities. Keep the initial version of your plugin smaller if you do not need them yet.

## Local development loop

- keep your plugin directory under `~/.wox/plugins/`, or symlink your working directory there
- after changing `plugin.json`, reload the plugin from Wox settings or restart Wox
- after changing built TypeScript output, rebuild your plugin and reload it

If your plugin touches core/host contracts, rebuild Wox itself instead of assuming the host will pick up type changes automatically.

## Recommended debugging approach

When something fails:

1. verify `plugin.json` first
2. confirm the right runtime host is being used
3. add plugin-side logging through the SDK API
4. inspect the core log at `~/.wox/log/wox.log`, then check UI or host logs in the same log directory if needed
5. if the problem crosses layers, rebuild from the repository root with `make build`
