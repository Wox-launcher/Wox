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

### Plugin installation failed. What should I check?

1. Confirm network access to the plugin store and release host.
2. Check whether the plugin requires Node.js or Python.
3. Open the Wox log directory and inspect the newest core and plugin-host logs.
4. Try `wpm` again after restarting Wox if a runtime host was just installed.

### How do I update plugins?

Run `wpm`, select the plugin, and use the update action when one is available. You can also manage installed plugins from Plugin Manager settings.

## File Search

### Does Wox require Everything?

No. Wox has its own File plugin and indexes the roots you configure in plugin settings. Install [Everything](https://www.voidtools.com/) only if you also want to use Everything outside Wox.

### Why does file search ask for permissions on macOS?

macOS may block access to Desktop, Documents, Downloads, removable drives, or other protected locations. Grant Wox file access in **System Settings -> Privacy & Security** if search status or logs report permission errors.

## Customization

### How do I change the theme?

Run `theme` in Wox or open **Settings -> Theme**.

### How do I change the hotkey?

Open **Settings -> General** and edit the hotkey field.

## Wayland

### How do I use double-modifier hotkeys or CapsLock combos on Wayland? {#wayland-double-modifier-hotkeys}

On Wayland, Wox cannot globally intercept raw key events through the display server like it does on X11. To enable double-modifier hotkeys (such as `ctrl+ctrl`, `shift+shift`) and CapsLock-combo hotkeys (such as `capslock+a`), Wox reads keyboard events directly from the Linux evdev interface.

This requires your user account to have read access to `/dev/input/event*` devices, which is granted by membership in the `input` group.

**Setup:**

1. Add your user to the `input` group:

   ```bash
   sudo usermod -aG input $USER
   ```

2. Log out and log back in (or reboot) for the group change to take effect.

3. Restart Wox.

After that, double-modifier and CapsLock-combo hotkeys will work on Wayland. Regular combination hotkeys (like `ctrl+space`) continue to work via the `org.freedesktop.portal.GlobalShortcuts` portal regardless of this setting.

> **Note:** This does NOT require root or a system daemon. Wox only reads evdev events passively — it does not grab, remap, or inject any keyboard input.
