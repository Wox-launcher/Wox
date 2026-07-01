---
title: "Did You Know: Wox Can Restore Your Workspace Layout"
description: Save apps and screen positions in the Window Manager plugin, then reopen and arrange that workspace from Wox.
date: 2026-07-01
---

# Did You Know: Wox Can Restore Your Workspace Layout

Some work always starts with the same set of windows: editor on one side, terminal below it, browser or notes on another display. Wox can save that arrangement as a workspace layout and bring it back from the launcher.

![Wox workspace layout editor](/images/did-you-know-wox-workspace-layouts.png)

Create one from the Window Manager plugin settings:

1. Open Wox Settings.
2. Go to Plugins, then Window Manager.
3. Add a workspace under Workspace layouts.
4. Select each display, choose a layout, and assign apps to the slots.

After saving it, open Wox and run:

```text
window group
```

Pick the workspace you saved. Wox will move matching app windows into the configured slots. If an assigned app is not open and Wox knows its app path, it can open the app first, then place the window when it appears.

This is useful for repeat contexts: a coding workspace, a writing workspace, a meeting setup, or any layout you rebuild by hand after restarting apps or reconnecting displays.

For a one-keystroke flow, bind a Query Hotkey to the exact workspace query, such as:

```text
window group Work
```

If that query resolves to one workspace, silent execution can apply it without showing the launcher.

Workspace layouts arrange normal app windows on the current display setup. If an app blocks system window management, or a saved display is not connected, Wox applies what it can and reports the parts that could not be moved.
