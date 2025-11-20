# 常见问题 (FAQ)

## 通用

### Wox 无法启动？

检查日志文件：

- Windows: `%USERPROFILE%\\.wox\\log`
- macOS/Linux: `~/.wox/log`

### 如何重置 Wox？

删除用户数据目录：

- Windows: `%USERPROFILE%\\.wox`
- macOS/Linux: `~/.wox`

## 插件

### 插件安装失败？

- 检查您的网络连接。
- 如果插件需要，请确保已安装所需的运行时（Python/Node.js）。
- 查看日志以获取详细的错误信息。

### 如何更新插件？

使用 `wpm update` 命令更新所有插件或特定插件。

### Everything 插件?

Wox 内置文件插件（触发 `f`）依赖 Everything 引擎。请安装并运行 [Everything](https://www.voidtools.com/)，确保其服务已启动并完成索引；让 Everything 在后台运行，Wox 会调用其 API 进行查询。

## 自定义

### 如何更改主题？

在 Wox 中输入 `theme` 列出可用主题，或前往 设置 -> 主题 进行选择。

### 如何更改快捷键？

前往 设置 -> 常规 -> 快捷键。
