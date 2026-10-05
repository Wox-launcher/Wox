---
title: "Screenshot capture and history in Wox"
description: "Capture, annotate, pin, and search screenshots from the Wox launcher, with optional OCR."
---

# Screenshot capture and history in Wox

<ReleaseStamp />

The Screenshot plugin captures the screen, lets you annotate the image, and keeps a searchable history in the launcher.

![Screenshot capture in Wox](/images/guide/plugin_screenshot.png)

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

On Windows and macOS, hover over a window to highlight it and click to select it. Drag to select a custom region. Overlapping windows follow their visible stacking order. On macOS, a window spanning displays is selected within the display under the pointer.

When native window capture is available, an unchanged window selection uses the window's own pixels at export, preserving transparent rounded corners without the desktop behind them. The editor keeps showing the original desktop pixels. Moving, resizing, or replacing the selection cancels this transparency effect, even if you later return to the original bounds. Transparent captures are saved as PNG by default and keep their alpha when copied or pinned. Choosing JPEG explicitly produces an opaque image.

Window screenshots also have a **Show background** toolbar toggle (D). The editor preloads the wallpaper in the background and prepares the composition once window pixels are ready; subsequent toggles reuse it. It centers the original-size window on the current system wallpaper inside a rounded canvas. Horizontal and vertical margins use the same percentage of the window width and height, keeping the original aspect ratio. The selection outline, handles, and size label follow the full background canvas and preview the composition before copying, saving, or pinning. Turning it off restores the transparent-window export. Changing the selection cancels both effects. Exiting the editor cancels pending background work and releases its images; the history job releases its own image data after saving. Editable history retains the wallpaper and toggle state.
