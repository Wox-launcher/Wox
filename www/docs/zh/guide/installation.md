# 安装

优先选择你平时管理桌面应用的方式。包管理器更适合后续升级；Release 可执行文件更适合需要便携目录的场景。Wox 默认使用稳定版更新通道。

## 包管理器

| 平台 | 方式 | 命令 |
| --- | --- | --- |
| macOS | Homebrew | `brew install --cask wox` |
| Windows | Winget | `winget install -e --id Wox.Wox` |
| Windows | Scoop | `scoop install extras/wox` |
| Arch Linux | AUR | `yay -S wox-bin` |
| Debian / Ubuntu | `.deb` | 从 [Releases](https://github.com/Wox-launcher/Wox/releases) 下载 `wox-linux-amd64.deb`，然后执行 `sudo apt install ./wox-linux-amd64.deb` |
| Fedora / RHEL | `.rpm` | 从 [Releases](https://github.com/Wox-launcher/Wox/releases) 下载 `wox-linux-amd64.rpm`，然后执行 `sudo dnf install ./wox-linux-amd64.rpm` |
| openSUSE | `.rpm` | 从 [Releases](https://github.com/Wox-launcher/Wox/releases) 下载 `wox-linux-amd64.rpm`，然后执行 `sudo zypper install ./wox-linux-amd64.rpm` |

安装完成后，从应用启动器打开 Wox，或运行一次已安装的可执行文件。首次启动时 Wox 会创建用户数据目录。

## 手动下载

如果你的平台暂时没有包管理器入口，或你希望把 Wox 放在固定目录中运行，可以从 [GitHub Releases](https://github.com/Wox-launcher/Wox/releases) 下载最新稳定版安装包。

## 更新通道

Wox 默认检查稳定版通道。如需接收测试版预发布版，打开 **设置 -> 更新 -> 更新通道** 并选择 **测试版通道**。测试版通道用户会收到 beta 预发布版和后续稳定版正式版；稳定版通道用户不会自动收到预发布版。

### Windows

1. 从 Releases 下载 `wox-windows-amd64.exe`。
2. 将它放到你自己管理的目录，例如 `C:\Tools\Wox`。
3. 运行下载的可执行文件。

如果希望设置和插件跟着这份副本走，请在第一次启动前，在可执行文件旁边建一个 `wox-user` 文件夹。见[便携用户数据](#便携用户数据)。

如果 Windows SmartScreen 弹出确认，请先确认文件来自官方 Wox Release 页面。

### macOS

1. 从 Releases 下载 macOS 磁盘镜像。
2. 打开镜像，把 Wox 拖入 `Applications`。
3. 从 `Applications` 启动 Wox。

如果 macOS 第一次阻止启动，可以在 Finder 中右键 Wox，选择 **打开**，再在确认弹窗中继续。

### Linux

1. 优先使用安装包：
   - Debian / Ubuntu：从 Releases 下载 `.deb`，执行 `sudo apt install ./wox-linux-amd64.deb`
   - Fedora / RHEL：从 Releases 下载 `.rpm`，执行 `sudo dnf install ./wox-linux-amd64.rpm`
   - openSUSE：从 Releases 下载 `.rpm`，执行 `sudo zypper install ./wox-linux-amd64.rpm`
   - Arch Linux：从 AUR 安装 `wox-bin`
   - AppImage：下载 `wox-linux-amd64.AppImage`，赋予执行权限后直接运行
2. 也可以从 Releases 下载 `wox-linux-amd64`，放到稳定目录中，赋予执行权限后运行。

如果二进制或 AppImage 没有执行权限：

```bash
chmod +x ./wox-linux-amd64
# 或
chmod +x ./wox-linux-amd64.AppImage
```

## 用户数据

Wox 会把设置、插件数据、缓存和日志放在应用目录之外：

| 平台 | 数据目录 | Core 日志 |
| --- | --- | --- |
| Windows | `%USERPROFILE%\.wox` | `%USERPROFILE%\.wox\log\wox.log` |
| macOS | `~/.wox` | `~/.wox/log/wox.log` |
| Linux | `~/.wox` | `~/.wox/log/wox.log` |

迁移配置时，优先备份这个目录。

### 便携用户数据

在启动某一份 Wox 之前，先在可执行文件旁边建一个 `wox-user` 目录。Wox 会把设置、插件、主题和用户数据库放在这个目录里。

日志、插件 host、缓存、模型和文件索引仍留在上面的本机数据目录中。这些文件换一台电脑通常不能共用。Wox 不会自己在可执行文件旁边创建 `wox-user`。删掉这个目录后，会回到普通的用户数据位置。

## 卸载

先删除应用本体，再决定是否保留用户数据。

### Windows

- Winget：`winget uninstall -e --id Wox.Wox`
- Scoop：`scoop uninstall wox`
- 手动安装：删除下载的可执行文件
- 完全重置：删除 `%USERPROFILE%\.wox`

### macOS

- Homebrew：`brew uninstall --cask wox`
- 手动安装：从 `Applications` 删除 Wox
- 完全重置：删除 `~/.wox`

### Linux

- AUR：用你的 AUR helper 或包管理器移除 `wox-bin`
- Debian / Ubuntu：`sudo apt remove wox`
- Fedora / RHEL：`sudo dnf remove wox`
- openSUSE：`sudo zypper remove wox`
- 手动或 AppImage 安装：删除二进制或 AppImage 文件
- 完全重置：删除 `~/.wox`
