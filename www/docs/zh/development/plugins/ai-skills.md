# 用于插件开发的 AI Skills

Wox 默认内置 [`wox-plugin-creator`](https://github.com/Wox-launcher/Wox/tree/master/.agents/skills/wox-plugin-creator)，用于辅助插件开发。它始终可用，不能在 Wox 设置中修改或删除。外部 agent 也可以从仓库单独安装同一份 skill。

## 为什么推荐使用

这些 Wox skills 把插件开发相关的项目知识打包好了，agent 不需要每次都从零推断 Wox 的约定和细节。

对于插件开发，通常会带来这些收益：

- 更快地创建 Python、Node.js、脚本插件和单文件 SDK 插件脚手架
- 更准确地编写 `plugin.json`
- 更清楚地处理 `SettingDefinitions`、validator、dynamic settings 和 i18n
- 更明确地指导发布到 Wox 商店

## 推荐 Skill

优先使用 `wox-plugin-creator`。

它是 Wox 插件开发的主 skill，覆盖内容包括：

- 插件脚手架创建
- SDK 用法
- `plugin.json` 元数据
- settings 和 validator 模式
- script plugin 模板
- 单文件 SDK 插件模板
- 发布到 Wox 商店

## 适用场景

当你希望 agent 协助下面这些任务时，建议使用这个 skill：

- 创建新插件
- 把一个想法快速落成 Wox 插件脚手架
- 编辑 `plugin.json`
- 实现设置界面
- 添加 validator 或 dynamic settings
- 准备发布到 Wox 商店

## 说明

- 是否在对话中使用该 skill 是可选的；Wox 内置副本会始终保留。
- 当 agent 在 Wox 相关工作区内工作，并且能读取 skill 自带的 references 时，这个 skill 的效果最好。
