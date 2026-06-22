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

### 插件安装失败怎么办？

1. 确认能访问插件商店和插件 release 地址。
2. 检查插件是否需要 Node.js 或 Python。
3. 打开 Wox 日志目录，查看最新 core 日志和 plugin host 日志。
4. 如果刚安装运行时，重启 Wox 后再执行一次 `wpm`。

### 如何更新插件？

运行 `wpm`，选中插件，在有可用更新时执行更新动作。也可以从插件管理器设置中管理已安装插件。

## 文件搜索

### Wox 必须安装 Everything 吗？

不必须。Wox 有自己的 File 插件，会索引你在插件设置中配置的根目录。只有当你想在 Wox 之外也使用 Everything 时，才需要安装 [Everything](https://www.voidtools.com/)。

### macOS 文件搜索为什么提示权限？

macOS 可能会限制 Desktop、Documents、Downloads、外置磁盘等位置。若搜索状态或日志提示权限问题，在 **系统设置 -> 隐私与安全性** 中给 Wox 对应的文件访问权限。

## 自定义

### 如何修改主题？

在 Wox 中运行 `theme`，或打开 **设置 -> 主题**。

### 如何修改快捷键？

打开 **设置 -> 常规**，编辑快捷键字段。

## Wayland

### 在 Wayland 下如何使用双修饰键热键或 CapsLock 组合键？ {#wayland-double-modifier-hotkeys}

在 Wayland 下，Wox 无法像在 X11 上那样通过显示服务器全局拦截原始按键事件。为了启用双修饰键热键（如 `ctrl+ctrl`、`shift+shift`）和 CapsLock 组合键热键（如 `capslock+a`），Wox 会直接从 Linux evdev 接口读取键盘事件。

所需权限取决于你要使用哪种热键：

#### 双修饰键热键（如 `ctrl+ctrl`）— 只需要 `input` 组

这授予对 `/dev/input/event*` 设备的读权限。Wox 被动监听键盘事件，不会 grab 或重映射键盘。

```bash
sudo usermod -aG input $USER
```

重新登录，然后重启 Wox。

#### CapsLock 组合键（如 `capslock+a`）— 需要 `input` 和 `uinput` 两个组

除了 `input` 组之外，CapsLock 组合键还需要 `uinput` 组。当 CapsLock 被用作组合键前缀时，由于 Wox 在 Wayland 下无法拦截原始事件，系统会切换 CapsLock 状态。Wox 通过 uinput 虚拟键盘注入一个 CapsLock 按键事件来撤销这个切换。如果没有 uinput，每次使用 CapsLock 组合键时大小写灯都会被切换。

```bash
sudo groupadd -r uinput 2>/dev/null
sudo usermod -aG input,uinput $USER
```

添加 udev 规则确保 `/dev/uinput` 权限正确：

```bash
echo 'KERNEL=="uinput", MODE="0660", GROUP="uinput"' | sudo tee /etc/udev/rules.d/99-uinput.rules
sudo udevadm control --reload-rules && sudo udevadm trigger
```

重新登录，然后重启 Wox。

设置完成后，单独按下 CapsLock 时正常切换大小写；将 CapsLock 用作组合键前缀时，系统的 CapsLock 切换会被自动撤销。普通组合键热键（如 `ctrl+space`）不受此设置影响，始终通过 `org.freedesktop.portal.GlobalShortcuts` portal 工作。

> **注意：** 此方案不需要 root 权限或系统守护进程。Wox 只是被动读取 evdev 事件，仅在组合键触发后使用 uinput 注入一个 CapsLock 按键事件来恢复大小写状态。
