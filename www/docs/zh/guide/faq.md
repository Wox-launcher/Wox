# 常见问题

## 启动和日志

### Wox 启动不了，先看哪里？

先打开 core 日志：

| 平台 | Core 日志 |
| --- | --- |
| Windows | `%USERPROFILE%\.wox\log\wox.log` |
| macOS | `~/.wox/log/wox.log` |
| Linux | `~/.wox/log/wox.log` |

优先看最新的 core 日志。如果 UI 能打开但某个插件失败，再看同一数据目录下的插件日志。

### 如何重置 Wox？

退出 Wox 后删除用户数据目录：

| 平台 | 数据目录 |
| --- | --- |
| Windows | `%USERPROFILE%\.wox` |
| macOS | `~/.wox` |
| Linux | `~/.wox` |

这会删除设置、已安装插件、插件数据、缓存和日志。

## 搜索

### 为什么应用、文件或书签搜不到？

- 新安装的应用可能需要几秒钟才完成索引。
- 文件搜索只会返回配置根目录下、且 Wox 有权限读取的路径。
- 浏览器书签来自受支持的浏览器 profile，浏览器同步可能有延迟。
- 打开对应插件设置，确认插件处于启用状态。

### 为什么结果太杂？

明确使用插件关键字。例如 `f report` 搜文件，`cb report` 搜剪贴板。全局查询会让多个插件一起回答，这是预期行为。

## 插件

### 如何安装下载的 `.wox` 插件？

双击 `.wox` 文件。Wox 会打开插件安装界面，由你确认安装、升级、重装或降级。也可以先选中该文件再唤起 Wox，走的是同一套安装器。

Windows 和 Linux 上，安装或更新 Wox 后需要先启动一次，才会注册 `.wox` 文件关联。

### 插件安装失败怎么办？

1. 确认能访问插件商店和插件 release 地址。
2. 检查插件是否需要 Node.js 或 Python。
3. 打开 Wox 日志目录，查看最新 core 日志和 plugin host 日志。
4. 如果刚安装运行时，重启 Wox 后再执行一次 `wpm`。

### 如何更新插件？

运行 `wpm`，选中插件，在有可用更新时执行更新动作。也可以在 **设置 -> 插件 -> 已安装插件** 中管理插件。

## 文件搜索

### Wox 必须安装 Everything 吗？

