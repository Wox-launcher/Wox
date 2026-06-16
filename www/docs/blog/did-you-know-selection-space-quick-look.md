---
title: "Did You Know: Wox Can Preview Files with the Space Key on Windows"
description: Enable Space Quick Look in the Selection plugin to preview the selected file from Windows File Explorer or open/save dialogs.
date: 2026-05-26
---

# Did You Know: Wox Can Preview Files with the Space Key on Windows

Windows users can use Wox's Selection plugin for a Quick Look-style flow: select one file, press Space, and Wox opens a focused file preview without launching the full associated app.

<video src="/videos/did-you-know-selection-space-quick-look.mp4" controls muted loop playsinline style="width: 100%; border-radius: 8px;"></video>

Turn it on from the Selection plugin settings:

| Selection setting | Value |
| --- | --- |
| Enable Space Quick Look | Enabled |

After that, use it from supported Windows file-selection surfaces:

1. Select one file in Windows File Explorer or an open/save dialog.
2. Press Space.
3. Wox opens a preview-only panel for that file.

This is useful when you want to check a document, image, archive, or config file quickly, but do not want to switch into the full associated application. It also keeps the result list out of the way, so the preview itself gets the available space.

The same Selection plugin still supports the normal selection actions. If you trigger Wox on selected files, you can copy paths, open the containing folder, or use the regular Preview result. Space Quick Look is just the fastest entry point when the file is already selected.

Platform note: Space Quick Look is currently available on Windows only. macOS keeps the setting disabled because Finder already has native Quick Look, and Wox does not install a macOS Space-key monitor for Selection preview.
