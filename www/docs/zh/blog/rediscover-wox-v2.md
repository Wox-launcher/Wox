---
title: "重新认识 Wox v2"
description: "从 2013 年的 WinAlfred，到今天的跨平台原生启动器：聊聊我为什么重新做 Wox，以及这三年里它变了什么。"
date: 2026-10-20
category: product
cover: /images/blog/rediscover-wox-v2.webp
coverDark: /images/blog/rediscover-wox-v2-dark.webp
coverAlt: 绿色手写 Wox v1 图标通过箭头过渡到白底黑色 W 的 Wox v2 图标
---

# 重新认识 Wox v2

十三年前，我（[qianlifeng](https://github.com/qianlifeng)）在 V2EX 发了一个帖子，介绍自己做的小工具 [WinAlfred](https://v2ex.com/t/93922)。当时只是想在 Windows 上用到类似 Alfred 的启动器：按一个快捷键，输入几个字母，就能打开应用，不用再翻开始菜单。

后来，这个工具有了一个新名字：Wox。

十三年过去，它经历过活跃的日子，也有过很长一段维护跟不上的时候。现在，Wox 2.x 已经可以在 Windows、macOS 和 Linux 上使用。我想借这个机会，重新介绍一下它，也聊聊为什么我又花了三年时间来做这件事。

## 从 WinAlfred 到 Wox

WinAlfred 这个名字，意思很直白，就是 Windows 上的 Alfred。后来收到了 Alfred 关于名称的警告信，需要换个名字。在社区朋友 [@Sylv 的建议](https://v2ex.com/t/98605#r_939722)下，最终选了 Wox，取的是 “it just works” 的意思。

2013 到 2015 年，我主要负责 Wox 的开发。2015 年开始，我转向使用 Mac，也把项目从个人账号转到了 Wox-launcher 组织，交由社区继续维护。[bao-qian](https://github.com/bao-qian) 接过了后续的主要维护工作，还有许多[贡献者](https://github.com/Wox-launcher/Wox/graphs/contributors?all=1)一起帮它走了下来。没有他们，Wox 走不到今天，再次感谢。

Wox 曾经是很多 Windows 用户认识启动器的起点。后来维护逐渐跟不上，问题和反馈积压，用户也开始寻找别的选择，社区里有了 Flow Launcher。对于一直留在 Wox 的用户来说，那段等待并不好受。

## 为什么重写 Wox

2023 年，我的空闲时间多了一些，重新注意到这些问题，想把 Wox 拾起来。但有一个很实际的障碍：我的主力电脑仍然是 MacBook Pro，而 Wox v1 是用 .NET 开发、面向 Windows 的应用。

如果我自己都没办法每天使用它，就很难有信心长期维护下去。跨平台的想法是这么来的。2023 年 10 月，我写下了 [Wox v2 的计划](https://github.com/Wox-launcher/Wox/discussions/3937)，开始重写。

这时候，启动器已经有了很多选择。除了 Alfred，还有 Raycast、uTools、Flow Launcher，每个工具都有自己的用户。我也反复想过，既然已经有这么多好用的产品，再做一个 Wox，应该把力气花在哪里？

我想做的，是一个能在自己不同电脑上使用的启动器，常用功能足够齐全，界面认真打磨，同时尽量少占资源。它应该随叫随到，完成操作后就退到后台。

没想到，这一做就是三年。中间一直觉得还不够好，没到可以再认真写一篇介绍、推荐大家使用的程度。直到功能逐渐完善，日常使用也稳定下来，才觉得可以重新说一声：来试试现在的 Wox 吧。

![Wox v2 的深色玻璃主题与启动器界面](/images/hero-glass-dark.png)

## 原生界面与内存占用

现在的 Wox 使用原生 GPU 渲染界面。启动器本身没有使用 Electron，也没有用 WebView 包一层网页作为应用界面。v2 的界面经历过调整，在 [v2.4.0](/zh/changelog/2.4.0) 中从 Flutter 转向了现在的嵌入式原生 UI。

对一个大部分时间藏在后台、每次可能只用几秒钟的工具，我很在意它的资源占用。目前日常查询通常在约 150 MB 内存，隐藏并空闲后，在我的使用中可以低至约 60 MB。这是日常观察，不是所有机器上的固定值；插件、索引和正在使用的功能都会影响实际占用。

外观也可以按自己的习惯调整：跟随系统切换明暗主题，或者到[主题商店](/zh/store/themes)挑一种配色、背景和窗口样式。macOS 26 上还支持 Liquid Glass 材质。

## 现在有哪些内置功能

最基础的操作还是熟悉的那一套：呼出 Wox，输入应用名，按回车打开。但现在不安装第三方插件，也有不少可以直接用的功能。

比如找文件。你可以输入 `f report`，在配置好的目录中查找名称包含 `report` 的文件和文件夹，选中结果后查看预览。下图选中的是一个文件夹，右侧会列出它的内容。Windows 上可选装 NTFS 快速索引服务，利用 MFT 和 USN 日志维护索引；macOS 和 Linux 则使用按目录建立的索引。需要搜索文件内容时，可以另外配置内容索引目录。具体入口和权限要求见[文件搜索指南](/zh/guide/plugins/system/file)。

![Wox 搜索 report，右侧预览选中文件夹的内容](/images/guide/plugin_file.png)

找回刚复制过的一段文字，可以用[剪贴板历史](/zh/guide/plugins/system/clipboard)；临时算一个数、换算单位或查看另一个时区，可以直接在查询框输入。浏览器书签、Emoji、颜色、媒体播放控制和系统命令，也都在内置工具里。

截图标注、短视频录制、笔记和倒计时，现在也都能在 Wox 里完成。语音输入可以下载模型后在本地离线运行。AI 聊天和 AI 命令则由你配置自己的模型提供商和 API Key；例如，把选中的一段话[翻译后替换回原处](/zh/blog/did-you-know-ai-command-silent-translation-query-hotkey)。模型服务的费用由对应提供商收取。

我自己也经常用快捷键打开一个 X 的小窗口，看一下就关掉。这是 [网页搜索的 WebView 选项](/zh/guide/plugins/system/websearch)配合查询热键的用法，目前支持 Windows 和 macOS。这里的 WebView 用来显示网页内容，启动器界面仍然是原生的。

如果每天都要把编辑器、浏览器和终端摆回同样的位置，还可以保存一套[工作区布局](/zh/blog/did-you-know-wox-workspace-layouts)，下次一键恢复。

## 跨平台与云同步

跨平台最初是为了解决我自己的问题：换到 Mac 后，还能继续使用自己维护的工具。现在，我希望 Windows、macOS 和 Linux 用户都能用熟悉的方式打开应用、查找文件和执行命令。系统权限和桌面环境仍然有差异，部分功能也有平台限制，需要按对应系统配置。

配置多了以后，每台电脑从头设一遍也很麻烦。[云同步](/zh/guide/plugins/system/cloudsync)可以加密同步设置、已安装插件和主题等数据。笔记正文可以同步，笔记图片和部分仅本地设置则留在设备上。

云同步有持续的服务器和流量费用，因此目前 Wox 的付费项是云同步 Pro。免费方案支持最多两台有效设备、手动同步；Pro 支持不限设备数量和自动同步。只在一台电脑上使用 Wox，不需要订阅云同步。

输入 `sync`，可以查看当前同步状态和历史记录，也可以手动发起同步。

![Wox 云同步的运行状态与同步历史](/images/guide/plugin_cloudsync.jpg)

## 插件与扩展

内置功能覆盖不了所有人的需要。[插件商店](/zh/store/plugins)里已经有社区贡献的第三方插件，也可以用 Node.js 或 Python 写自己的插件。仓库还提供了 [wox-plugin-creator skill](https://github.com/Wox-launcher/Wox/tree/master/.agents/skills/wox-plugin-creator)，可以把一个具体需求交给支持 skill 的 AI 编程工具，帮助创建插件，再在本机调试。

我一直喜欢启动器的这种可定制性：只用它打开应用也可以，想多做一点，就把自己的脚本和常用工具接进来。

如果你很久没用过 Wox，可以从[安装当前稳定版](/zh/guide/installation)开始，先拿一个每天都要做的操作试试。代码和问题反馈都在 [GitHub](https://github.com/Wox-launcher/Wox)，具体用法也会继续写在这个博客里。

从 WinAlfred 到 Wox，十三年比当时想象的长很多。谢谢这些年参与维护、提交插件、报告问题，以及一直在用它的人。希望 Wox 还有下一个十三年。

![Wox 播放彩纸效果](/images/confetti.png)
