# Wox 诊断插件

诊断插件检查常见配置问题：权限、运行时、更新状态，以及各平台的特殊要求。

![诊断插件](/images/guide/plugin_doctor.jpg)

## 快速开始

```text
doctor
```

按列表逐项处理。在 Linux Wayland 上，诊断也会提示双修饰键和 CapsLock 热键需要的 `input`、`uinput` 权限。见 [Wayland 常见问题](../../faq.md#wayland-double-modifier-hotkeys)。

## 内存诊断

输入 `doctor memory` 查看 Wox 和辅助进程的内存占用。可以从结果进入明细，也可以直接输入命令。

- `doctor memory go` 按类别查看 Go 运行时保留的内存。
- `doctor memory native` 按组件查看原生内存占用。
- `doctor memory processes` 查看插件宿主和辅助进程。
- `doctor memory profile` 打开保存 Go 堆分析文件的操作。

将 **Wox 内存** 选为主 Glance，即可查看当前内存占用。
