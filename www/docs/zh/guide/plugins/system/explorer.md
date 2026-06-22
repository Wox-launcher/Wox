# 文件夹浏览插件

Explorer 是上下文插件。当前激活窗口是文件管理器或打开/保存对话框时，它会返回文件夹导航结果。

## 在文件管理器中

当 File Explorer 或 Finder 聚焦时打开 Wox，输入当前目录下子文件夹或文件名的一部分：

```text
Documents
Downloads
project
```

选中文件夹即可跳转。需要显示位置或新窗口打开时，使用操作面板。

## 在打开/保存对话框中

当对话框处于激活状态时打开 Wox，可以跳转到常用位置：

- 当前文件管理器已打开的文件夹。
- 当前应用最近使用过的文件夹。
- Desktop、Documents、Downloads 等常用系统目录。

## 注意事项

- 没有受支持上下文时，Explorer 会保持安静，不主动刷屏。
- Windows 和 macOS 支持最完整；Linux 取决于具体桌面环境。
