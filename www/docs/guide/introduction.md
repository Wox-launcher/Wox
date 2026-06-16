# Introduction

Wox is a keyboard-first launcher for Windows, macOS, and Linux. Use it to open apps, find files, run calculations, search the web, reuse clipboard history, talk to AI models, and extend the launcher with plugins.

It is built for people who want one fast command window instead of a collection of small utilities. The core app stays small; plugins add the workflows you actually use.

## What Wox is good at

- **Opening things quickly**: apps, folders, files, browser bookmarks, URLs, and system actions.
- **Finishing the next step**: every result can expose actions such as copy, reveal, delete, paste, install, or open in another tool.
- **Keeping local workflows close**: clipboard history, calculator, converter, file search, browser tabs, and media controls are built in.
- **Letting plugins do the custom work**: install community plugins, write script plugins, or build full plugins with the Node.js and Python SDKs.
- **Staying portable**: Wox stores user data under `~/.wox` on macOS/Linux and `%USERPROFILE%\.wox` on Windows, so settings and logs are easy to inspect.

## How queries work

Wox routes what you type to plugins.

Some plugins listen globally. For example, app search, calculator, converter, and web search can show results without a keyword. Other plugins use an explicit trigger keyword:

| Example | What it does |
| --- | --- |
| `f invoice` | Search files with the built-in File plugin |
| `cb token` | Search clipboard history |
| `emoji check` | Search emoji |
| `wpm install` | Search the plugin store |
| `chat explain this` | Start an AI chat |

When a result is selected, press `Enter` for the primary action or open the Action Panel for more choices.

## Recommended first pass

1. [Install Wox](./installation.md).
2. Open Wox with the default hotkey: `Alt + Space` on Windows, `Command + Space` on macOS, or `Ctrl + Space` on Linux.
3. Try app search first by typing an application name.
4. Open [Querying](./usage/querying.md) to learn keyword and fallback behavior.
5. Open [System Plugins](./plugins/system/overview.md) when you want to tune the built-in workflows.
