# {{.Name}} - Node.js Plugin Scaffold

## plugin.json

```json
{
  "Id": "{{.PluginID}}",
  "Name": "{{.Name}}",
  "Description": "{{.Description}}",
  "Version": "1.0.0",
  "MinWoxVersion": "2.0.0",
  "Runtime": "NODEJS",
  "Entry": "dist/index.js",
  "Icon": "emoji:🚀",
  "TriggerKeywords": {{.TriggerKeywordsJSON}},
  "SupportedOS": ["Windows", "Linux", "Macos"]
}
```

## src/index.ts

```typescript
import { Context, Plugin, PluginInitParams, Query, Result, WoxImage, PublicAPI } from "@wox-launcher/wox-plugin"

class {{.PascalName}}Plugin implements Plugin {
  private api!: PublicAPI

  async init(ctx: Context, params: PluginInitParams): Promise<void> {
    this.api = params.API
    // Load initial settings or setup resources here
  }

  async query(ctx: Context, query: Query): Promise<Result[]> {
    if (!query.Search) {
       return [{
         Title: "{{.Name}} Ready",
         SubTitle: "Type specific keywords to search...",
         Icon: { ImageType: "emoji", ImageData: "🚀" } as WoxImage,
         Actions: []
       }]
    }

    return [{
      Title: `Echo: ${query.Search}`,
      SubTitle: "Select to show notification",
      Icon: { ImageType: "emoji", ImageData: "✨" } as WoxImage,
      Actions: [{
        Id: "action_notify",
        Name: "Show Notification",
        IsDefault: true,
        Action: async (ctx, actionContext) => {
          await this.api.Notify(ctx, `You selected: ${query.Search}`)
        }
      }]
    }]
  }
}

export const plugin = new {{.PascalName}}Plugin()
```

## package.json

```json
{
  "name": "{{.KebabName}}",
  "version": "1.0.0",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "watch": "tsc -w"
  },
  "dependencies": {
    "@wox-launcher/wox-plugin": "latest"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "@types/node": "^20.0.0"
  }
}
```

## tsconfig.json

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "commonjs",
    "outDir": "./dist",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true
  },
  "include": ["src/**/*"]
}
```

## Build Steps

1. `pnpm install`
2. `pnpm build`
