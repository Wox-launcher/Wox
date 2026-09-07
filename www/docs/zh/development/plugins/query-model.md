# 查询模型

Wox 会把用户的每次输入或选择整理为一个 `Query` 对象发送给插件。理解分段方式可以让插件行为更可控。

## 查询类型

- `input`：普通文本输入，例如 `wpm install wox`。
- `selection`：选中/拖拽的数据（文本/文件/图片）。仅当插件在 `Features` 中声明 `querySelection` 时才会送达。

## 查询结构

| 字段 | 说明 |
| --- | --- |
| `RawQuery` | 完整查询文字，按原有规则解析，包含触发关键字；Hint 不覆盖此内容。 |
| `QueryHint` | `input` 查询可选的有序元素列表，见下文结构化查询。 |
| `TriggerKeyword` | `plugin.json` 中声明的关键字之一。`"*"` 表示全局触发，空值代表全局查询（注册了 `*` 时）。 |
| `Command` | 触发关键字后的命令段，来源于 `plugin.json` 的 `Commands`。 |
| `Search` | 去掉触发关键字和命令后的剩余部分。 |
| `Selection` | `Type=selection` 时携带，含 `Type`、`Text`、`FilePaths`，仅在启用 `querySelection` 时提供。 |
| `Env` | 额外环境信息（活动窗口标题/进程/图标、浏览器 URL 等），仅在启用 `queryEnv` 时提供。 |

`wpm install wox` 拆分示例：

- `TriggerKeyword`：`wpm` 
- `Command`：`install`
- `Search`：`wox`
- `RawQuery`：`wpm install wox`

## 查询环境 (`queryEnv` 功能)

当 `Features` 包含 `queryEnv` 时，Wox 会附加：

- `ActiveWindowTitle`
- `ActiveWindowPid`
- `ActiveWindowIcon`（WoxImage）
- `ActiveBrowserUrl`（需要安装 Wox Chrome 扩展且浏览器为活动窗口）

可以通过 feature 参数声明只需要的字段（见 [规范](./specification.md)）。

## 特殊查询变量

Wox 在把查询交给插件前会展开以下占位符：

- `{wox:selected_text}`
- `{wox:active_browser_url}`
- `{wox:file_explorer_path}`

适合用来把当前选中文本或文件管理器路径作为搜索种子传给插件。


## 结构化查询

`QueryHint` 是 `input` 查询的可选字段，用于行内参数和整块操作的内容。
它不是新的查询类型，也不是在 `QueryText` 中嵌入 `<woxblock>` 等标记。

此能力目前在开发版本中实现。发布插件前，应确认包含该能力的首个 Wox 和 SDK
版本并设置相应最低版本；单文件插件原有的运行时版本下限不代表支持结构化查询。

`QueryHint` 的设计理念是：**提供提示和背景语义，连续输入与用户编辑优先。**
它可以承载实际参数值，而不只是占位文字；但不能把查询变成强制填写的表单。
命令与参数共用一个编辑器，保留正常选择、删除、复制、粘贴和输入法行为，Tab 仅是辅助快捷操作。
当编辑使语义边界失效时，保留用户文字并撤掉不可靠的提示，不为维护结构而阻止输入。
只有明确声明的 `block` 才按整体操作，带背景的普通参数仍然可以自由编辑。

### 声明命令后面的模板

`Commands` 中的每条命令可以增加 `Aliases` 和 `QueryHint`。模板只包含
命令后面的元素，不要再写命令文字，也不要使用保留 ID `command`；Wox 会根据
实际匹配的入口补上前缀。以下是全局插件（`TriggerKeywords: ["*"]`）的命令条目：

```json
{
  "Command": "set-volume",
  "Description": "Set volume",
  "Aliases": ["set volume", "volume"],
  "QueryHint": {
    "Elements": [
      {
        "Id": "volume",
        "Kind": "argument",
        "Placeholder": "Volume (0–100)",
        "Required": true
      }
    ]
  }
}
```

对于触发词为 `gh`、命令为 `issues` 的插件，入口为 `gh issues`。别名同样带上
插件触发词，不用于解析参数。静态元数据和 `RegisterQueryCommands` /
`register_query_commands` 使用同一套模板。

