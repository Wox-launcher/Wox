---
title: "Window layouts and workspace restore"
description: "Snap windows from Wox, then save and restore multi-app workspace layouts on Windows and macOS."
---

# Window layouts and workspace restore

<ReleaseStamp />

On Windows and macOS, the Window Manager plugin snaps the active window and can restore a saved workspace. Starting in v2.4.9, Wox also offers Hyprland-specific window commands on Linux.

![Window layouts in Wox](/images/guide/plugin_window_manager.png)

## Snap the current window

Query `window`, then a layout name:

```text
window left
window right-half
window maximize
window next-display
```

Common commands include halves, quarters, thirds, maximize, center, minimize, restore, and move to the next or previous display. Many commands accept aliases such as `left` or `左半屏`.

## Restore a workspace

A workspace layout remembers which apps belong on which display slots.

1. Open **Settings -> Plugins -> Window Manager**.
2. Add a workspace under **Workspace layouts**.
3. Choose a layout for each display and assign apps to the slots.
4. Run:

```text
window group
window group Work
```

Wox moves matching windows into the saved slots. If an assigned app is not open and Wox knows its path, it can launch the app first, then place the window.

For a one-keystroke flow, bind a Query Hotkey to `window group Work`. If that query resolves to one workspace, **Silent Run** can apply it without showing the launcher. See [Hotkeys](/guide/usage/hotkeys).

Workspace layouts arrange normal app windows on the current display setup. If an app blocks system window management, or a saved display is disconnected, Wox applies what it can.

## Hyprland on Linux

Starting in v2.4.9, query `window` on Hyprland to access compositor-native actions such as toggling floating or fullscreen, swapping windows, and moving windows between workspaces or displays. Use `window focus-window` to find and focus an existing window. Available actions depend on the current desktop state. Hyprland does not use the Windows/macOS snap layouts or saved workspace groups described above; other Linux desktop environments do not currently provide these Window Manager commands.
