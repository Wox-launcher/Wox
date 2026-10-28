# FAQ

## Startup and Logs

### Wox does not start. Where should I look first?

Open the core log:

| Platform | Core log |
| --- | --- |
| Windows | `%USERPROFILE%\.wox\log\wox.log` |
| macOS | `~/.wox/log/wox.log` |
| Linux | `~/.wox/log/wox.log` |

Start with the newest core log. If the UI opens but a plugin fails, check the plugin-specific log directory under the same Wox data folder.

### How do I reset Wox?

Quit Wox, then remove the Wox data directory:

| Platform | Data directory |
| --- | --- |
| Windows | `%USERPROFILE%\.wox` |
| macOS | `~/.wox` |
| Linux | `~/.wox` |

This removes settings, installed plugins, plugin data, cache, and logs.

## Search

### Why is an app, file, or bookmark missing?

- App search may need a few seconds after installing a new app.
- File search only returns paths inside configured roots and readable by Wox.
- Browser bookmarks are read from supported browser profiles; browser sync can delay updates.
- Open the related plugin settings and confirm the plugin is enabled.

### Why are results noisy?

Use an explicit keyword when you want one plugin. For example, `f report` searches files and `cb report` searches clipboard history. Global queries intentionally let multiple plugins answer.

## Plugins

### How do I install a downloaded `.wox` plugin?

Double-click the `.wox` file. Wox opens the plugin installer so you can confirm Install, Upgrade, Reinstall, or Downgrade. You can also select the file and invoke Wox, which uses the same installer.

On Windows and Linux, start Wox once after installing or updating so it can register the `.wox` file association.

### Plugin installation failed. What should I check?

1. Confirm network access to the plugin store and release host.
2. Check whether the plugin requires Node.js or Python.
3. Open the Wox log directory and inspect the newest core and plugin-host logs.
4. Try `wpm` again after restarting Wox if a runtime host was just installed.

### How do I update plugins?

Run `wpm`, select the plugin, and use the update action when one is available. You can also manage installed plugins from Plugin Manager settings.

## File Search

### Does Wox require Everything?

No. Wox has its own File plugin. On Windows it can enable **Fast Indexing**, which uses the same NTFS MFT and USN journal technique as [Everything](https://www.voidtools.com/) and can index whole volumes instead of walking every folder. macOS and Linux keep the regular root-based index.

Install Everything only if you also want to use Everything outside Wox. Fast Index setup is in [File Search](./plugins/system/file.md#fast-index-windows).

### Why does file search ask for permissions on macOS?

macOS may block access to Desktop, Documents, Downloads, removable drives, or other protected locations. Grant Wox file access in **System Settings -> Privacy & Security** if search status or logs report permission errors.

## Customization

### How do I change the theme?

Run `theme` in Wox or open **Settings -> Theme**.

### How do I change the hotkey?

Open **Settings -> General** and edit the hotkey field. Query hotkeys, tray queries, and the selection hotkey are on the same page. See [Hotkeys](./usage/hotkeys.md).

### Where did Explorer go?

File Explorer Search was renamed to **Quick Jump**. The trigger is now `jump`. Open Wox in File Explorer, Finder, or an open/save dialog and type to navigate. Details are in the [Quick Jump guide](./plugins/system/explorer.md).

### How do I report a problem?

Query `feedback` to export diagnostics, review crash reports, clear logs, or open a GitHub issue. You can also query `doctor` for common setup checks.

## Wayland

### How do I use double-modifier hotkeys or CapsLock combos on Wayland? {#wayland-double-modifier-hotkeys}

On Wayland, Wox cannot globally intercept raw key events through the display server like it does on X11. To enable double-modifier hotkeys (such as `ctrl+ctrl`, `shift+shift`) and CapsLock-combo hotkeys (such as `capslock+a`), Wox reads keyboard events directly from the Linux evdev interface.

The permissions required depend on which hotkey styles you want to use:

#### Double-modifier hotkeys (e.g. `ctrl+ctrl`) — requires `input` group only

This grants read access to `/dev/input/event*` devices. Wox passively listens to keyboard events without grabbing or remapping the keyboard.

```bash
sudo usermod -aG input $USER
```

Save your work and **restart your computer** to apply the new group membership, then open Wox. Run `id -nG` to verify that the current session includes `input`.

Logging out and back in may leave `systemd --user` running with its old groups. Desktops and applications launched by that user service manager can inherit those old groups even after a new login. Restarting the computer ensures they pick up the updated membership.

#### CapsLock combo hotkeys (e.g. `capslock+a`) — `input` group required, `uinput` group recommended

CapsLock combos need the `input` group (evdev read access) to detect the combo. The `uinput` group is **not required** to register or trigger the hotkey — it is only used to restore CapsLock state and delete the stray combo character after the combo fires.

When CapsLock is used as a combo prefix, the system toggles the caps lock state because Wox cannot consume the raw event on Wayland. Wox undoes this toggle by injecting a CapsLock key event through a temporary uinput virtual keyboard. Without uinput, the hotkey still fires, but the caps lock LED may be left toggled and an extra character may be typed into the focused field.

To enable full CapsLock restoration, copy and run this entire block in a terminal as your normal user. It adds the required groups, configures device permissions, and loads `uinput` now and at boot. If a command fails, the script stops; check the error before continuing.

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

After the script completes successfully, save your work and **restart your computer** to apply the new group membership, then verify:

```bash
id -nG
ls -l /dev/uinput
test -w /dev/uinput && echo "uinput writable"
```

`id -nG` should include `input` and `uinput`, the device should show `crw-rw---- root uinput`, and the last command should print `uinput writable`. Then open Wox and run `doctor` to check access.

If a command fails:

- **`udevadm trigger` reports `No such device`**: run `sudo modprobe uinput` first. Check that `/sys/class/misc/uinput` exists before retrying the udev trigger command above. Creating a group or writing a rule does not load the module.
- **`modprobe` reports `Module uinput not found`**: check whether your current kernel provides the module. If you recently upgraded your kernel, reboot into the new kernel and retry.
- **The device still belongs to `root:root` or lacks group write permission**: check the saved udev rule, reload the rules, trigger the device event, and wait for `udevadm settle` to finish.
- **Device permissions are correct but it is still not writable**: run `id -nG` without a username to check the current session's groups. `id -nG "$USER"` shows the account's configured groups, which may differ from the groups active in the session. If the account includes `input` and `uinput` but the session does not, restart your computer; reopening a terminal or logging out and back in may retain the old groups. If the session already has the required groups and you only change device permissions, restarting Wox is enough; a computer restart is not needed.

After setup, when CapsLock is pressed alone, it toggles caps lock normally. When CapsLock is used as a combo prefix, the system's caps lock toggle is automatically undone. Regular combination hotkeys (like `ctrl+space`) continue to work via the `org.freedesktop.portal.GlobalShortcuts` portal regardless of this setting.

> **Note:** The system setup above requires `sudo`, but Wox itself runs as a regular user without an extra system daemon. For CapsLock combos, Wox reads evdev events passively and uses uinput to restore CapsLock state and delete the stray combo character. Without uinput, the hotkey still fires, but these recovery operations cannot complete.
