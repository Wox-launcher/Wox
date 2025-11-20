# 插件规范

`plugin.json` 位于插件根目录（脚本插件的同样结构写在注释 JSON 里）。Wox 读取它来决定插件能否在当前平台加载、运行哪个入口文件、注册哪些触发关键字/命令。

## plugin.json 字段

| 字段                 | 必填 | 描述                                                     | 示例                                                      |
| -------------------- | ---- | -------------------------------------------------------- | --------------------------------------------------------- |
| `Id`                 | ✅   | 唯一标识（建议 UUID）                                    | `"cea0f...28855"`                                         |
| `Name`               | ✅   | 展示名称                                                 | `"Calculator"`                                            |
| `Description`        | ✅   | 商店/设置里展示的简介                                    | `"Quick math in the launcher"`                            |
| `Author`             | ✅   | 作者                                                     | `"Wox Team"`                                              |
| `Version`            | ✅   | 插件语义化版本                                           | `"1.0.0"`                                                 |
| `MinWoxVersion`      | ✅   | 需要的最低 Wox 版本                                      | `"2.0.0"`                                                 |
| `Website`            | ⭕   | 首页/仓库链接                                            | `"https://github.com/Wox-launcher/Wox"`                   |
| `Runtime`            | ✅   | `PYTHON`、`NODEJS`、`SCRIPT`（Go 保留作系统插件）        | `"PYTHON"`                                                |
| `Entry`              | ✅   | 入口文件，相对插件根目录。脚本插件由 Wox 自动填写。      | `"main.py"`                                               |
| `Icon`               | ✅   | [WoxImage](#icon-格式) 字符串（emoji/base64/相对路径等） | `"emoji:🧮"`                                              |
| `TriggerKeywords`    | ✅   | 一个或多个触发关键字。`"*"` 表示全局触发。               | `["calc"]`                                                |
| `Commands`           | ⭕   | 可选命令（见 [查询模型](./query-model.md)）              | `[{"Command":"install","Description":"Install plugins"}]` |
| `SupportedOS`        | ✅   | `Windows`/`Linux`/`Macos`，脚本插件留空时默认全部        | `["Windows","Macos"]`                                     |
| `Features`           | ⭕   | 可选能力开关（见下方）                                   | `[{"Name":"debounce","Params":{"IntervalMs":"200"}}]`     |
| `SettingDefinitions` | ⭕   | 设置表单定义                                             | `[...]`                                                   |

### Icon 格式

`Icon` 使用 WoxImage 字符串格式：

- `emoji:🧮`
- `data:image/png;base64,<...>` 或纯 base64（默认为 png）
- `relative/path/to/icon.png`（相对插件目录）
- 支持绝对路径，但建议避免以保持可移植性。

### 示例 plugin.json

```json
{
  "Id": "cea0fdfc6d3b4085823d60dc76f28855",
  "Name": "Calculator",
  "Description": "Quick math in the launcher",
  "Author": "Wox Team",
  "Version": "1.0.0",
  "MinWoxVersion": "2.0.0",
  "Runtime": "PYTHON",
  "Entry": "main.py",
  "Icon": "emoji:🧮",
  "TriggerKeywords": ["calc"],
  "SupportedOS": ["Windows", "Linux", "Macos"],
  "Features": [{ "Name": "debounce", "Params": { "IntervalMs": "250" } }, { "Name": "ai" }],
  "SettingDefinitions": [
    {
      "Type": "textbox",
      "Value": {
        "Key": "api_key",
        "Label": "API Key",
        "Tooltip": "Get it from your provider",
        "DefaultValue": ""
      }
    }
  ]
}
```

## Feature 能力

在 `Features` 中声明需要的特殊能力：

- `querySelection`：接收 `QueryTypeSelection`（拖拽/选中文本）查询。
- `debounce`：输入时防抖。参数：`IntervalMs`（字符串，毫秒）。
- `ignoreAutoScore`：关闭 Wox 默认的使用频率评分。
- `queryEnv`：请求查询环境。参数：`requireActiveWindowName` / `requireActiveWindowPid` / `requireActiveWindowIcon` / `requireActiveBrowserUrl`（`"true"`/`"false"`）。
- `ai`：允许使用 Wox 的 AI API。
- `deepLink`：插件自定义深度链接。
- `resultPreviewWidthRatio`：控制结果列表与预览区宽度比例，`WidthRatio` 取 0~1。
- `mru`：启用最近使用（MRU），插件需实现 `OnMRURestore`。

## SettingDefinitions

定义在 Wox 设置页展示的表单，并在插件宿主中可读取：

| 类型            | 描述                           | 关键字段                                                                         |
| --------------- | ------------------------------ | -------------------------------------------------------------------------------- |
| `head`          | 分组标题                       | `Content`                                                                        |
| `label`         | 只读文本                       | `Content`、`Tooltip`、可选 `Style`                                               |
| `textbox`       | 单/多行文本                    | `Key`、`Label`、`Suffix`、`DefaultValue`、`Tooltip`、`MaxLines`、`Style`         |
| `checkbox`      | 布尔开关                       | `Key`、`Label`、`DefaultValue`、`Tooltip`、`Style`                               |
| `select`        | 下拉选择                       | `Key`、`Label`、`DefaultValue`、`Options[] { Label, Value }`、`Tooltip`、`Style` |
| `selectAIModel` | AI 模型下拉（由 Wox 动态填充） | `Key`、`Label`、`DefaultValue`、`Tooltip`、`Style`                               |
| `table`         | 可编辑表格                     | `Key`、`Columns`、`DefaultValue`、`Tooltip`、`Style`                             |
| `dynamic`       | 由插件运行时动态替换           | 仅 `Key`                                                                         |
| `newline`       | 视觉分隔                       | 无                                                                               |

`Style` 支持 `PaddingLeft/Top/Right/Bottom`、`Width`、`LabelWidth`。设置值会在初始化参数传入插件，并在脚本插件中以 `WOX_SETTING_<KEY>` 环境变量提供。

### SettingDefinitions 示例

带布局的最小配置与 AI 模型选择：

```json
{
  "SettingDefinitions": [
    { "Type": "head", "Value": "API" },
    {
      "Type": "textbox",
      "Value": {
        "Key": "api_key",
        "Label": "API Key",
        "Tooltip": "从服务商获取",
        "DefaultValue": "",
        "Style": { "Width": 320, "LabelWidth": 90 }
      }
    },
    {
      "Type": "selectAIModel",
      "Value": {
        "Key": "model",
        "Label": "Model",
        "DefaultValue": "",
        "Tooltip": "使用已配置的 AI 提供商"
      }
    },
    { "Type": "newline" }
  ]
}
```

表格 + 动态设置（运行时由插件填充）：

```json
{
  "SettingDefinitions": [
    { "Type": "head", "Value": "规则" },
    {
      "Type": "table",
      "Value": {
        "Key": "rules",
        "Tooltip": "键值规则",
        "Columns": [
          { "Title": "Key", "Width": 150 },
          { "Title": "Value", "Width": 240 }
        ],
        "DefaultValue": [
          ["foo", "bar"],
          ["hello", "world"]
        ]
      }
    },
    {
      "Type": "dynamic",
      "Value": {
        "Key": "runtime_options"
      }
    }
  ]
}
```

设置值如何到达插件：

- 全功能插件：通过宿主 SDK 的 `GetSetting`/`SaveSetting` 读写，`dynamic` 内容通过动态设置回调提供。
- 脚本插件：每个键会导出为 `WOX_SETTING_<UPPER_SNAKE_KEY>` 环境变量。

#### Dynamic 设置回调（后端如何填充）

Python（wox-plugin）：

```python
from wox_plugin import Plugin, Context, PluginInitParams
from wox_plugin.models.setting import PluginSettingDefinitionItem, PluginSettingDefinitionType, PluginSettingValueSelect

class MyPlugin(Plugin):
    async def init(self, ctx: Context, params: PluginInitParams) -> None:
        self.api = params.api

        async def get_dynamic(key: str):
            if key == "runtime_options":
                return PluginSettingDefinitionItem(
                    type=PluginSettingDefinitionType.SELECT,
                    value=PluginSettingValueSelect(
                        key="runtime_options",
                        label="Runtime Options",
                        default_value="a",
                        options=[
                            {"Label": "Option A", "Value": "a"},
                            {"Label": "Option B", "Value": "b"},
                        ],
                    ),
                )
            return None  # 未识别的 key

        await self.api.on_get_dynamic_setting(ctx, get_dynamic)
```

Node.js（SDK）：

```typescript
import { Plugin, Context, PluginInitParams, PluginSettingDefinitionItem } from "@wox-launcher/wox-plugin"

class MyPlugin implements Plugin {
  private api: any

  async init(ctx: Context, params: PluginInitParams): Promise<void> {
    this.api = params.API

    await this.api.OnGetDynamicSetting(ctx, (key: string): PluginSettingDefinitionItem | null => {
      if (key !== "runtime_options") return null
      return {
        Type: "select",
        Value: {
          Key: "runtime_options",
          Label: "Runtime Options",
          DefaultValue: "a",
          Options: [
            { Label: "Option A", Value: "a" },
            { Label: "Option B", Value: "b" }
          ]
        }
      }
    })
  }
}
```

> 提示：动态设置会在打开设置页面时按需获取。请保持回调快速且可预期，如需远程数据请做好缓存，避免拖慢 UI。
