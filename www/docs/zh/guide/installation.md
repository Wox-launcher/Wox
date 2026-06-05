# 安装

优先选择你平时管理桌面应用的方式。包管理器更适合后续升级；Release 压缩包更适合需要便携目录的场景。Wox 默认使用稳定版更新通道。

## 包管理器

| 平台 | 方式 | 命令 |
| --- | --- | --- |
| macOS | Homebrew | `brew install --cask wox` |
| Windows | Winget | `winget install -e --id Wox.Wox` |
| Windows | Scoop | `scoop install extras/wox` |
| Arch Linux | AUR | `yay -S wox-bin` |

安装完成后，从应用启动器打开 Wox，或运行一次已安装的可执行文件。首次启动时 Wox 会创建用户数据目录。

## 手动下载

如果你的平台暂时没有包管理器入口，或你希望把 Wox 放在固定目录中运行，可以从 [GitHub Releases](https://github.com/Wox-launcher/Wox/releases) 下载最新稳定版安装包。

## 更新通道

Wox 默认检查稳定版通道。如需接收测试版预发布版，打开 **设置 -> 通用 -> 更新通道** 并选择 **测试版通道**。测试版通道用户会收到 beta 预发布版和后续稳定版正式版；稳定版通道用户不会自动收到预发布版。

### Windows

1. 从 Releases 下载 Windows 压缩包。
2. 解压到你自己管理的目录，例如 `C:\Tools\Wox`。
3. 运行 `Wox.exe`。

如果 Windows SmartScreen 弹出确认，请先确认文件来自官方 Wox Release 页面。

### macOS

1. 从 Releases 下载 macOS 磁盘镜像。
2. 打开镜像，把 Wox 拖入 `Applications`。
3. 从 `Applications` 启动 Wox。

如果 macOS 第一次阻止启动，可以在 Finder 中右键 Wox，选择 **打开**，再在确认弹窗中继续。

### Linux

1. 从 Releases 下载 Linux 压缩包。
2. 解压到稳定目录，例如 `~/Applications/wox`。
3. 运行 `./wox`。

如果解压后没有执行权限：

```bash
chmod +x ./wox
```

## 用户数据

Wox 会把设置、插件数据、缓存和日志放在应用目录之外：

| 平台 | 数据目录 | 日志目录 |
| --- | --- | --- |
| Windows | `%USERPROFILE%\.wox` | `%USERPROFILE%\.wox\log` |
| macOS | `~/.wox` | `~/.wox/log` |
| Linux | `~/.wox` | `~/.wox/log` |

迁移配置时，优先备份这个目录。

## 卸载

先删除应用本体，再决定是否保留用户数据。

### Windows

- Winget：`winget uninstall -e --id Wox.Wox`
- Scoop：`scoop uninstall wox`
- 手动安装：删除解压目录
- 完全重置：删除 `%USERPROFILE%\.wox`

### macOS

- Homebrew：`brew uninstall --cask wox`
- 手动安装：从 `Applications` 删除 Wox
- 完全重置：删除 `~/.wox`

### Linux

- AUR：用你的 AUR helper 或包管理器移除 `wox-bin`
- 手动安装：删除解压目录
- 完全重置：删除 `~/.wox`
