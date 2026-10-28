---
title: Preview Files with the Space Key on Windows
description: Select a file in Explorer or a file dialog and press Space to preview it without opening another app.
date: 2026-05-26
category: preview
cover: /images/blog/file-preview.webp
coverDark: /images/blog/file-preview-dark.webp
coverAlt: A file preview panel and a spacebar
---

# Preview Files with the Space Key on Windows

When attaching a file, a few similar filenames can leave you guessing. Is this the image you just exported? Does that document contain the section you need? Opening each file in its usual app means leaving the file list just to check.

On Windows, Wox can preview the file you already have selected. Press Space to see it in a separate preview panel before you open or upload it.

<video src="/videos/did-you-know-selection-space-quick-look.mp4" controls muted loop playsinline style="width: 100%; border-radius: 8px;"></video>

## Enable it once, then use it from the file list

Open **Settings → Plugins → Selection** and turn on **Enable Space Quick Look**. The option is off by default.

Try it with an image:

1. With Wox running, select one image in Windows File Explorer.
2. Press Space on its own. You do not need to open the launcher first.
3. Check the image in the preview panel that appears.

The same flow works in supported open/save dialogs. When choosing an email attachment, for example, select an image in the file list and press Space to check it before continuing.

## Give the file room to show its contents

Space Quick Look hides the query box and result list, leaving most of the panel for the file itself. You can inspect images, supported text files, PDFs, and Office documents. What you see depends on the format and available preview support; larger documents may show file details before their content loads.

Select just one file to trigger the preview. If Space does nothing, check that the option is enabled, that you have not selected several files, and that focus is still in the file list.

For actions such as copying the path or opening the containing folder, use Wox's **Selection hotkey** instead. That opens the regular actions for the selected file, while Space takes you straight to its preview.

Space Quick Look is currently Windows-only. On macOS, Finder already provides Quick Look; Wox does not enable this Space-preview setting on macOS or Linux.
