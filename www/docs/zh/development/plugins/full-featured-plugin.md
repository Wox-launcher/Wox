# 全功能插件开发指南

全功能插件会运行在专用宿主进程（Python 或 Node.js）中，通过 WebSocket 与 `wox.core` 通信。它们可以常驻、保留状态，并使用更完整的 Wox API，例如预览、设置 UI、toolbar message、MRU 恢复、截图、AI 流式返回和深度链接。

## 什么时候该选全功能插件

当你的插件需要以下任意一种能力时，优先考虑全功能插件：

- 跨查询保留状态
- 异步或网络请求较重
- 自定义设置界面
- 更丰富的预览和操作
- 由插件发起的截图或剪贴板流程
- AI 或 MRU 集成

如果只是一个单文件的小自动化脚本，先看 [脚本插件](./script-plugin.md)。

## 快速开始

1. 在 `~/.wox/plugins/<你的插件 id>/` 下创建目录
2. 添加 `plugin.json` 和入口文件（`main.py`、`index.js`，或者构建产物如 `dist/index.js`）
3. 安装 SDK
4. 在 Wox 设置里重载插件，或直接重启 Wox

SDK 安装命令：

- Python：`uv add wox-plugin`
- Node.js：`pnpm add @wox-launcher/wox-plugin`

如果你使用 Codex，可查看 [用于插件开发的 AI Skills](./ai-skills.md)。

## 最小示例

这些示例返回 `QueryResponse`，因此插件的 `plugin.json` 必须将
`MinWoxVersion` 设置为 `2.0.4` 或更高版本。如果同一个插件构建还要运行在旧版 Wox 上，请直接返回 `list[Result]` 或 `Result[]`。

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
                sub_title="示例结果",
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
          SubTitle: "示例结果",
          Icon: { ImageType: "emoji", ImageData: "👋" },
          Score: 100,
        },
      ],
    }
  }
}

export const plugin = new MyPlugin()
```

直接返回 `list[Result]` 或 `Result[]` 已 deprecated。Python 和 Node.js host 仍会为了兼容旧版 Wox 继续接受旧写法。只有当 `plugin.json` 声明 `MinWoxVersion` >= `2.0.4` 时，才应返回 `QueryResponse`。

## `plugin.json` 关键点

- 完整字段定义见 [规范](./specification.md)
- `Runtime` 取 `PYTHON` 或 `NODEJS`
- `Entry` 指向 Wox 实际执行的文件
- `Features` 只声明你真正需要的能力

示例：

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

## 查询处理

Wox 会把规范化后的 `Query` 传给 `query()`：

- `Query.Type` 可能是 `input` 或 `selection`
- `Query.RawQuery` 保留用户原始输入
- `Query.TriggerKeyword`、`Query.Command`、`Query.Search` 是拆好的查询段
- `Query.Id` 适合在异步后续更新里保留使用
- `Query.Env` 在启用 `queryEnv` 时提供环境信息

具体字段拆分和条件字段可参考 [查询模型](./query-model.md)。

## 构建结果

每个 `Result` 可以带上：

- `Icon`
- `Preview`
- `Tails`
- `Actions`
- `Group` 和 `GroupScore`

常见用法：

- 用 `Preview` 展示 markdown、文本、图片、URL 或文件预览
- 用 `Tails` 展示徽标或补充元数据
- 当一个 action 执行后还要继续原地更新结果时，给它加上 `PreventHideAfterAction`

如果需要在 action 开始后继续修改当前可见结果，可使用：

- `GetUpdatableResult`
- `UpdateResult`

如果需要针对当前查询继续追加或流式推送结果，可使用：

- `PushResults`

## 设置

在 `plugin.json` 里通过 `SettingDefinitions` 定义设置界面。

常用类型：

- `textbox`
- `checkbox`
- `select`
- `selectAIModel`
- `table`
- `dynamic`
- `head`
- `label`
- `newline`

运行时常用 API：

- `GetSetting`
- `SaveSetting`
- `OnSettingChanged`
- `OnGetDynamicSetting`

## 常见能力开关

- `querySelection`：接收文本/文件选择查询
- `queryEnv`：接收活动窗口或浏览器上下文
- `ai`：使用 Wox 配置好的 AI 能力
- `deepLink`：注册插件深度链接
- `mru`：从 Wox 的最近使用记录恢复结果
- `resultPreviewWidthRatio`：已 deprecated，改用 `QueryResponse.Layout.ResultPreviewWidthRatio`
- `gridLayout`：已 deprecated，改用 `QueryResponse.Layout.GridLayout`

只打开真正需要的能力，这些能力会直接影响 Wox 如何路由查询和构建插件上下文。

## 截图 API

现在全功能插件可以直接调用 Wox 内置截图流程。

适合这些场景：

- OCR
- 图片上传
- 缺陷反馈
- 插件自己后处理 PNG 的视觉流程

### 返回结果

`Screenshot()` 会返回：

- `Success`：截图是否成功完成
- `ScreenshotPath`：成功时导出的 PNG 路径
- `ErrMsg`：失败原因；如果成功但存在提示信息，也会放在这里

### 可选参数

`ScreenshotOption` 目前支持：

- `HideAnnotationToolbar`：只保留更纯粹的选区流程
- `AutoConfirm`：用户完成有效选区后立即结束

### Node.js 示例

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

行为说明：

- API 返回的是导出的文件路径，是否复制到剪贴板由插件自己决定
- 第三方插件触发截图时，悬浮工具栏会自动显示插件自己的图标
- 如果你想保留 Wox 内置标注 UI，就不要设置 `HideAnnotationToolbar`

## AI、深度链接与 MRU

- AI 能力需要声明 `ai`
- 深度链接需要声明 `deepLink` 并注册 `OnDeepLink`
- MRU 恢复需要声明 `mru` 并实现 `OnMRURestore`

这些都是可选能力。不需要时不要先加，先把插件的核心路径做小做稳。

## 本地开发循环

- 插件目录放在 `~/.wox/plugins/` 下，或者把工作目录软链接到这里
- 修改 `plugin.json` 后，需要在 Wox 设置里重载插件，或直接重启 Wox
- 修改 TypeScript 构建产物后，先重新构建插件，再重载

如果你的改动涉及 core、宿主和 SDK 的共享契约，不要只假设热更新能覆盖，应该把 Wox 本体重新构建一遍。

## 推荐排错方式

出问题时，建议按这个顺序查：

1. 先检查 `plugin.json`
2. 确认实际走的是哪一个运行时宿主
3. 在插件里通过 SDK API 打日志
4. 查看 `~/.wox/log/wox.log` core 日志，需要时再看同一日志目录里的 UI 或宿主日志
5. 如果问题跨层，回到仓库根目录执行 `make build`
