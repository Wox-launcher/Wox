---
title: "Cross-platform launcher for Linux Wayland"
description: "Wox is a native Linux launcher with Wayland layer-shell support, including portal and hotkey limits."
---

# Cross-platform launcher for Linux Wayland

<ReleaseStamp />

Wox is a native launcher on Linux, not an Electron shell. On Wayland it shows as a layer-shell overlay and still receives regular global hotkeys through the desktop portal.

## What it is

Wox ships stable Linux builds alongside Windows and macOS. The main window is a layer-shell surface (namespace `gtk-layer-shell`) on the overlay layer, so it can appear above other windows without becoming a normal XDG toplevel.

Ordinary combination hotkeys such as `Ctrl+Space` go through `org.freedesktop.portal.GlobalShortcuts`. Double-modifier and CapsLock-combo hotkeys need extra input permissions because Wayland does not let Wox intercept raw keys the way X11 does.

## Why the limits exist

Wayland compositors own animations, global shortcuts, and clipboard portals. Wox does not bypass those with a root daemon. Layer-shell also means compositor rules that target application windows will not hide Wox's open/close animation — you configure a layer rule instead.

These constraints are still true on current Wayland compositors.

## How to set it up

- [Double-modifier and CapsLock hotkeys on Wayland](/guide/faq.html#wayland-double-modifier-hotkeys)
- [Disable the Wox window animation on Wayland](/guide/faq.html#wayland-disable-animation)
- [Installation](/guide/installation)
