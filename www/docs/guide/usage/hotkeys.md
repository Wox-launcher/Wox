# Hotkeys

Wox has one main launcher hotkey, plus optional hotkeys that run a prepared query. Open **Settings -> General** to change them.

## Main Hotkey

The default toggle is:

| Platform | Default |
| --- | --- |
| Windows | `Alt + Space` |
| macOS | `Command + Space` |
| Linux | `Ctrl + Space` |

There is also a **Selection hotkey**. It opens Wox against the current text or file selection so plugins such as Selection, Web Search, and AI Command can act on it.

On macOS, grant Accessibility if hotkey recording asks for it. On Linux Wayland, ordinary combinations such as `Ctrl + Space` use the desktop portal; double-modifier and CapsLock combos need extra input permissions. See the [Wayland FAQ](../faq.md#wayland-double-modifier-hotkeys).

## Fullscreen Applications

Enable **Settings -> General -> Ignore hotkeys in fullscreen** to suppress the main, selection and query hotkeys while the foreground window is fullscreen. It is off by default; ordinary maximized windows are unaffected. Dictation hotkeys are independent of this launcher setting.

Detection supports Windows, macOS (Accessibility permission required), Linux X11 with `xprop`, and Hyprland with `hyprctl`. The switch is disabled on unsupported Wayland desktops or when the required Linux command is missing. Detection failures leave hotkeys enabled.

This suppresses Wox actions without unregistering the shortcuts, so it does not guarantee that the foreground application receives those keys.

## Query Hotkeys

A Query Hotkey binds a shortcut to a query. Instead of opening Wox and typing `webview x` or `window group Work`, the shortcut does that in one step.

Open **Settings -> General -> Query Hotkeys**, add a row, and start from a preset:

| Preset | Use it for |
| --- | --- |
| Normal Query | Show the launcher and run the query |
| Preview Query | Hide the query box and toolbar; useful for WebView panels and file previews |
| Silent Run | Run the query without showing the launcher when there is a single clear action |
| Custom | Keep the preset defaults, then override position, width, result count, or chrome |

You can still override window position, width, result count, or whether the query box and toolbar stay visible.

Settings also include a **Query Test** button so you can try a hotkey, shortcut, or tray query without leaving the page.

### Examples

| Query | What the hotkey does |
| --- | --- |
| `app launchpad` | Open the app grid |
| `webview x` | Open a compact X panel |
| `window group Work` | Restore a saved workspace |
| `ai translate {wox:selected_text}` | Translate the current selection |

Silent translation is the same pattern: create an AI Command, then bind a Query Hotkey with the **Silent Run** preset. See [AI Commands](../ai/commands.md).

## Tray Queries

Tray Queries are the mouse-friendly version of Query Hotkeys. In **Settings -> General -> Tray Queries**, add an icon and a query. Clicking the tray or menu-bar item opens Wox near the tray and runs that query.

This is useful for a dashboard, a WebView site, clipboard history (`cb`), or notes (`note`).

## Other Built-in Hotkey Surfaces

| Feature | Where |
| --- | --- |
| Open the Action Panel | Default `Ctrl+K` (`Cmd+K` on macOS); change it in **Settings -> General** |
| Open a Web Search in WebView | Result action `Ctrl+Enter` (`Cmd+Enter` on macOS); the query box stays visible; Escape returns focus to the query |
| Close the current WebView page | Action Panel `Ctrl+W` (`Cmd+W` on macOS); destroys the page so its memory can be released |
| Hide a background WebView page | Action Panel `Ctrl+H` (`Cmd+H` on macOS); only on query-hotkey full preview windows with Run in background |
| Dictation press / double-press / hold | **Settings -> Plugins -> Dictation** |
| Screenshot capture | **Settings -> Plugins -> Screenshot**, or query `screenshot` |
| Hotkey overview | Query `hotkeys` to list registered shortcuts |
