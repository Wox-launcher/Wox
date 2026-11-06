# Wox Plugin SDK for TypeScript/JavaScript

TypeScript type definitions and SDK for developing Wox plugins in TypeScript/JavaScript.

Plugin-owned windows are available through `API.CreateWindow(ctx, option)`.
The returned instance has `Show`, `Hide`, `Close`, `SetOptions`, `Content.Set`,
`Content.Update` and `Content.PostMessage`, all with named Option/Result contracts.
See the [window API guide and HTML examples](../wox.core/ui/pluginwindow/README.md).

## WoxView protocol types

The SDK exports `WoxViewTree`, `WoxViewNode`, `WoxViewHandler`, component props,
style types and `WoxViewEventData`. These describe the language-independent JSON
wire format. Event props contain `{ $handler: "id" }`; callbacks stay in plugin
code. See the [shared SDK guide](../wox.core/ui/woxview/README.md#sdk-wire-types)
for a complete example, event dispatch, single-file plugins and type generation.

## Quick Start

Query debounce can be declared in `plugin.json` at plugin, trigger and command
levels. Command `DebounceMs` overrides `TriggerDebounceMs[keyword]`, which overrides
the `debounce` feature's `IntervalMs`. Omit a value to inherit, or use `0` to run
immediately. Runtime `RegisterTriggerKeyword({ Keyword, DebounceMs })` overrides
the static trigger setting, and `RegisterQueryCommands` accepts `DebounceMs` on
each `MetadataCommand`. Trigger/command overrides apply to input queries only.
See the [metadata example and precedence rules](../wox.core/plugin/README.md#query-scheduling).

Set `Preview.DefaultHidden: true` on a result to keep its preview collapsed initially.
Users can open it with the row icon or toggle it with Ctrl/Cmd+P. The manual choice
applies to all results in the current query, including live updates. A new query restores
each result's default; before manual input, each result uses its own default.
Omitting the field preserves automatic previews. Global search does not expose plugin previews, including manually opened previews.

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
- **Settings**: `GetSetting()`, `SetSetting()`, `OnSettingChanged()`; legacy `SaveSetting()` remains available but is deprecated
- **Logging**: `log()`
- **i18n**: `getTranslation()`
- **Results**: `getUpdatableResult()`, `updateResult()`, `RemoveResult()`
- **AI**: `llmStream()`
- **MRU**: `onMruRestore()`
- **Callbacks**: `onUnload()`, `onDeepLink()`
- **Commands**: `registerQueryCommands()`
- **Plugin Tools**: `RegisterPluginTool()`, `UnregisterPluginTool()`, `ListPluginTools()`, `InvokePluginTool()` (Wox >= 2.4.5)
- **Clipboard**: `copy()`
- **Cache**: `GetCacheFolder()`
- **Theme**: `GetThemeColors()` (Wox >= 2.4.5)

## Plugin Tools

Plugin Tools are schema-checked operations that plugins register at runtime. They require Wox >= 2.4.5.

```typescript
await this.api.RegisterPluginTool(ctx, {
  Tool: {
    Name: "echo_text",
    Description: "Echo the supplied text",
    InputSchema: {
      type: "object",
      properties: { text: { type: "string" } },
      required: ["text"]
    },
    OutputSchema: {
      type: "object",
      properties: { text: { type: "string" } },
      required: ["text"]
    },
    Annotations: { ReadOnly: true, Destructive: false, Idempotent: true, RequiresUI: false }
  },
  Handler: async (_ctx, option) => ({ Output: { text: option.Arguments.text } })
})

const created = await this.api.InvokePluginTool(ctx, {
  PluginId: notesPluginId,
  Name: "create_note",
  Arguments: { title: "Roadmap", text: "Ship it" }
})
if (created.Error) {
  await this.api.Notify(ctx, created.Error.Message)
  return
}
const opened = await this.api.InvokePluginTool(ctx, {
  PluginId: notesPluginId,
  Name: "open_note",
  Arguments: { noteId: created.Output?.noteId }
})
```

## Actions

For a transient notification with one optional action, use `ShowToastMsg` on Wox 2.5.0 or newer:

```typescript
await api.ShowToastMsg(ctx, {
  Message: "The file is ready",
  Title: "Transfer complete",
  Action: { Name: "Open", ContextData: { path }, Action: async (ctx, action) => openFile(action.ContextData.path) }
})
```

The action runs at most once. `Notify` remains supported for existing plugins and keeps its original routing.

Actions are operations users can perform on results:

Only the default Enter action appears in the toolbar automatically. Set
`ShowInToolbar: true` on an execute or form action to also show it in the toolbar,
with or without a `Hotkey`. The default is `false`. All actions remain available
in the Action Panel and their keyboard shortcuts continue to work.

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

`SetSetting()` requires Wox >= 2.4.0 and accepts a single `SetSettingOption`. Set
`IsLocal: true` when a value must stay on the current device and remain outside
Cloud Sync. Use legacy `SaveSetting()` only when the same plugin build must run
on Wox releases before 2.4.0.

```typescript
const settings: PluginSettingDefinitionItem[] = [
  {
    Type: "password",
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
  },
  {
    Type: "table",
    Value: {
      Key: "sites",
      Title: "Sites",
      DefaultValue: "[]",
      Groups: [{ Key: "advanced", Title: "Advanced", CollapsedByDefault: true }],
      Columns: [
        { Key: "name", Label: "Name", Type: "text" },
        { Key: "injectCss", Label: "Inject CSS", Type: "text", HideInTable: true, Group: "advanced" }
      ]
    } as PluginSettingValueTable,
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
const model: AI.AIModel = { Provider: "deepseek", Name: "deepseek-chat" }
const conversations: AI.Conversation[] = [
  { Role: "system", Text: "You are a helpful assistant.", Timestamp: Date.now() },
  { Role: "user", Text: "Hello!", Timestamp: Date.now() }
]

await api.AIChatStream(ctx, model, conversations, {}, (data: AI.ChatStreamData) => {
  if (data.Status === "streaming") {
    console.log("Chunk:", data.Data)
  } else if (data.Status === "finished") {
    console.log("Complete:", data.Data)
  }
})
```

The plugin must declare the `ai` feature. `model` is required; a `selectAIModel` setting stores the same JSON. Pass `{}` when no thinking mode or loop policy is needed.

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

## Static HTML preview

Use `webview` for inline HTML, including CSS. No HTTP server or temporary HTML file is needed; `html` is a payload field, not a preview type.

```typescript
import type { WoxPreview, WoxPreviewWebviewData } from "@wox-launcher/wox-plugin"

const preview: WoxPreview = {
  PreviewType: "webview",
  PreviewData: JSON.stringify({
    html: '<!doctype html><html><body><h1 style="color:teal">Hello Wox</h1></body></html>'
  } satisfies WoxPreviewWebviewData)
}
// Assign preview to Result.Preview.
```

Set either `html` or `url`. Optional JSON fields are `injectCss`, `userAgent`, `cacheDisabled`, and `cacheKey` (defaults to the URL or HTML). Inline HTML has no plugin-relative base URL: embed CSS/images or use absolute resource URLs. This is browser content, not sanitized Markdown; escape untrusted text before interpolating it into HTML.

## Structured queries

`QueryHint` adds inline argument slots and atomic blocks to `input` queries.
Declare a suffix template in `MetadataCommand.QueryHint` (static `Commands`
or `RegisterQueryCommands`), read `query.QueryHint.Elements` by element ID,
and pass a complete instance to `ChangeQuery`. Existing text queries stay valid.
This is a development-build capability; verify release and SDK support before
setting a distributable plugin's minimum versions.

Arguments can offer optional, ordered `Suggestions` while still accepting free text:

```typescript
const hint: QueryHint = {
  Elements: [{ Id: "filter", Kind: "argument", Suggestions: ["created", "assigned", "search"] }],
}
// Use as the QueryHint of the "issues" command, or of a trigger registration.
```

The empty slot shows `created / assigned / search`. Typing `cr` shows `eated`
and a Tab mark. Tab appends that suffix without executing, adding a space, or
moving to another argument; another Tab uses ordinary argument navigation.
Suggestions are literal input values, not translated labels. Matching ignores case,
uses declaration order, and stops on an exact match. Unmatched input remains valid.

With `TriggerKeywords: ["gh"]` and `Commands: [{ Command: "issues", Description: "Issues" },
{ Command: "prs", Description: "Pull requests" }]`, typing `gh ` automatically
shows `issues / prs` unless the trigger has an explicit Query Hint. Runtime
commands participate too. Global `*` triggers do not aggregate suggestions.
The empty preview shows at most three complete candidates and an ellipsis for
the remainder, shrinking to fit the available width. Automatic command previews
prefer shorter names; matching still uses the full list in declaration order.
Tab completion of an automatic command appends a space and immediately activates
that command's own Query Hint, if declared. Ordinary argument suggestions do not
append a space. Existing registrations and `ChangeQuery` carry `Suggestions` without
another API; omitted suggestions preserve ordinary placeholders.

To disable automatic command hints, add `{ "Name": "disableAutoCommandHint" }`
to the plugin metadata's `Features` array. Explicit Query Hints remain available.

### Result icon container

Set `IconShowContainer: true` on a result to show its icon inside a rounded background
and border that adapt to the theme and selection state. The default is `false`;
the image keeps its aspect ratio.

### Updating an inline HTML preview without navigation

`UpdateResult` also accepts a one-shot WebView instruction:
`{"updateMode":"script","script":"window.updateProgress({count: 1})"}`.
The method signature and preview type are unchanged. Do not combine this instruction
with `html` or `url`, or return it from `query()`. It can target only the current
result's plugin-owned inline HTML document; URL previews and noncurrent results
return `false`. Scope the call with the query context that produced the result.

`true` means accepted, not successfully executed. Wox queues accepted instructions
in order until that document is ready. Switching results, replacing the document,
or destroying the preview discards its pending instructions. JavaScript exceptions
are logged without replacing the page. Wox does not return script values or await
Promises. Instructions are neither cached nor replayed by redraws; other fields in
the same update, such as Actions, retain their normal behavior.

Return complete HTML from each query, using the latest task state. Define an updater
in that HTML and change existing DOM nodes inside it so scrolling, selection, and
animation survive. Serialize state as JSON rather than concatenating raw output into
JavaScript. When embedding initial JSON in a `<script>` element, also escape `<`
as `\u003c` to prevent a literal `</script>` from ending the element.

This capability requires a Wox host containing script-mode preview updates; older
hosts do not support it. No SDK publication is required to transport these JSON keys.

```typescript
// query() returns HTML defining window.updateProgress, initialized with current state.
// Later, update only the current document. Keep ctx from that query.
const state = { count: 1, message: "Scanning…" }
const accepted = await api.UpdateResult(ctx, {
  Id: resultId,
  Preview: {
    PreviewType: "webview",
    PreviewData: JSON.stringify({
      updateMode: "script",
      script: `window.updateProgress(${JSON.stringify(state)});`
    } satisfies WoxPreviewWebviewData)
  }
})
```

## Declarative global hotkeys (development)

Declare `Type: "hotkey"`, `Scope: "global"` in static plugin settings, then subscribe
with `OnHotkey` (`on_hotkey` in Python) during init. Wox manages registration, conflicts
and cleanup. See the [contract and examples](../wox.core/plugin/hotkey.md) for gesture
kinds, paired hold events, stable table IDs and asynchronous callback handling.

### Removing a result

```typescript
const removed = await api.RemoveResult(ctx, { ResultId: actionContext.ResultId, SelectedResultId: nextResultId })
```

Use the original query context and a result owned by your plugin. `SelectedResultId` is optional; if it is absent or unavailable, Wox preserves the current selection or selects the nearest remaining row. `removed.Success` indicates whether the result was visible and removed.

## Interactive WoxView windows

Create a view once and mount it through the existing window API. Each `asWindow()` call creates an independent session; keep business state in the plugin and call `invalidate()` when it changes. Wox retains native interaction state such as focus and text editing. Supply stable `key` values for nodes that can move in a list.

```typescript
import { Button, Column, Text, createWoxView } from "@wox-launcher/wox-plugin"

let count = 0
const view = createWoxView({
  onError: error => console.error(error),
  render: session => Column({ children: [
    Text({ text: `Count: ${count}` }),
    Button({ key: "add", label: "Add", onPress: () => { count++; session.invalidate() } })
  ] })
})
const result = await api.CreateWindow(ctx, {
  Window: { Title: "Counter", Presentation: "window", Width: 360, Height: 240 },
  ...view.asWindow()
})
```

Single-file Node.js plugins use `const { createWoxView, Column, Text, Button } = require("@wox-launcher/wox-plugin")`; Wox supplies the SDK, so no package installation is required. Packaged plugins import it as above. `woxViewNode(type, props, children, key)` supports all schema-defined components, while `Column`, `Text`, and `Button` are convenience builders. Callback arguments follow the WoxView event contract.

The adapter owns `Content` and `OnEvent`; do not override them after spreading the returned options. It serializes updates, coalesces invalidations while an update is pending, and releases handlers when the window closes or the plugin unloads. Errors go to `onError`. This requires matching SDK and host versions. The session is surface-independent, but WoxView previews and `asPreview()` are not yet implemented.
