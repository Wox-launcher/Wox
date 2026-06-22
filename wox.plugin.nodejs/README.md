# Wox Plugin SDK for TypeScript/JavaScript

TypeScript type definitions and SDK for developing Wox plugins in TypeScript/JavaScript.

## Quick Start

### Installation

```bash
npm install wox-plugin
```

### Basic Plugin

This example returns `QueryResponse`, so the plugin's `plugin.json` should set
`MinWoxVersion` to `2.0.4` or newer. Return `Result[]` directly if you need the
same plugin build to run on older Wox releases.

```typescript
import { Plugin, Context, Query, QueryResponse, Result, NewContext, WoxImage } from "wox-plugin"

class MyPlugin implements Plugin {
  private api: PublicAPI

  async init(ctx: Context, initParams: PluginInitParams): Promise<void> {
    this.api = initParams.API
    await this.api.Log(ctx, "Info", "MyPlugin initialized")
  }

  async query(ctx: Context, query: Query): Promise<QueryResponse> {
    const results: Result[] = []

    for (const item of this.getItems(query.Search)) {
      results.push({
        Title: item.name,
        SubTitle: item.description,
        Icon: { ImageType: "emoji", ImageData: "🔍" },
        Score: 100,
        Actions: [
          {
            Name: "Open",
            Icon: { ImageType: "emoji", ImageData: "🔗" },
            IsDefault: true,
            Action: async (ctx, actionCtx) => {
              await this.openItem(item)
            }
          }
        ]
      })
    }

    return { Results: results }
  }
}
```

## Key Components

### Plugin Interface

Every plugin must implement the `Plugin` interface:

```typescript
interface Plugin {
  init: (ctx: Context, initParams: PluginInitParams) => Promise<void>
  query: (ctx: Context, query: Query) => Promise<QueryReturn>
}
```

Returning `Result[]` directly is deprecated. The Node.js host still accepts it
for compatibility with older Wox releases. Use `QueryResponse` only when
`plugin.json` declares `MinWoxVersion` >= `2.0.4` so results, refinements, and
layout hints are carried together.

When a plugin needs to control the preview width or grid layout, set
`QueryResponse.Layout.ResultPreviewWidthRatio` or `QueryResponse.Layout.GridLayout`.
The older `resultPreviewWidthRatio` and `gridLayout` metadata features are
deprecated because they can only describe static plugin or command defaults.

### Query Models

- **Query**: User query with search text, type, selection, environment
- **QueryType**: `INPUT` (typing) or `SELECTION` (selected content)
- **Selection**: Text or file paths selected by user
- **QueryEnv**: Environment context (active window, browser URL)

### Result Models

- **Result**: Search result with title, icon, preview, actions
- **ResultAction**: User action on a result
- **ResultActionType**: `EXECUTE` (immediate) or `FORM` (show form)
- **ResultTail**: Additional visual elements (text or image)
- **UpdatableResult**: Result that can be updated in UI

### Image Types

Supported image types:

- `absolute`: Absolute file path
- `relative`: Path relative to plugin directory
- `base64`: Base64 encoded image with data URI prefix (`data:image/png;base64,...`)
- `svg`: SVG string content
- `url`: HTTP/HTTPS URL
- `emoji`: Emoji character
- `lottie`: Lottie animation JSON

```typescript
// Emoji icon
{ ImageType: "emoji", ImageData: "🔍" }

// Base64 image
{ ImageType: "base64", ImageData: "data:image/png;base64,iVBORw0..." }

// Relative path
{ ImageType: "relative", ImageData: "./icons/icon.png" }
```

### Public API

Methods for interacting with Wox:

