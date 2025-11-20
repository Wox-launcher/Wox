# 查询模型

Wox 会把用户的每次输入或选择整理为一个 `Query` 对象发送给插件。理解分段方式可以让插件行为更可控。

## 查询类型

- `input`：普通文本输入，例如 `wpm install wox`。
- `selection`：选中/拖拽的数据（文本/文件/图片）。仅当插件在 `Features` 中声明 `querySelection` 时才会送达。

## 查询结构

| 字段 | 说明 |
| --- | --- |
| `RawQuery` | 用户原始输入，包含触发关键字。 |
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