不必须。Wox 有自己的 File 插件。在 Windows 上可以开启 **Fast Indexing**，技术和 [Everything](https://www.voidtools.com/) 一样，都是读 NTFS 的 MFT 和 USN 日志，因此可以索引整块磁盘，而不必遍历每个文件夹。macOS 和 Linux 仍使用常规的根目录索引。

只有当你想在 Wox 之外也使用 Everything 时，才需要安装 Everything。Fast Index 的设置见 [文件搜索](./plugins/system/file.md#快速索引windows)。

### macOS 文件搜索为什么提示权限？

macOS 可能会限制 Desktop、Documents、Downloads、外置磁盘等位置。若搜索状态或日志提示权限问题，在 **系统设置 -> 隐私与安全性** 中给 Wox 对应的文件访问权限。

## 自定义

### 如何修改主题？

在 Wox 中运行 `theme`，或打开 **设置 -> 主题**。

### 如何修改快捷键？

打开 **设置 -> 热键**，修改主热键、选中热键和快捷键查询。托盘查询位于 **设置 -> 常规**。见 [快捷键](./usage/hotkeys.md)。

### Explorer 去哪了？

文件资源管理器搜索已改名为 **快速跳转**，触发关键字是 `jump`。在资源管理器、Finder 或打开/保存对话框中打开 Wox，输入即可跳转。详见 [快速跳转](./plugins/system/explorer.md)。

### 如何反馈问题？

查询 `feedback` 可以导出诊断信息、查看崩溃报告、清理日志，或打开 GitHub issue。也可以查询 `doctor` 做常见配置检查。

## Wayland

### 在 Wayland 下如何使用双修饰键热键或 CapsLock 组合键？ {#wayland-double-modifier-hotkeys}

在 Wayland 下，Wox 无法像在 X11 上那样通过显示服务器全局拦截原始按键事件。为了启用双修饰键热键（如 `ctrl+ctrl`、`shift+shift`）和 CapsLock 组合键热键（如 `capslock+a`），Wox 会直接从 Linux evdev 接口读取键盘事件。

所需权限取决于你要使用哪种热键：

#### 双修饰键热键（如 `ctrl+ctrl`）— 只需要 `input` 组

这授予对 `/dev/input/event*` 设备的读权限。Wox 被动监听键盘事件，不会 grab 或重映射键盘。

```bash
sudo usermod -aG input $USER
```

保存工作后，**重启电脑**，让新增的组成员资格生效，然后打开 Wox。运行 `id -nG`，确认当前会话包含 `input`。

注销并重新登录后，`systemd --user` 仍可能沿用旧的用户组。由这个用户服务管理器启动的桌面和应用，即使在重新登录后也可能继承旧权限。重启电脑可以确保它们使用更新后的组成员资格。

#### CapsLock 组合键（如 `capslock+a`）— 需要 `input` 组，推荐 `uinput` 组

CapsLock 组合键需要 `input` 组（evdev 读取权限）来检测组合键。`uinput` 组**不是**注册或触发热键的必需条件——它仅在组合键触发后用于恢复 CapsLock 状态并删除多打的组合字符。

当 CapsLock 被用作组合键前缀时，由于 Wox 在 Wayland 下无法拦截原始事件，系统会切换 CapsLock 状态。Wox 通过 uinput 虚拟键盘注入一个 CapsLock 按键事件来撤销这个切换。如果没有 uinput，热键仍会触发，但大小写灯可能被切换，并可能在当前输入框里多输入一个字符。

要启用完整的 CapsLock 状态恢复，以普通用户打开终端，复制并运行下面整个代码块即可。它会添加所需的用户组、配置设备权限、加载 `uinput` 并设置开机自动加载。如果某条命令失败，脚本会停止，请先检查错误再继续。

```bash
sudo bash -s -- "$USER" <<'EOF'
set -eu
getent group uinput >/dev/null || groupadd -r uinput
usermod -aG input,uinput "$1"
echo 'KERNEL=="uinput", MODE="0660", GROUP="uinput"' > /etc/udev/rules.d/80-uinput.rules
echo uinput > /etc/modules-load.d/uinput.conf
udevadm control --reload-rules
modprobe uinput
udevadm trigger --action=change /sys/class/misc/uinput
udevadm settle
EOF
```

脚本成功执行后，保存工作并**重启电脑**，让新增的组成员资格生效，然后检查：

```bash
id -nG
ls -l /dev/uinput
test -w /dev/uinput && echo "uinput writable"
```

`id -nG` 应包含 `input` 和 `uinput`，设备应显示 `crw-rw---- root uinput`，最后一条命令应输出 `uinput writable`。确认后打开 Wox，运行 `doctor` 检查权限。

遇到错误时：

- **`udevadm trigger` 报 `No such device`**：先执行 `sudo modprobe uinput`。检查 `/sys/class/misc/uinput` 是否存在，再触发上面的 udev 命令；仅创建组或写入规则不会加载模块。
- **`modprobe` 报 `Module uinput not found`**：检查当前内核是否提供该模块。如果刚升级过内核，先重启到新内核再重试。
- **设备仍是 `root:root` 或没有组写权限**：检查 udev 规则是否保存正确，重新加载规则、触发设备事件并等待 `udevadm settle` 完成。
- **设备权限正确但仍不可写**：用不带用户名的 `id -nG` 检查当前会话的组。`id -nG "$USER"` 显示的是账户配置中的组，可能与当前会话实际生效的组不同。如果账户配置包含 `input` 和 `uinput`，但当前会话没有，请重启电脑；只重开终端或注销后重新登录仍可能沿用旧的用户组。如果当前会话已包含所需的组，只是修改设备权限，则重启 Wox 即可，无需重启电脑。

设置完成后，单独按下 CapsLock 时正常切换大小写；将 CapsLock 用作组合键前缀时，系统的 CapsLock 切换会被自动撤销。普通组合键热键（如 `ctrl+space`）不受此设置影响，始终通过 `org.freedesktop.portal.GlobalShortcuts` portal 工作。

> **注意：** 上述系统配置需要 `sudo`，但 Wox 本身以普通用户运行，无需额外的系统守护进程。在 CapsLock 组合键流程中，Wox 被动读取 evdev 事件，并通过 uinput 恢复大小写状态、删除多打的组合字符。没有 uinput 时热键仍能触发，但这些恢复操作无法完成。
