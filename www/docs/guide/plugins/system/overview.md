# System Plugins Overview

System plugins are the built-in workflows that ship with Wox. They are normal plugins from the user's point of view: each one can return results, expose actions, keep settings, and participate in query routing.

## Everyday Plugins

| Plugin | Trigger | Use it for |
| --- | --- | --- |
| [Application](./application.md) | Global | Launch apps, open app folders, activate running apps |
| [File](./file.md) | `f` | Search indexed files and folders |
| [WebSearch](./websearch.md) | Global / engine keyword | Search the web with configured engines |
| [Clipboard](./clipboard.md) | `cb` | Reuse clipboard text and images |
| [Calculator](./calculator.md) | Global / `calculator` | Evaluate expressions and copy results |
| [Converter](./converter.md) | Global / `calculator` | Convert units, currencies, crypto, bases, and time values |

## Context Plugins

| Plugin | Trigger | Use it for |
| --- | --- | --- |
| [Browser Bookmark](./browser-bookmark.md) | Global | Open bookmarks from supported browser profiles |
| [Explorer](./explorer.md) | Global when a file manager or open/save dialog is active | Jump between folders |
| Browser | Contextual | Search and switch browser tabs when browser integration is available |
| Selection | Selection query | Run actions on selected text or files |
| MediaPlayer | `media` | Control active media playback |

## AI and Custom Workflows

| Plugin | Trigger | Use it for |
| --- | --- | --- |
| [AI Chat](./chat.md) | `chat` | Talk to configured models and agents |
| AI Command | `ai` | Run saved model prompts |
| [Emoji](./emoji.md) | `emoji` | Search and copy emoji |
| Plugin Manager | `wpm`, `store`, `pm` | Install, update, create, and inspect plugins |
| Theme | `theme` | Apply, install, remove, or generate themes |

## Maintenance Plugins

| Plugin | Trigger | Use it for |
| --- | --- | --- |
| Doctor | `doctor` | Diagnose common setup issues |
| Update | `update`, `upgrade` | Check or apply Wox updates |
| Backup | `backup`, `restore` | Export or restore settings |
| Shell | `>` / global command detection | Run shell commands from Wox |
| Sys | Global | Run system actions |

## How to Tune a Plugin

1. Open **Settings**.
2. Go to **Plugins**.
3. Select the plugin.
4. Review trigger keywords, enablement, and plugin-specific settings.

Most confusion comes from global plugins competing for the same text. If a query produces too many unrelated results, use a plugin keyword such as `f`, `cb`, `emoji`, or `wpm`.
