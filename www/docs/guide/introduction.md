# Introduction

Wox is a keyboard-first launcher for Windows, macOS, and Linux. Use it to open apps, find files, capture notes, take screenshots, run timers, search the web, reuse clipboard history, talk to AI models, and extend the launcher with plugins.

If you are choosing between Wox, Flow Launcher, Raycast, or PowerToys Run, see [Wox vs Flow, Raycast, and PowerToys](/compare/).

The core app stays small. Built-in plugins cover daily work; the store and SDKs add the rest.

## What Wox is good at

- **Opening things quickly**: apps, folders, files, bookmarks, URLs, and system actions.
- **Finishing the next step**: every result can expose actions such as copy, reveal, paste, save to Notes, or open in another tool.
- **Keeping local workflows close**: clipboard history, calculator, converter, file search, notes, screenshots, timers, dictation, and window layouts are built in.
- **Letting plugins do the custom work**: install community plugins, write script plugins, or build full plugins with the Node.js and Python SDKs.
- **Staying portable**: Wox stores user data under `~/.wox` on macOS/Linux and `%USERPROFILE%\.wox` on Windows.

## How queries work

Wox routes what you type to plugins.

Some plugins listen globally. App search, calculator, converter, and web search can show results without a keyword. Other plugins use an explicit trigger:

| Example | What it does |
| --- | --- |
| `f invoice` | Search files |
| `cb token` | Search clipboard history |
| `note meeting` | Search notes |
| `timer 5m` | Start a countdown |
| `wpm install` | Search the plugin store |
| `chat explain this` | Start an AI chat |

When a result is selected, press `Enter` for the primary action or open the [Action Panel](./usage/action-panel.md) for more choices.

## Recommended first pass

1. [Install Wox](./installation.md).
2. Open Wox with the default hotkey: `Alt + Space` on Windows, `Command + Space` on macOS, or `Ctrl + Space` on Linux.
3. Type an application name to confirm app search.
4. Read [Querying](./usage/querying.md) for keywords, query hints, and fallback results.
5. Open **Settings -> Plugins**, or pick a built-in plugin from the sidebar, when you want to tune a workflow.
