# 全功能插件开发指南

全功能插件在专用宿主进程（Python/Node.js）中常驻运行，通过 WebSocket 与 Go 核心通信。它们可以保留状态、使用完整 API（AI、预览、MRU、设置 UI、深度链接等），适合复杂需求。

## 快速开始

- 在 `~/.wox/plugins/<你的插件 id>/` 下创建插件目录。
- 添加 `plugin.json`（见[规范](./specification.md)）和入口文件（如 `main.py`、`index.js`）。
- 安装 SDK：Python ≥ 3.8 用 `uv add wox-plugin`，Node.js ≥ 16 用 `pnpm add @wox-launcher/wox-plugin`。
- 如果你使用 Codex，可查看 [用于插件开发的 AI Skills](./ai-skills.md)。
- 重启 Wox 或在设置里禁用/启用插件以重新加载。

## 最小示例

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
                sub_title="示例结果",
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
        SubTitle: "示例结果",
        Icon: { ImageType: "emoji", ImageData: "👋" },
        Score: 100,
      },
    ]
  }
}

export const plugin = new MyPlugin()
```

## plugin.json 关键点

- 按 [规范](./specification.md) 填写字段、能力开关和设置。
- `Runtime` 取 `PYTHON` 或 `NODEJS`，`Entry` 指向构建后的文件（TypeScript 请指向编译产物）。
- 需要选择查询、查询环境、AI、MRU、预览宽度控制、深度链接等能力时，在 `Features` 中声明。

示例：

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

## 处理查询

- `Query.Type` 可能是 `input` 或 `selection`，只有声明 `querySelection` 才会收到 selection。
- `Query.Env`（活动窗口标题/进程/图标、浏览器 URL）只有启用 `queryEnv` 才会赋值。
- 查看 [查询模型](./query-model.md) 了解 `TriggerKeyword`、`Command`、`Search` 的拆分。

## 构建结果

- 使用 `Result` 可附加 `Preview`（markdown/text/image/url/file/remote）、`Tails`（文本或图片徽标）、`Group`/`GroupScore`、`Actions`。
- `ResultAction` 支持 `Hotkey`、`IsDefault`、`PreventHideAfterAction`、自定义 `ContextData`。
- 通过 `UpdateResult`（使用 `ActionContext` 提供的 id）可以更新正在展示的结果。
- 如果需要更宽的预览区，可以开启 `resultPreviewWidthRatio` 特性。

## 设置

- 在 `plugin.json` 里用 `SettingDefinitions` 定义 UI（textbox/checkbox/select/selectAIModel/table/dynamic/head/label/newline）。
- 值会在初始化参数中传入，可用 `GetSetting`/`SaveSetting` 读写（支持区分平台）。
- 动态设置可通过 API 运行时替换，用于依赖插件数据的下拉或表格。

## AI、深度链接、MRU

- 使用 AI API 需在 `Features` 中声明 `ai`，请求会经由用户配置的模型/秘钥。
- 需要深度链接时先添加 `deepLink` 特性，再注册回调。
- 希望按最近使用排序时，添加 `mru` 并实现 `OnMRURestore`，从存储的 MRU 数据恢复结果。

## 本地测试技巧

- 插件目录放在 `~/.wox/plugins/`（或在此位置做符号链接）。
- 修改 `plugin.json` 或重新构建后，禁用/启用插件或重启 Wox 以重新加载。
- 使用 SDK 类型做单元测试，`query` 内保持快速，尽量异步并缓存。
