---
title: "Windows / macOS / Linux 文件搜索启动器"
description: "Wox 在 Windows、macOS 和 Linux 上索引你选择的目录。Windows 从 v2.4.2 起支持 Fast Index（NTFS MFT/USN）。"
---

# Windows / macOS / Linux 文件搜索启动器

<ReleaseStamp />

Wox 可以在 Windows、macOS 和 Linux 的启动器里搜本地文件。你自己选根目录，结果和应用、剪贴板出现在同一份列表里。

## 是什么

想只要文件结果时，用 `f` 关键字。Wox 会索引你配置的目录，并跳过常见的生成目录。

在 **Windows** 上，文件搜索可以打开可选的 **快速索引（Fast Index）**：通过 NTFS 服务读取卷的 MFT 和 USN，让大磁盘保持更新，而不必整盘爬取。该能力出现在 v2.4.2。macOS 和 Linux 仍使用常规的根目录索引。

![文件插件搜索结果](/images/system-plugin-filesearch.png)

## 不依赖 Everything

Wox **不需要** [Everything](https://www.voidtools.com/)。文件名搜索用的是 Wox 自己的索引。内容搜索目录和文件名根目录是分开配置的。

## 怎么用

根目录、忽略规则、快速索引和排查见[文件插件指南](/zh/guide/plugins/system/file)。
