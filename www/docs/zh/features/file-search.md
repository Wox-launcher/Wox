---
title: "Windows、macOS 和 Linux 上的文件搜索启动器"
description: "Wox 在 Windows、macOS 和 Linux 上索引你选择的文件夹。Windows 从 v2.4.2 起可以使用 Fast Index（NTFS MFT/USN）。"
---

# Windows、macOS 和 Linux 上的文件搜索启动器

<ReleaseStamp />

Wox 可以在启动器里搜索本地文件，支持 Windows、macOS 和 Linux。根目录由你选择。结果和应用、剪贴板项出现在同一份列表里。

![文件插件](/images/guide/plugin_file.png)

## 它是什么

想只要文件结果、不要全局混合结果时，使用 `f` 关键字。Wox 会索引你配置的目录，并跳过常见的系统/生成目录。

空的 `f` 查询会显示系统最近文件：Windows 跳转列表、macOS Spotlight 最近使用时间，以及 Linux 的 `recently-used.xbel`。

在 **Windows** 上，文件搜索可以使用可选的 **Fast Index**：通过 NTFS 服务读取卷的 MFT 和 USN 日志，让大磁盘保持更新，而不必完整遍历。Fast Index 在 v2.4.2 加入。macOS 和 Linux 继续使用常规的根目录索引。

## 为什么不是只能用 Everything

Wox 不依赖 [Everything](https://www.voidtools.com/)。文件名搜索使用 Wox 自己的索引。内容搜索目录和文件名搜索根目录是分开配置的。

## 怎么用

根目录、忽略规则、Fast Index、内容搜索和排查见 [文件插件指南](/zh/guide/plugins/system/file)。
