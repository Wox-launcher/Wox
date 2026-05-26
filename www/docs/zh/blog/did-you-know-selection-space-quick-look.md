---
title: 你知道吗：Wox 可以用空格键快速预览文件
description: 在 Selection 插件里启用空格快速预览后，可以像 macOS Quick Look 一样，从文件管理器或打开/保存对话框里直接预览选中的文件。
date: 2026-05-26
---

# 你知道吗：Wox 可以用空格键快速预览文件

macOS 用户很熟悉 Finder 里的空格预览：选中文件，按一下空格，不用打开完整应用就能快速看一眼。Wox 的 Selection 插件也可以打开类似流程：选中一个文件，按空格，Wox 会直接打开一个聚焦在文件预览上的面板。

<video src="/videos/did-you-know-selection-space-quick-look.mp4" controls muted loop playsinline style="width: 100%; border-radius: 8px;"></video>

先在 Selection 插件设置里开启这个选项：

| Selection 设置 | 值 |
| --- | --- |
| 启用空格快速预览 | 开启 |

之后，在 Wox 能读取当前文件选区的位置使用它：

1. 在文件资源管理器、Finder 或打开/保存对话框里选中一个文件。
2. 按下空格键。
3. Wox 会打开只显示文件预览的面板。

这个功能适合快速检查文档、图片、压缩包、配置文件等内容，但又不想切到完整关联应用的场景。预览模式会把结果列表收起来，让可用空间尽量留给文件内容本身。

Selection 插件原来的选中内容操作仍然保留。如果你用选中文件触发 Wox，还是可以复制路径、打开所在文件夹，或者使用普通的预览结果。空格快速预览只是当文件已经被选中时最快的入口。

平台说明：空格快速预览目前支持 Windows 和 macOS。
