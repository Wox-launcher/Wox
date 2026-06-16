# Wox Node.js Plugin SDK Reference

## Installation

`pnpm add @wox-launcher/wox-plugin`

## Core Interface

### Plugin

```typescript
interface Plugin {
  init(ctx: Context, params: PluginInitParams): Promise<void>;
  query(ctx: Context, query: Query): Promise<QueryResponse>;
}
```

Return `QueryResponse` when `plugin.json` declares `MinWoxVersion` >= `2.0.4`.
Use `QueryResponse.Layout.ResultPreviewWidthRatio` and
`QueryResponse.Layout.GridLayout` for query-scoped layout. The older
`resultPreviewWidthRatio` and `gridLayout` metadata features are deprecated
because they can only describe static plugin or command defaults.

### PluginInitParams

```typescript
interface PluginInitParams {
  API: PublicAPI;
  PluginDirectory: string;
}
```

## Data Models

### Query

```typescript
interface Query {
  Type: "input" | "selection";
  RawQuery: string;
  TriggerKeyword: string;
  Command: string;
  Search: string;
}
```

### Result

```typescript
interface Result {
  Title: string; // Supports "i18n:key" prefix for auto-translation
  SubTitle?: string; // Supports "i18n:key" prefix
  Icon: WoxImage;
  Actions: ResultAction[];
  Score?: number; // 0-100, optional
  ContextData?: any; // Data passed to actions
}
```

### ResultAction

```typescript
interface ResultAction {
  Id: string;
  Name: string;
  IsDefault?: boolean;
  Action: (ctx: Context, actionContext: ActionContext) => Promise<void>;
}
```

### WoxImage

```typescript
type WoxImageType = "absolute" | "relative" | "base64" | "svg" | "url" | "emoji" | "lottie";
interface WoxImage {
  ImageType: WoxImageType;
  ImageData: string;
}
```

## Public API Methods

The `ctx` object is required for all API calls.

### General

- `ChangeQuery(ctx, query: PlainQuery)`: Update the search bar text.
- `HideApp(ctx)`: Hide the Wox window.
- `ShowApp(ctx)`: Show the Wox window.
- `Notify(ctx, message)`: Display a system notification.
- `Log(ctx, level, msg)`: Write to plugin logs. Levels: `"Info"`, `"Error"`, `"Debug"`, `"Warning"`.
- `Copy(ctx, params: CopyParams)`: Copy text or image to clipboard.
- `IsVisible(ctx)`: Check if Wox window is visible.

### Settings

- `GetSetting(ctx, key)`: Retrieve a stored setting.
- `SaveSetting(ctx, key, value, isPlatformSpecific)`: Save a setting.
- `OnSettingChanged(ctx, callback)`: Subscribe to setting changes.
- `OnGetDynamicSetting(ctx, callback)`: Provide runtime-generated setting definitions for `dynamic` settings.

### UI Updates

- `UpdateResult(ctx, result: UpdatableResult)`: Update a specific result in real-time (e.g., progress bars).
- `PushResults(ctx, query, results)`: Append results to the current list.
- `RefreshQuery(ctx, param)`: Re-run the current query.
- `GetUpdatableResult(ctx, resultId)`: Get current state of a result.

### AI & LLM

- `AIChatStream(ctx, model, conversations, options, callback)`: Stream responses from AI providers.

### Internationalization (i18n)

- `GetTranslation(ctx, key)`: Get a raw translated string (without formatting).
  > **Note**: You must handle string formatting (e.g., `sprintf` or template literals) in your code. This method only returns the raw string from the lang file.

## Settings Authoring Notes

- Read `references/plugin_json_schema.md` before writing `plugin.json` settings.
- For ready-to-copy settings examples and advanced patterns, read `references/settings_patterns.md`.
- `OnGetDynamicSetting` is used together with a `dynamic` entry in `SettingDefinitions`.
- Use static `QueryRequirements` in `plugin.json` when a query requires settings such as API keys. Wox blocks the query before calling `query()` and shows the built-in `query_requirement_settings` setup preview.
- There is no runtime `register_query_requirements` API. Declare query requirements in metadata.

### QueryRequirements Types

```typescript
export interface PluginQueryRequirement {
  SettingKey: string;
  Validators?: PluginSettingValidator[];
  Message?: string;
}

export interface PluginQueryRequirements {
  AnyQuery?: PluginQueryRequirement[];
  QueryWithoutCommand?: PluginQueryRequirement[];
  QueryWithCommand?: Record<string, PluginQueryRequirement[]>;
}
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

## Usage Example

```typescript
import { Plugin, Query, Result, WoxImage } from "@wox-launcher/wox-plugin";

class MyPlugin implements Plugin {
  private api: any;

  async init(ctx, params) {
    this.api = params.API;
  }

  async query(ctx, query) {
    // Example: Getting a translation and formatting it
    const rawTemplate = await this.api.GetTranslation(ctx, "hello_template"); // "Hello, %s!"
    const greeting = rawTemplate.replace("%s", query.Search);

    return [
      {
        Title: greeting,
        Icon: { ImageType: "emoji", ImageData: "👋" },
        Actions: [{ Id: "copy", Name: "Copy", Action: async () => {} }],
      },
    ];
  }
}
export const plugin = new MyPlugin();
```
