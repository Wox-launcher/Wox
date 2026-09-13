# 简介

Wox 是一个面向键盘使用的快速启动器，支持 Windows、macOS 和 Linux。你可以用它打开应用、查找文件、写笔记、截图、倒计时、搜索网页、复用剪贴板历史、调用 AI 模型，也可以通过插件把自己的工作流接进来。

如果想比较 Wox、Flow Launcher、Raycast 或 PowerToys Run，见 [Wox 对比 Flow / Raycast / PowerToys](/zh/compare/)。

Wox 本体保持轻量。日常能力由系统插件提供，其余交给插件商店和 SDK。

## Wox 适合做什么

- **快速打开对象**：应用、文件夹、文件、书签、URL 和系统动作。
- **继续完成下一步**：每个结果都可以提供复制、显示位置、粘贴、保存到笔记、用其他工具打开等动作。
- **覆盖本地常用流程**：剪贴板、计算器、单位转换、文件搜索、笔记、截图、计时器、听写和窗口布局都可以直接从 Wox 调用。
- **通过插件扩展**：可以安装社区插件，也可以用脚本插件、Node.js SDK 或 Python SDK 写自己的插件。
- **方便排查和迁移**：用户数据默认在 Windows 的 `%USERPROFILE%\.wox`，以及 macOS/Linux 的 `~/.wox`。

## 查询是怎么工作的

Wox 会把你输入的内容路由给插件。

有些插件会监听普通输入，例如应用搜索、计算器、单位转换和网页搜索。另一些插件需要明确触发关键字：

| 示例 | 作用 |
| --- | --- |
| `f invoice` | 搜索文件 |
| `cb token` | 搜索剪贴板历史 |
| `note meeting` | 搜索笔记 |
| `timer 5m` | 开始倒计时 |
| `wpm install` | 搜索插件商店 |
| `chat explain this` | 启动 AI 对话 |

选中结果后，按 `Enter` 执行主要动作，或打开 [操作面板](./usage/action-panel.md) 查看其他动作。

## 建议的第一次使用路径

1. 先完成 [安装](./installation.md)。
2. 用默认快捷键打开 Wox：Windows 为 `Alt + Space`，macOS 为 `Command + Space`，Linux 为 `Ctrl + Space`。
3. 先输入一个应用名，确认应用搜索正常。
4. 阅读 [查询](./usage/querying.md)，了解关键字、查询提示和 fallback 结果。
5. 打开 **设置 -> 插件**，或从侧栏选择一个内置插件，按自己的使用习惯调整。
