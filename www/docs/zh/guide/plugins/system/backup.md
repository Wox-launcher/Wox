# 备份与恢复插件

备份插件可以在本机保存 Wox 用户数据的副本，并在之后恢复。触发关键字是 `backup` 和 `restore`。

![备份插件](/images/guide/plugin_backup.jpg)

## 快速开始

```text
backup
restore
```

备份保存在 Windows 的 `%USERPROFILE%\.wox\backup` 或 macOS、Linux 的 `~/.wox/backup` 中，不会自动导出到外部目录。

重置 Wox 前：

1. 输入 `backup`，执行备份动作。
2. 输入 `restore`，选中新备份，在操作面板中选择**打开文件夹**。
3. 将整个备份文件夹复制到待删除目录之外。删除 `.wox` 时，其中的备份也会一起删除。

要在另一份 Wox 中恢复复制的备份，将包含 `backup.json` 的完整备份文件夹放入目标机器的 `.wox/backup`，再运行 `restore` 选择它。恢复会替换当前用户数据，请先备份目标机器上的数据。

云同步可以在设备之间同步受支持的数据，见 [云同步](./cloudsync.md)。
