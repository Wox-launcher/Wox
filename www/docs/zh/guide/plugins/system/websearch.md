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
| Enabled | 是否显示该搜索引擎 |
| Default | 是否用于 fallback 搜索 |

## URL 变量

| 变量 | 值 |
| --- | --- |
| `{query}` | 原始查询文本 |
| `{lower_query}` | 小写查询文本 |
| `{upper_query}` | 大写查询文本 |

示例 URL：

```text
https://www.google.com/search?q={query}
```

如果一个搜索引擎配置了多个 URL，Wox 会按顺序打开。

## 选中文本

当你对选中文本触发 Wox 时，网页搜索可以为这段选中文本显示 fallback 搜索引擎。它适合快速搜索错误信息、符号名或其他应用里的短语。
