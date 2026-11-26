# Launcher window, focus, and input boundaries

## Runtime contract

- Clipboard paste and selection replacement publish the clipboard while the launcher can still own it. The action then hides the launcher, verifies the captured foreground window, and confirms the launcher is still hidden before injecting the keys. The focus wait is bounded to 400 ms. When an ID was captured, any other window of the same process is a different target. When the target cannot be verified, the clipboard remains available for manual paste.
- Ordinary Escape and focus loss use the native window focus lifecycle. They avoid a second activation after the user has already switched to another app.
- Escape during preedit belongs to the input method. The repeat and release of a consumed press stay with that press. A separate Escape after preedit closes the launcher.
- A fresh show clears the prior query, results, and preview and applies compact bounds before native presentation. MRU and other legitimate results can grow the launcher afterward.
- The launcher restores editor focus in the native show turn. The operating system routes keys typed before native focus to the previous app. Shortcut coverage starts at the point where the native launcher takes focus.

## Automated and manual acceptance

| Boundary | Automated evidence | Physical desktop checks |
| --- | --- | --- |
| Original edit control and exact focus return | Windows `launcher/045` uses two native edit windows in one process and verifies paste text and Escape focus. Linux strategy tests verify live compositor identity matching using fixtures. | On Windows, macOS, X11, KDE and Hyprland, open two windows of one app. Invoke Wox from the second window, then paste or replace selected text. Verify the original editor receives it and its sibling stays unchanged. Repeat after closing the target and confirm manual paste is offered. |
| Composition Escape | Windows `launcher/046` delivers native end-before-key messages. Linux X11 `launcher/046` drives GTK Unicode preedit. | Try Windows Microsoft Pinyin and WeChat IME, macOS Pinyin, and IBus and Fcitx5 on the supported X11 and Wayland desktops. Select or cancel a candidate, hold Escape through repeat, then release and press again. Verify only the separate Escape hides Wox. |
| Fresh reshow | `launcher/047` checks the logical height and accessibility node count of the first submitted frame, followed by the empty query tree. On Windows it also checks that DWM synchronization completed before the retained HWND was shown. | On DirectComposition, macOS IOSurface, and GTK GL/Cairo, expand a result list and preview, hide, and show repeatedly in Fresh and Blank mode. Verify the old content and height never flash. |
| First characters | `launcher/048` sends native input immediately after opening the Recent Items page, while its initial results can still be loading. Windows and macOS use OS Unicode key events to keep the test independent of the active keyboard layout. X11 uses XTest key events. | Trigger the real global hotkey and immediately type several distinct letters. Repeat with automatic input-source switching on and off. On macOS, also run Secure Input case `launcher/044`. |
| Display placement and work area | `position_display_test.go` and Windows native display/DPI unit tests cover overlapping logical bounds, negative origins, moved and disconnected monitors, and scale conversion. | Use 100% alongside 150% or 200%, and place a screen left of or above the primary. Move Wox across screens, unplug its saved screen, and reopen it. Verify placement, cursor hit testing, row height, and the active display's work area. |
| Full screen, desktops, and tiling | Linux desktop backend policy tests cover layer-shell selection and X11 policy. | Verify macOS full-screen Space, Windows virtual desktops with Hide on Blur, and Hyprland, KDE, GNOME, COSMIC, and an X11 tiling WM. Verify Wox opens on the current workspace and dismisses without switching away. |

## Platform capabilities

| Platform | Paste target | Automatic Latin input |
| --- | --- | --- |
| Windows | Captured HWND and PID are verified against the foreground window. | Existing TSF / IMM handling. |
| macOS | AX raises the captured window, then focus is checked. Exact window capture requires Accessibility permission. | ABC remains selected after Wox hides. |
| X11 | EWMH supplies the window ID and XTest injects the paste chord. This requires an EWMH window manager and the XTest extension. | The existing input source stays active. |
| Hyprland | The compositor window address is verified. Paste uses the Wayland virtual keyboard and needs uinput access. | Fcitx5 uses the configured default keyboard. |
| KDE Plasma Wayland | KWin checks the captured window. Wox waits for the virtual keyboard and uses Ctrl+Shift+V for Konsole. | The existing input source stays active. |
| GNOME and other Wayland sessions | Sessions without a foreground identity API use manual paste. | The existing input source stays active. |

The core smoke suite includes `launcher/047`. The Linux CI smoke path runs X11 under Xvfb. Wayland acceptance requires the named compositor and IME. A generic headless compositor would leave the KWin and Hyprland focus paths untested.