| Kind | 内容字段 | 行为 |
| --- | --- | --- |
| `text` | `Text` | 普通可编辑文字，也用于显式分隔符。 |
| `argument` | `Value`，可选 `Placeholder`、`Required` | 行内可编辑范围，清空后仍保留占位符。 |
| `block` | `Value` | 整体选择和删除，不能在内部逐字编辑。 |

元素 ID 必须非空且唯一，列表不支持嵌套。第一版不支持下拉框、自定义渲染和文本
标记解析。占位符支持常规 `i18n:` 翻译。`Required` 是声明信息，插件仍须检查
空值、数值范围等约束，再提供和执行动作；查询过程中不要直接修改音量。

### 读取结构化值

Node.js 使用 `query.QueryHint.Elements`，Python 使用
`query.query_hint.elements`，按元素 ID 找到参数的 `Value` / `value`。
结构存在但参数为空时仍是结构化查询，不能回退为普通文本解析。
只有结构不存在时，才沿用原来的 `Search` / `search` 解析逻辑。

Wox 会继续提供 `RawQuery`、`Command`、`Search`：按顺序连接元素的真实内容，
不包含占位符。需要空格时显式加入 `text` 分隔元素。这种文本表示会丢失参数边界，
不要通过拆分它来重建结构。Hint 不携带插件身份、不强制路由，也不隐式创建 Scope。
查询仍按原有文字和显式 `QueryScope` 路由。完整实例需要包含适用的触发词（例如 `gh issues `），
Wox 不会根据 `ChangeQuery` 的调用插件推断或补充触发词。

### 插件主动打开完整实例

`ChangeQuery` 与模板不同，需要包含命令前缀。以下代码放在结果动作中，
`api` / `self.api` 和 `ctx` 来自插件现有上下文。

`ChangeQuery` 必须传入 `QueryType`，并按类型提供完整的 `QueryText`（input）或
`QuerySelection`（selection），即使携带 Hint 也不能省略。`QueryHint` 只是可选的视觉优化，
不能替代、生成或覆盖查询内容。输入 Hint 必须与完整 `QueryText` 一致，包含触发词、命令、
分隔符和参数；Hint 格式非法或与查询不一致时忽略 Hint，以 `QueryText` 为准继续查询。去掉 Hint 后，查询文本与路由必须保持相同。
显式传入 `QueryText: ""` 仍可清空输入。

```typescript
await api.ChangeQuery(ctx, {
  QueryType: "input",
  QueryText: "set volume 50",
  QueryHint: {
    Elements: [
      { Id: "command", Kind: "text", Text: "set volume " },
      { Id: "volume", Kind: "argument", Value: "50", Placeholder: "Volume (0–100)", Required: true }
    ]
  }
})
```

```python
from wox_plugin import ChangeQueryParam, QueryElement, QueryHint, QueryType

await self.api.change_query(ctx, ChangeQueryParam(
    query_type=QueryType.INPUT,
    query_text="set volume 50",
    query_hint=QueryHint(elements=[
        QueryElement(id="command", kind="text", text="set volume "),
        QueryElement(id="volume", kind="argument", value="50",
                     placeholder="Volume (0–100)", required=True),
    ]),
))
```

这些 API 用于 SDK 和单文件 SDK 插件，不要假定脚本插件的受限 `change-query`
动作也支持相同的结构化参数。

### 交互与兼容

- 输入完整且无歧义的命令后展示提示，空格或 Tab 激活模板。
- 命令和参数共用一个连续文本编辑器；Tab / Shift+Tab 选中元素范围，跳过空白分隔符。
- 单个参数不加装饰，保持一行连续文本。多个参数使用轻背景标记，空槽位的占位名各自成块，避免名称或取值里的空格被当成下一个参数。占位提示不参与选择、复制和参数值。
- 保留正常逐字、逐词删除和跨元素选择。跨边界编辑保留用户文字，撤掉无法可靠对应的结构。
- 重新显示并全选时选中整个查询，直接输入会替换命令和所有参数。撤销和历史保留结构，block 仍按整体操作。
- 整段粘贴带参数的命令保持普通文本，不自动解析；参数范围内粘贴会更新该参数值。
- 没有 `QueryHint` 的查询保持原有行为；`ChangeQuery` 的查询内容始终以显式的 `QueryText` / `QuerySelection` 为准。

接入时覆盖空值、合法/非法值、多槽位焦点、重新显示后的整体替换、撤销和旧文本路径。
Set Volume 是首个内置接入示例，音量范围规则由该插件负责，不是通用查询协议的一部分。
