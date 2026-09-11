---
title: "面向 Linux Wayland 的跨平台启动器"
description: "Wox 是原生 Linux 启动器，支持 Wayland layer-shell。快捷键和 portal 限制见文档。"
---

# 面向 Linux Wayland 的跨平台启动器

<ReleaseStamp />

Wox 在 Linux 上是原生启动器，不是 Electron。在 Wayland 下它作为 layer-shell 浮层显示，普通全局快捷键走桌面 portal。

## 是什么

Wox 的 Linux 正式版和 Windows、macOS 一起发。主窗口是 overlay 层上的 layer-shell 表面（namespace 为 `gtk-layer-shell`），可以盖在其他窗口上，但不是普通的 XDG 顶层窗口。

像 `Ctrl+Space` 这样的常规组合键走 `org.freedesktop.portal.GlobalShortcuts`。双修饰键和 CapsLock 组合键需要额外输入权限，因为 Wayland 不能像 X11 那样让 Wox 拦截原始按键。

## 限制从哪来

动画、全局快捷键和剪贴板 portal 由合成器管理。Wox 不会用 root 守护进程绕过这些接口。正因为是 layer-shell，针对普通应用窗口的合成器规则也关不掉 Wox 的开关动画——需要写 layer 规则。

这些限制在当前的 Wayland 合成器上仍然成立。

## 怎么设置

- [Wayland 下的双修饰键和 CapsLock 组合键](/zh/guide/faq.html#wayland-double-modifier-hotkeys)
- [在 Wayland 下关闭 Wox 窗口动画](/zh/guide/faq.html#wayland-disable-animation)
- [安装](/zh/guide/installation)
