# {{.Name}} - Node.js Plugin Scaffold

## plugin.json
{"Id":"{{.PluginID}}","Name":"{{.Name}}","Description":"{{.Description}}","Version":"1.0.0","MinWoxVersion":"2.0.0","Runtime":"NODEJS","Entry":"dist/index.js","Icon":"emoji:🚀","TriggerKeywords":{{.TriggerKeywordsJSON}},"SupportedOS":["Windows","Linux","Macos"]}

## src/index.ts
import { Context, Plugin, PluginInitParams, Query, Result, WoxImage, PublicAPI } from "@wox-launcher/wox-plugin"

class {{.PascalName}}Plugin implements Plugin {
  private api!: PublicAPI

  async init(ctx: Context, params: PluginInitParams): Promise<void> {
    this.api = params.API
  }

  async query(ctx: Context, query: Query): Promise<Result[]> {
    return [{
      Title: "Hello from {{.Name}}",
      SubTitle: query.Search || "Type something...",
      Icon: { ImageType: "emoji", ImageData: "🚀" } as WoxImage,
      Actions: [{
        Id: "action",
        Name: "Execute",
        IsDefault: true,
        Action: async (ctx, actionContext) => {
          await this.api.Notify(ctx, "Action executed!")
        }
      }]
    }]
  }
}

export const plugin = new {{.PascalName}}Plugin()

## package.json
{"name":"{{.KebabName}}","version":"1.0.0","main":"dist/index.js","scripts":{"build":"tsc"},"dependencies":{"@wox-launcher/wox-plugin":"latest"},"devDependencies":{"typescript":"^5.0.0"}}

## tsconfig.json
{"compilerOptions":{"target":"ES2020","module":"commonjs","outDir":"./dist","strict":true,"esModuleInterop":true},"include":["src/**/*"]}

## Build Steps
1. pnpm install
2. pnpm build
