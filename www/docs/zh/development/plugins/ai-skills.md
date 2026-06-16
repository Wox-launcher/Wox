# 用于插件开发的 AI Skills

如果你使用 Codex 或其他兼容的 agent，建议安装 [`wox.core/resource/ai/skills`](https://github.com/Wox-launcher/Wox/tree/master/wox.core/resource/ai/skills) 目录下发布的 Wox skills 来加速插件开发。

## 为什么推荐使用

这些 Wox skills 把插件开发相关的项目知识打包好了，agent 不需要每次都从零推断 Wox 的约定和细节。

对于插件开发，通常会带来这些收益：

- 更快地创建 Python、Node.js 和脚本插件脚手架
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

- skills 不是必需的，不使用它们也可以直接基于 SDK 和文档开发插件。
- 当 agent 在 Wox 相关工作区内工作，并且能读取 skill 自带的 references 时，这个 skill 的效果最好。
