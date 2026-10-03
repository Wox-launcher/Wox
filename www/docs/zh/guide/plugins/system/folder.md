# 文件夹插件

文件夹插件会在输入路径时补全目录。它监听全局输入，所以 `~/Documents` 或 `%LOCALAPPDATA%` 这类路径可以直接变成文件夹结果。

![文件夹插件](/images/guide/plugin_folder.png)

## 快速开始

```text
~/Documents
C:\Users
%LOCALAPPDATA%
```

按 `Enter` 打开文件夹，按 `Shift+Enter` 在 Wox 内浏览该目录，之后用相同快捷键继续进入子文件夹。选中文件时，`Shift+Enter` 会浏览其所在目录；收藏的文件夹也使用相同快捷键。操作面板提供这些动作，以及在文件管理器中打开所在位置、在这里执行 Shell 命令、切换是否显示隐藏文件。

## 收藏

打开 **设置 -> 插件 -> 文件夹** 可以保存带名称的收藏路径。收藏按操作系统分开存储。

## 相关

- [文件搜索](./file.md) 按名称找文件
- [快速跳转](./explorer.md) 在资源管理器、Finder 或对话框中跳转
- [Shell](./shell.md) 在某个文件夹里运行命令
