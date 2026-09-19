---
title: "Screenshot capture and history in Wox"
description: "Capture, annotate, pin, and search screenshots from the Wox launcher, with optional OCR."
---

# Screenshot capture and history in Wox

<ReleaseStamp />

The Screenshot plugin captures the screen, lets you annotate the image, and keeps a searchable history in the launcher.

![Screenshot capture in Wox](/images/plugin_screenshot.png)

## What it is

Query `screenshot` or `截图` to browse history. Use `screenshot new` to start a capture.

From the editor you can crop, add numbered markers, inspect colors, type a pixel size, export PNG or JPEG, and pin a capture as a desktop overlay. History is kept for 15 days by default.

OCR is optional. Enable it in **Settings -> Plugins -> Screenshot** to search captures by text in the image. You can use the system OCR engine or a downloadable offline PaddleOCR model.

## Common actions

| Action | Use |
| --- | --- |
| Copy | Put the image on the clipboard |
| Pin | Keep a floating overlay on the desktop |
| Save to Notes | Attach the capture to a new or existing note |
| Open | Open the file in the default image app |

## How to use it

1. Run `screenshot new`, or bind a hotkey to that query.
2. Select the region and annotate if needed.
3. Later, type `screenshot` and filter by date or OCR text.
