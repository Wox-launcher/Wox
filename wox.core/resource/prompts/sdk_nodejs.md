# Wox Node.js Plugin SDK

## Installation
pnpm add @wox-launcher/wox-plugin

## Key Types

### Plugin Interface
interface Plugin {
  init(ctx: Context, params: PluginInitParams): Promise<void>
  query(ctx: Context, query: Query): Promise<Result[]>
}

### Query
interface Query {
  Type: "input" | "selection"
  RawQuery: string
  TriggerKeyword: string
  Command: string
  Search: string
}

### Result
interface Result {
  Title: string
  SubTitle?: string
  Icon: WoxImage
  Actions: ResultAction[]
  Score?: number
}

### ResultAction
interface ResultAction {
  Id: string
  Name: string
  IsDefault?: boolean
  Action: (ctx: Context, actionContext: ActionContext) => Promise<void>
}

### WoxImage
type WoxImageType = "absolute" | "relative" | "base64" | "svg" | "url" | "emoji" | "lottie"
interface WoxImage { ImageType: WoxImageType; ImageData: string }

### API Methods
- ChangeQuery(ctx, query): Change the query
- HideApp(ctx): Hide Wox
- ShowApp(ctx): Show Wox
- Notify(ctx, message): Show notification
- Log(ctx, level, msg): Write log
- GetSetting(ctx, key): Get plugin setting
- SaveSetting(ctx, key, value): Save plugin setting
- LLMStream(ctx, conversations, callback): Chat with LLM

## Example
import { Plugin, Query, Result, WoxImage } from "@wox-launcher/wox-plugin"

class MyPlugin implements Plugin {
  async init(ctx, params) { this.api = params.API }
  async query(ctx, query) {
    return [{
      Title: "Hello " + query.Search,
      Icon: { ImageType: "emoji", ImageData: "👋" },
      Actions: [{ Id: "copy", Name: "Copy", Action: async () => {} }]
    }]
  }
}
export const plugin = new MyPlugin()
