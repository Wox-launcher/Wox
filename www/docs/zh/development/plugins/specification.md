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
| `SupportedOS`        | ✅   | `Windows`/`Linux`/`Darwin`，脚本插件留空时默认全部       | `["Windows","Darwin"]`                                    |
| `Features`           | ⭕   | 可选能力开关（见下方）                                   | `[{"Name":"debounce","Params":{"IntervalMs":"200"}}]`     |
| `SettingDefinitions` | ⭕   | 设置表单定义                                             | `[...]`                                                   |
| `I18n`               | ⭕   | 内联翻译（见 [国际化](#国际化)）                         | `{"en_US":{"key":"value"}}`                               |

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
  "SupportedOS": ["Windows", "Darwin", "Linux"],
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
- `resultPreviewWidthRatio`：已 deprecated。请改用 `QueryResponse.Layout.ResultPreviewWidthRatio`，以便按每次查询控制预览宽度。
- `mru`：启用最近使用（MRU），插件需实现 `OnMRURestore`。
- `gridLayout`：已 deprecated。请改用 `QueryResponse.Layout.GridLayout`，以便按每次查询控制网格展示。兼容说明见 [网格布局](#网格布局)。

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

`Style` 支持 `PaddingLeft/Top/Right/Bottom`、`Width`。设置值会在初始化参数传入插件，并在脚本插件中以 `WOX_SETTING_<KEY>` 环境变量提供。

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
        "Style": { "Width": 320 }
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
import { Plugin, Context, PluginInitParams, PluginSettingDefinitionItem } from "@wox-launcher/wox-plugin";

class MyPlugin implements Plugin {
  private api: any;

  async init(ctx: Context, params: PluginInitParams): Promise<void> {
    this.api = params.API;

    await this.api.OnGetDynamicSetting(ctx, (key: string): PluginSettingDefinitionItem | null => {
      if (key !== "runtime_options") return null;
      return {
        Type: "select",
        Value: {
          Key: "runtime_options",
          Label: "Runtime Options",
          DefaultValue: "a",
          Options: [
            { Label: "Option A", Value: "a" },
            { Label: "Option B", Value: "b" },
          ],
        },
      };
    });
  }
}
```

> 提示：动态设置会在打开设置页面时按需获取。请保持回调快速且可预期，如需远程数据请做好缓存，避免拖慢 UI。

## 国际化

Wox 支持插件国际化（i18n），让你的插件可以根据用户的语言偏好显示不同的文本。有两种方式提供翻译：

### 方式一：在 plugin.json 中内联配置（推荐脚本插件使用）

直接在 `plugin.json` 中使用 `I18n` 字段定义翻译。这对于没有目录结构的脚本插件特别有用：

```json
{
  "Id": "my-plugin-id",
  "Name": "My Plugin",
  "Description": "i18n:plugin_description",
  "TriggerKeywords": ["mp"],
  "I18n": {
    "en_US": {
      "plugin_description": "A useful plugin",
      "result_title": "Result: {0}",
      "action_copy": "Copy to clipboard"
    },
    "zh_CN": {
      "plugin_description": "一个有用的插件",
      "result_title": "结果: {0}",
      "action_copy": "复制到剪贴板"
    }
  }
}
```

### 方式二：语言文件（推荐全功能插件使用）

在插件根目录创建 `lang/` 目录，存放以语言代码命名的 JSON 文件：

```
my-plugin/
├── plugin.json
├── main.py
└── lang/
    ├── en_US.json
    └── zh_CN.json
```

每个语言文件包含扁平的键值对：

```json
// lang/en_US.json
{
  "plugin_description": "A useful plugin",
  "result_title": "Result: {0}",
  "action_copy": "Copy to clipboard"
}
```

```json
// lang/zh_CN.json
{
  "plugin_description": "一个有用的插件",
  "result_title": "结果: {0}",
  "action_copy": "复制到剪贴板"
}
```

### 使用翻译

要使用翻译，在文本前加上 `i18n:` 前缀，后跟翻译键：

```python
# Python 示例
result = Result(
    title="i18n:result_title",
    sub_title="i18n:action_copy"
)

# 或使用 API 程序化获取翻译文本
translated = await api.get_translation(ctx, "i18n:result_title")
```

```typescript
// Node.js 示例
const result: Result = {
  Title: "i18n:result_title",
  SubTitle: "i18n:action_copy",
};

// 或使用 API
const translated = await api.GetTranslation(ctx, "i18n:result_title");
```

### 翻译优先级

Wox 按以下顺序查找翻译：

1. plugin.json 中的内联 `I18n`（当前语言）
2. `lang/{当前语言}.json` 文件
3. plugin.json 中的内联 `I18n`（en_US 回退）
4. `lang/en_US.json` 文件（回退）
5. 如果都未找到，返回原始键

### 支持的语言

| 代码    | 语言             |
| ------- | ---------------- |
| `en_US` | 英语（美国）     |
| `zh_CN` | 简体中文         |
| `pt_BR` | 葡萄牙语（巴西） |
| `ru_RU` | 俄语             |

> 提示：始终提供 `en_US` 翻译作为回退语言。

## 结果预览宽度比例

> Deprecated：优先使用 `QueryResponse.Layout.ResultPreviewWidthRatio`。该
> metadata 功能仍会兼容旧插件，但只能表达插件级或命令级的静态默认值；
> QueryResponse 可以根据每次查询返回的结果集决定预览宽度。

`resultPreviewWidthRatio` 功能用于控制预览面板可见时，结果列表占启动器宽度的比例。`WidthRatio` 必须在 `0` 到 `1` 之间；例如 `0.3` 表示结果列表占 30%，预览区占 70%。

当只有部分命令需要特殊布局时，可以使用 `Commands`。空 `Commands` 表示所有命令生效，`["preview"]` 表示只对该命令生效，`["!preview"]` 表示除 `preview` 外的所有命令生效。`WidthRatio: 0.0` 会隐藏结果列表，适合类似 Quick Look 的纯预览命令。

```json
{
  "Features": [
    {
      "Name": "resultPreviewWidthRatio",
      "Params": {
        "WidthRatio": 0.0,
        "Commands": ["preview"]
      }
    }
  ]
}
```

## 网格布局

> Deprecated：优先使用 `QueryResponse.Layout.GridLayout`。该 metadata 功能
> 仍会兼容旧插件，但只能表达插件级或命令级的静态默认值；QueryResponse
> 可以根据每次查询返回的结果集决定是否使用网格布局。

`gridLayout` 功能可将结果以网格形式展示，替代默认的垂直列表。适用于展示表情符号、图标、颜色或图片缩略图等视觉元素的插件。

### 兼容配置

现有插件仍可在 `plugin.json` 中添加 `gridLayout` 功能：

```json
{
  "Features": [
    {
      "Name": "gridLayout",
      "Params": {
        "Columns": "8",
        "ShowTitle": "false",
        "ItemPadding": "12",
        "ItemMargin": "6"
      }
    }
  ]
}
```

### 参数

| 参数          | 类型   | 默认值    | 描述                                            |
| ------------- | ------ | --------- | ----------------------------------------------- |
| `Columns`     | string | `"8"`     | 每行列数                                        |
| `ShowTitle`   | string | `"false"` | 是否在图标下方显示标题（`"true"` 或 `"false"`） |
| `ItemPadding` | string | `"12"`    | 网格项内边距（像素）                            |
| `ItemMargin`  | string | `"6"`     | 网格项外边距（像素）                            |

### 结果结构

使用网格布局时，每个结果应包含：

- **Icon**：网格单元格中显示的主要视觉元素（必需）
- **Title**：如果 `ShowTitle` 为 `"true"`，则显示在图标下方（过长时省略号截断）
- **Group**：可选分组，用于将项目组织成带标题的分区

### 示例：表情选择器插件

```json
{
  "Id": "emoji-picker-plugin",
  "Name": "Emoji Picker",
  "MinWoxVersion": "2.0.4",
  "TriggerKeywords": ["emoji"],
  "Features": [
    {
      "Name": "gridLayout",
      "Params": {
        "Columns": "12",
        "ShowTitle": "false",
        "ItemPadding": "12",
        "ItemMargin": "6"
      }
    }
  ]
}
```

```python
from wox_plugin import Plugin, Context, Query, QueryResponse, Result

class EmojiPlugin(Plugin):
    async def query(self, ctx: Context, query: Query) -> QueryResponse:
        emojis = ["😀", "😃", "😄", "😁", "😅", "😂", "🤣", "😊"]
        return QueryResponse(results=[
            Result(
                title=emoji,
                icon=f"emoji:{emoji}",
                group="笑脸"
            )
            for emoji in emojis
        ])
```

直接返回 `list[Result]` 已 deprecated。Python host 仍会为了兼容旧版 Wox 继续接受。只有当 `plugin.json` 声明 `MinWoxVersion` >= `2.0.4` 时，才应返回 `QueryResponse`。

### 分组项目

使用 `group` 字段将网格项目组织成分区。具有相同 group 值的项目会显示在同一个分组标题下：

```python
results = [
    Result(title="😀", icon="emoji:😀", group="笑脸"),
    Result(title="😃", icon="emoji:😃", group="笑脸"),
    Result(title="❤️", icon="emoji:❤️", group="爱心"),
    Result(title="💙", icon="emoji:💙", group="爱心"),
]
```

这会生成带有"笑脸"和"爱心"分区标题的布局，每个分区下是对应的表情网格。

### 布局计算

网格会根据以下规则自动计算项目尺寸：

1. 可用宽度 ÷ 列数 = 单元格宽度
2. 图标尺寸 = 单元格宽度 - (ItemPadding + ItemMargin) × 2
3. 单元格高度 = 单元格宽度 + 标题高度（如果启用 ShowTitle）

调整 `ItemPadding` 和 `ItemMargin` 可控制项目间距。较大的值会增加留白空间，较小的值可在屏幕上容纳更多项目。
