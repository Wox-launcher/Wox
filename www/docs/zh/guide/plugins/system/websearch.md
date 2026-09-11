# 网页搜索插件

网页搜索插件会从 Wox 打开搜索 URL。它可以作为普通文本的 fallback 结果，也可以通过明确的搜索引擎关键字触发。

## 快速开始

```text
Wox Launcher
g Wox Launcher
```

默认配置包含 Google，关键字为 `g`。你可以在插件设置中添加更多搜索引擎。

![网页搜索插件结果列表](/images/system-plugin-websearch.png)

## 搜索引擎设置

| 字段 | 用途 |
| --- | --- |
| Keyword | 查询前使用的快捷关键字，例如 `g` |
| Title | Wox 中显示的结果标题 |
| URL(s) | 搜索 URL 模板 |
| Browser | 打开该搜索的浏览器 |
| Incognito | 在浏览器支持时用无痕/隐私窗口打开。默认不勾选。 |
| Enabled | 是否显示该搜索引擎 |
| Default | 是否用于 fallback 搜索 |

## URL 变量

| 变量 | 值 |
| --- | --- |
| `{wox:parameter?name=query}` | 命名输入参数 |
| `{wox:parameter?name=query&case=lower}` | 参数值的小写形式 |
| `{wox:selected_text}` | 唤起 Wox 前的选中文本 |
| `{wox:clipboard_text}` | 唤起 Wox 前获取的剪贴板文本 |

示例 URL：

```text
https://www.google.com/search?q={wox:parameter?name=query}
```

如果一个搜索引擎配置了多个 URL，Wox 会按顺序打开。

已有标题和 URL 中的 `{query}`、`{lower_query}`、`{upper_query}` 会自动迁移为
`{wox:parameter?name=query}` 以及带 `case=lower` / `case=upper` 的形式。

多参数示例：
`https://example.com/search?text={wox:parameter?name=text}&language={wox:parameter?name=language}`。
输入关键词和空格后进入专属搜索范围，并显示参数槽位。按 Tab / Shift+Tab 切换槽位，
每个参数的值都可以包含空格。参数按其在 URL 列表中首次出现的顺序排列，同名参数只输入一次。
名称可用任意语言的文字、数字、下划线和空格，不能以数字或空格开头。标题只能引用 URL 中声明的输入参数。
在标题和 URL 编辑框中输入 `{` 或点击 `{}` 可插入变量，完整的 `{wox:...}` 会显示为芯片。
如果标题引用了 URL 中不存在的参数，保存会被拒绝。

单参数搜索仍接收关键词后的整段文本。粘贴多参数的纯文本查询时，会提供填写参数操作，
并将整段文本保留在第一个槽位中，不猜测参数边界。
变量只替换一次，按所在的 URL 路径、查询或片段编码，不支持替换协议或主机名。
环境文本不可用时会显示原因，不打开不完整的搜索。

## 选中文本

当你对选中文本触发 Wox 时，网页搜索可以为这段选中文本显示 fallback 搜索引擎。它适合快速搜索错误信息、符号名或其他应用里的短语。

Fallback 和选中文本搜索都要求恰好一个不同的输入参数。环境变量不计入参数数量；
零参数和多参数搜索即使勾选 fallback，也不会参与。
