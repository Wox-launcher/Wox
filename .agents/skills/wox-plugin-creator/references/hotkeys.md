# Plugin Hotkeys and Wox Built-ins

Read this before assigning `QueryResponse.Refinements[].Hotkey` or `ResultAction.Hotkey` (Python: `hotkey`). The tables describe application-local bindings and their scopes, not a global blacklist enforced by the SDK.

## Choosing a binding

- Use the platform primary modifier for refinements and prefer it for action defaults: `ctrl` on Windows/Linux, `cmd` on macOS. Emit a real chord such as `ctrl+t` or `cmd+t`, never `ctrl/cmd+t`.
- Always avoid the two window commands below. Prefer avoiding launcher and editor bindings too, and check the preview or form types the plugin actually uses. A binding that works in the query box can fail when a text field, action filter, or WebView has focus.
- Check all refinements, selected-result actions, and toolbar actions that can appear together. Do not give a refinement and an action the same chord; dispatch order can make one unreachable. User-configured Action Panel and query hotkeys can also collide, so no default is guaranteed free on every installation.
- Choose a mnemonic only after checking its context. Additional modifiers may help, but several built-in handlers check only whether the primary modifier is present, so `ctrl+shift+f` is not a universally safe replacement for `ctrl+f`.
- Leave `Hotkey` empty when there is no suitable binding. Refinements remain available in the filter panel and actions remain available in the Action Panel. Use `IsDefault` for the ordinary Enter action. An intentional conventional alternate action can use primary+Enter in a plain result list; do not reuse it for refinements or expect it to bypass an active form's submit handler.
- Verify the plugin with focus in the query editor, the open Action Panel, its refinement panel, and any preview/form it creates. Check both platform strings and preserve typing, clipboard, undo/redo, Tab navigation, and Escape dismissal.

## Window and launcher commands

`Primary` means Ctrl on Windows/Linux and Command on macOS unless a row explicitly says otherwise.

| Shortcut | Behavior | Scope / collision guidance |
| --- | --- | --- |
| Primary+`,` | Open/focus Settings | Wox windows; Note and Chat route to their plugin settings, and an active WebView routes to its owning plugin. Do not assign to plugins. |
| Primary+`W` | Close the current auxiliary window or hide Launcher | An active WebView closes its page first. Runs before widgets and plugin shortcuts; hotkey recording has priority. Do not assign to plugins. |
| Primary+`K` | Toggle Action Panel | Current default; user-configurable. Avoid for plugin defaults. |
| Primary+`J` | Toggle Action Panel | Legacy default retained for existing users; user-configurable. Avoid for plugin defaults. |
| Primary+Shift+`K` | Toggle About panel | Launcher. |
| Primary+`P` | Toggle selected-result preview | Launcher when a preview is available; Ctrl+P navigates upward when Action Panel is open. |
| Primary+`F` | Toggle refinements / filters | Launcher when refinements are available; also opens search in Settings. |
| Primary+`U` | Open unread Attention items | Launcher when unread items are available. |
| Ctrl+`N` / Ctrl+`P` | Next / previous action | Open Action Panel on all platforms, including macOS (these use Control, not Command). |
| Hold Alt, then `1`–`9` | Execute a numbered visible result | Windows/Linux quick select. |
| Hold Command, then `1`–`9` | Execute a numbered visible result | macOS quick select. |
| Ctrl+Up/Down; Option+Up/Down on macOS | Jump between result groups | Launcher. |
| Enter, Shift+Enter, Tab, Shift+Tab, Escape, arrows | Execute, insert a query newline, complete/navigate, dismiss, select | Focus-dependent controls. Do not use unmodified navigation keys for refinements or extra actions. |

Wox's shortcut overview shows the current Action Panel and global/query bindings. Main/selection/query hotkeys are user-configured global registrations, so do not present their defaults as fixed application-local reservations.

## Preview and form commands

| Shortcut | Behavior | Scope |
| --- | --- | --- |
| Primary+`B` | Toggle terminal fullscreen; toggle Chat history sidebar | Terminal / Chat previews and dedicated Chat. |
| Primary+Shift+`F` | Search terminal output | Terminal preview. |
| Primary+`L` | Load a deferred full-file preview | File preview. |
| Primary+`R` | Reload page / refresh models | Active WebView / model manager. |
| Primary+`[` / Primary+`]` | Back / forward | Active WebView. |
| Primary+`O` | Open page in the system browser | Active WebView. |
| Primary+`H` | Hide a cacheable page | Active cacheable WebView. |
| Primary+`W` | Close page | Active WebView; takes priority over closing/hiding its Wox window. |
| Primary+Enter | Submit | Action forms, requirement forms, table row editors, and trigger-conflict forms. |
| Primary+`S` | Save the current edit | Settings field editing and table row editors. |
| Primary+`N` | Add a row | Table editor when no row edit form is open. |

When a native WebView has focus, window shortcuts and the configured Action Panel hotkey use the host bridge. Do not assume every result/refinement hotkey is forwarded from a page. Keep the page's own editing commands working; avoid repurposing them in custom HTML previews.

## Note and shared editing commands

These bindings are contextual, not reservations across unrelated plugin queries. They matter when designing editors or choosing defaults meant to coexist with those windows.

| Shortcut | Behavior / scope |
| --- | --- |
| Primary+`P`; Primary+Shift+`P` | Note search; pin/unpin Note window. |
| Primary+`N`; Primary+`E`; Primary+`K` | New Note; toggle Markdown view; insert/edit a link. |
| Primary+`B` / `I` / `U`; Primary+Shift+`X` | Note bold / italic / underline; strikethrough. |
| Primary+Enter; Primary+`0` / `+` / `-`; Primary+`1`–`9` | Note task toggle; reset/increase/decrease zoom; open a pinned note. |
| Primary+`A` / `C` / `X` / `V` | Select all / copy / cut / paste in focused editors. |
| Primary+`Z`; Primary+Shift+`Z`; Primary+`Y` | Undo; redo in focused editors. |
| Ctrl+Home/End on Windows/Linux; Command+Up/Down on macOS | Document boundaries; Shift extends selection. |
| Command+Left/Right on macOS; word movement/deletion chords; Home/End | Line boundaries and normal text navigation/editing; Shift extends selection where applicable. |

Other system plugins also declare query-scoped shortcuts, for example primary+T/S for file filters/sort, primary+D for Clipboard/Color actions, and primary+M for file/application actions. Those do not reserve the chord for every third-party query. Check co-visible actions and refinements instead of banning every shortcut used by another plugin. The `T` example in [refinements.md](refinements.md) illustrates platform formatting, not a promise that the key is always available.

## Maintaining this reference

When changing Wox bindings in the repository, reconcile this table with `wox.core/ui/launcher/preview_structured.go` (the live overview), `window_shortcuts.go`, `app.go`, `actions.go`, `about_menu.go`, `quick_select.go`, the relevant `*_preview.go`, `notes_window.go`, `settings_search.go`, `settings.go`, `form.go`, `form_table.go`, and `model_manager.go`. Action Panel defaults live in `wox.core/setting/wox_setting.go`; text editing lives in `wox.core/ui/launcher/component/wox_text_field.go` and `wox.core/ui/runtime/text_editing.go`. Focus and native WebView forwarding can narrow which bindings are reachable; do not infer universal priority from a display label alone.
