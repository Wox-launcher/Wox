---
title: "Wox Launcher loves Omarchy"
description: "跟随 Omarchy 变化的主题、可搜索的桌面命令，以及更自然的 Hyprland 使用体验。"
date: 2026-10-28
category: product
cover: /images/blog/wox-launcher-loves-omarchy.webp
coverAlt: Wox 和 Omarchy 图标，中间是一颗爱心
---

# Wox Launcher loves Omarchy

Omarchy 为 Linux 桌面带来了统一的外观和适合键盘操作的工作流。我们希望 Wox 在这样的桌面上也能自然融入：颜色跟随桌面变化，平时使用的 Omarchy 功能也能从启动器里找到。

本文介绍的改进将在 v2.4.9 中生效。

## 跟随 Omarchy 主题变化

我们为Omarchy专门内置了 **Omarchy Auto** 主题。它读取当前 Omarchy 的配色，用于启动器的背景、文字、选中项和边框。切换 Omarchy 主题时，正在运行的 Wox 也会更新颜色，不用再为启动器手动挑选匹配的主题。

在全新安装的引导流程中，主题步骤会默认选择 Omarchy Auto。如果你已经使用其他主题，可以继续保留；也可以打开 Wox，输入 `theme`，选择 **Omarchy Auto**。之后试着切换 Omarchy 主题，再打开 Wox，就能看到启动器使用新的配色。如果想换一种外观，随时可以选择其他 Wox 主题。

![Wox 主题列表中选中了 Omarchy Auto](/images/blog/omarchy-theme-auto.png)

## 在 Wox 里搜索 Omarchy 命令

打开 Wox，搜索 Omarchy 菜单中的项目，例如 `theme` 或 `lock`。匹配的 Omarchy 命令会与其他结果一起出现，并显示 Omarchy 标识和菜单路径。选中后即可运行对应操作；由动态菜单提供的项目会打开相应的 Omarchy 子菜单。

![Wox 搜索结果中的 Omarchy 菜单命令](/images/blog/omarchy-menus.png)

Wox 会读取已安装的菜单和用户菜单扩展，因此自定义的菜单项也可以被搜索到。具体结果取决于本机菜单及其可用条件；Apps 子菜单则交给 Wox 的应用搜索处理。

## 快捷键、窗口和截图

在新的 Omarchy 配置中，启动 Wox 的默认快捷键是 **Alt+Space**；已经保存的快捷键会保留。在 Linux 上，引导流程中的查询快捷键示例使用 **Ctrl+Alt+V**，避开许多终端和应用使用的 Ctrl+Shift+V 粘贴快捷键。

在 Hyprland 上，选择已运行的应用时，Wox 可以切换到它所在的工作区并聚焦已有窗口。如果无法聚焦，Wox 会回退到启动应用。Linux 上的运行中应用结果还可以显示 CPU 和内存用量。

截图时，运行 `screenshot new`，再把鼠标移到窗口上：Wox 可以在 Hyprland 中高亮窗口以便选择。截图覆盖层也会覆盖面板所在的区域。为窗口截图添加背景时，Wox 可以找到 Omarchy 当前使用的壁纸。更多截图和编辑方法见[截图指南](/zh/features/screenshot)。

![Wox 在 Hyprland 上高亮待截图的窗口](/images/blog/omarchy-screenshot-window-selection.png)