- **UI Control**: `showApp()`, `hideApp()`, `isVisible()`, `notify()`
- **Toolbar Msg**: `ShowToolbarMsg()`, `ClearToolbarMsg()`, `OnEnterPluginQuery()`, `OnLeavePluginQuery()`
- **Query**: `changeQuery()`, `refreshQuery()`, `pushResults()`
- **Settings**: `getSetting()`, `saveSetting()`, `onSettingChanged()`
- **Logging**: `log()`
- **i18n**: `getTranslation()`
- **Results**: `getUpdatableResult()`, `updateResult()`
- **AI**: `llmStream()`
- **MRU**: `onMruRestore()`
- **Callbacks**: `onUnload()`, `onDeepLink()`
- **Commands**: `registerQueryCommands()`
- **Clipboard**: `copy()`

## Actions

Actions are operations users can perform on results:

```typescript
ResultAction({
  name: "Copy",
  icon: { ImageType: "emoji", ImageData: "📋" },
  isDefault: true,
  hotkey: "Ctrl+C",
  action: async (ctx, actionCtx) => {
    await this.copyToClipboard(actionCtx.contextData)
  }
})
```

## Settings

Define settings for your plugin:

```typescript
const settings: PluginSettingDefinitionItem[] = [
  {
    Type: "textbox",
    Value: {
      Key: "apiKey",
      Label: "API Key",
      Suffix: "",
      DefaultValue: "",
      Tooltip: "Enter your API key",
      MaxLines: 1,
      Validators: []
    } as PluginSettingValueTextBox,
    DisabledInPlatforms: [],
    IsPlatformSpecific: false
  },
  {
    Type: "checkbox",
    Value: {
      Key: "enabled",
      Label: "Enable Feature",
      DefaultValue: "true",
      Tooltip: ""
    } as PluginSettingValueCheckBox,
    DisabledInPlatforms: [],
    IsPlatformSpecific: false
  }
]
```

`Style` is deprecated and should not be used for new settings. Wox owns setting
spacing and control widths so plugin settings remain visually consistent.

## AI/LLM Integration

Stream responses from AI models:

```typescript
const conversations: AI.Conversation[] = [
  { Role: "system", Text: "You are a helpful assistant.", Timestamp: Date.now() },
  { Role: "user", Text: "Hello!", Timestamp: Date.now() }
]

await api.LLMStream(ctx, conversations, (data: AI.ChatStreamData) => {
  if (data.Status === "streaming") {
    console.log("Chunk:", data.Data)
  } else if (data.Status === "finished") {
    console.log("Complete:", data.Data)
  }
})
```

## Plugin Metadata

Plugins must declare metadata in a `plugin.json` file:

```json
{
  "ID": "com.myplugin.example",
  "Name": "My Plugin",
  "Author": "Your Name",
  "Version": "1.0.0",
  "MinWoxVersion": "2.0.4",
  "Runtime": "nodejs",
  "Entry": "main.js",
  "TriggerKeywords": ["my"],
  "Description": "My awesome Wox plugin",
  "Website": "https://github.com/user/myplugin",
  "Icon": "https://example.com/icon.png",
  "QueryRequirements": {
    "AnyQuery": [
      {
        "SettingKey": "apiKey",
        "Validators": [{ "Type": "not_empty" }],
        "Message": "i18n:my_plugin_api_key_required"
      }
    ],
    "QueryWithoutCommand": [],
    "QueryWithCommand": {
      "download": [
        {
          "SettingKey": "downloadPath",
          "Validators": [{ "Type": "not_empty" }]
        }
      ]
    }
  }
}
```

## Query Flow

1. User triggers Wox and types trigger keyword (e.g., "my query")
2. Wox calls `plugin.query()` with:
   - `query.TriggerKeyword = "my"`
   - `query.Command = ""`
   - `query.Search = "query"`
3. Plugin returns `QueryResponse` when `MinWoxVersion` is at least `2.0.4`
4. Wox displays results sorted by score

## For More Information

- Wox Documentation: https://github.com/Wox-launcher/Wox
- Plugin Examples: https://github.com/Wox-launcher/Wox.Plugin.Nodejs
