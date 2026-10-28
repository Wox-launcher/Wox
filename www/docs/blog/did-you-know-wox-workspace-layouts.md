---
title: Restore Your Workspace Layout with Wox
description: Save your apps and window positions, then reopen and arrange your workspace in one step.
date: 2026-07-01
category: workspace
cover: /images/blog/workspace-layouts.webp
coverDark: /images/blog/workspace-layouts-dark.webp
coverAlt: Three app windows arranged into a workspace layout
---

# Restore Your Workspace Layout with Wox

Starting a coding session might mean putting your editor on the main display, then arranging a terminal and browser beside it. Move those windows for a meeting or a bit of research, and you have to arrange them again when you return. A Wox workspace lets you configure that recurring layout and restore it from the launcher.

A workspace records where your apps' windows should go. For this example, call it `Work`: an editor on the left and a browser on the right, with a terminal or notes on a second display if you have one.

## Save the arrangement you use most

Open **Settings → Plugins → Window Manager** and add an entry under **Workspace layouts**:

1. Name it `Work`, or choose another name you will recognize.
2. Select a display and choose a window layout.
3. Assign an app to each position, such as your editor and browser.
4. Configure any other displays you want to use, then save.

![Wox workspace layout editor](/images/did-you-know-wox-workspace-layouts.png)

Start with the two windows you use together most often. Once they restore to the positions you want, add your terminal or notes. A small layout is easier to adjust than a desk full of windows.

## Bring it back when you start work

Open Wox and enter:

```text
window group
```

Select `Work` and run it. Wox moves matching app windows into their assigned positions. If an app is not running and its configuration includes a usable app path, Wox tries to launch it, wait for a window, and place it.

> Screenshot needed: The real desktop after configuring and applying the example `Work` workspace, with the editor on the left and browser on the right.

The layout restores window positions. Which project or document an app opens, and how it restores its previous session, still depend on that app. You can open the project you need first, then let Wox arrange the windows.

## Give the layout its own shortcut

For a layout you use every day, add a Query Hotkey in **Settings → General → Query Hotkeys**. Choose the **Silent Run** preset and enter:

```text
window group Work
```

Replace `Work` with your workspace name and record an unused shortcut. Try the query in the launcher first to confirm that it matches only the intended workspace. Silent Run executes the default action when there is a single result, so you can skip selecting the workspace each time.

The connected displays matter. If a saved display is missing or an app prevents window management, Wox applies the parts it can and reports what it could not complete. After switching to a different display setup, revisit the workspace settings to check the arrangement.
