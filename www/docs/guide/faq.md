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

The permissions required depend on which hotkey styles you want to use:

#### Double-modifier hotkeys (e.g. `ctrl+ctrl`) — requires `input` group only

This grants read access to `/dev/input/event*` devices. Wox passively listens to keyboard events without grabbing or remapping the keyboard.

```bash
sudo usermod -aG input $USER
```

Log out and back in, then restart Wox.

#### CapsLock combo hotkeys (e.g. `capslock+a`) — `input` group required, `uinput` group recommended

CapsLock combos need the `input` group (evdev read access) to detect the combo. The `uinput` group is **not required** to register or trigger the hotkey — it is only used to restore CapsLock state and delete the stray combo character after the combo fires.

When CapsLock is used as a combo prefix, the system toggles the caps lock state because Wox cannot consume the raw event on Wayland. Wox undoes this toggle by injecting a CapsLock key event through a temporary uinput virtual keyboard. Without uinput, the hotkey still fires, but the caps lock LED may be left toggled and an extra character may be typed into the focused field.

To enable full CapsLock restoration, add yourself to the `uinput` group:

```bash
sudo groupadd -r uinput 2>/dev/null
sudo usermod -aG input,uinput $USER
```

Then ensure `/dev/uinput` is group-writable. Many stock distros ship `/dev/uinput` as `crw------- root:root`, so group membership alone is not enough — you also need a udev rule:

```bash
echo 'KERNEL=="uinput", MODE="0660", GROUP="uinput"' | sudo tee /etc/udev/rules.d/80-uinput.rules
sudo udevadm control --reload-rules && sudo udevadm trigger /dev/uinput
```

Log out and back in, then restart Wox.

> **Troubleshooting:** If the Wox doctor check reports you are already in the `uinput` group but `/dev/uinput` is still not writable, the device node is missing group permissions. Apply the udev rule above and run `sudo udevadm trigger /dev/uinput` — re-logging in is not needed for the device-node change, but Wox must be restarted.

After setup, when CapsLock is pressed alone, it toggles caps lock normally. When CapsLock is used as a combo prefix, the system's caps lock toggle is automatically undone. Regular combination hotkeys (like `ctrl+space`) continue to work via the `org.freedesktop.portal.GlobalShortcuts` portal regardless of this setting.

> **Note:** Wox does NOT require root or a system daemon. It only reads evdev events passively and uses uinput solely to inject a single CapsLock key event when restoring the caps lock state after a combo. If uinput is unavailable, CapsLock combos still work — only the state restoration is skipped (a warning is logged).

### How do I disable the Wox window animation on Wayland? {#wayland-disable-animation}

On Wayland, Wox renders its main window as a layer-shell surface (namespace `gtk-layer-shell`) on the overlay layer, not as a regular XDG toplevel. As a result, compositor animation rules that target application windows (by app id or window class) do not apply to Wox. To remove the open/close/resize transition animation, configure a layer rule that targets the `gtk-layer-shell` namespace.

#### Hyprland

Add a `layer_rule` to `~/.config/hypr/hyprland.conf` (or the equivalent Lua config):

```ini
layerrule noanim, gtk-layer-shell
```

With the Lua config (`hyprland.lua`):

```lua
hl.layer_rule({
    name    = "wox-no-anim",
    match   = { namespace = "gtk-layer-shell" },
    no_anim = true,
})
```

Hyprland hot-reloads the config, so the change takes effect immediately. If Wox was already visible, toggle it once so the layer surface is recreated with the new rule.

#### Other compositors

Look for the equivalent layer-surface animation option in your compositor's documentation and target the `gtk-layer-shell` namespace. Wox does not control compositor-side animations from within the application.
