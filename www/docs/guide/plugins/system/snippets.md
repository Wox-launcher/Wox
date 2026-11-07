# Snippets Plugin

Snippets saves reusable text for quick copying, pasting, or automatic expansion as you type.

## Create a snippet

1. Open Wox Settings → Installed Plugins → Snippets.
2. Add a snippet with a name and the text you want to reuse. A keyword is optional.
3. To expand keywords while typing, turn on **Enable automatic expansion** on this device. It is off by default.

For example, set the keyword to `;email` and the text to `hello@example.com`. With automatic expansion enabled, typing `;email` in a supported text field replaces it with `hello@example.com`.

A keyword may contain printable ASCII characters, including spaces. Wox replaces the entire keyword as soon as it is typed, without waiting for Enter or a space. Include a trailing space or punctuation in the keyword if you want that character to trigger expansion; include it in the text too if it should remain after replacement. Keywords must be unique. If synced snippets have conflicting keywords, their automatic expansion is paused until the conflict is resolved.

## Find and use snippets

Type `snip` in Wox to list snippets, or `snip email` to search by name, keyword, or text. Press Enter to copy the expanded text. The Action Panel also offers a paste action when a target application is available. These actions work without enabling automatic expansion.

Immediately pressing Backspace after an automatic expansion attempts to restore the keyword. Undo support depends on the target editor and the text inserted.

## Insert variables

In the text field, type `{` or use the lightning button to insert a variable. Variables are resolved when the snippet is copied, pasted, or expanded.

| Variable | Result |
| --- | --- |
| `{wox:date}` | Today's date |
| `{wox:date?format=yyyy-MM-dd}` | Date in the specified format |
| `{wox:clipboard_text}` | Current clipboard text |
| `{wox:uuid}` | A new unique identifier |

Date formats support `yyyy`, `MM`, `dd`, `HH`, `mm`, and `ss`. Unsupported placeholders or date formats remain unchanged. If clipboard text is unavailable, expansion is cancelled.

## Sync and permissions

Snippets sync as one setting when Cloud Sync is enabled; concurrent edits on different devices follow the normal whole-setting conflict behavior. The automatic expansion switch is local, so enable it separately on each device.

Automatic expansion requires access to keyboard input and the focused editable control. On macOS, grant Wox accessibility access. On Linux, the desktop must expose accessibility information; Wayland also needs access to `/dev/input` and `/dev/uinput`. Some applications, input methods, and desktop sessions cannot provide enough context for expansion. The Snippets settings show a status message when access is unavailable.

Some expansion paths temporarily use the clipboard. Clipboard restoration depends on the platform and can be affected by simultaneous clipboard changes; on Wayland, the expanded text may remain on the clipboard.
