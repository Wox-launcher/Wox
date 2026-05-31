# Querying

Open Wox, type what you want, then act on the selected result. The launcher does not require you to remember every plugin keyword, but keywords are useful when you want a specific plugin to handle the query.

## Query Types

| Type | Example | Notes |
| --- | --- | --- |
| Global search | `chrome` | Lets global plugins compete for results, such as apps, calculator, converter, and web search. |
| Keyword query | `f invoice` | Sends the query to a plugin with a trigger keyword. |
| Command query | `wpm install` | Runs a command inside a plugin. |
| Selection query | Select text, then trigger Wox selection actions | Used by plugins that can work with selected text or files. |

## Keywords and Commands

A keyword is the first word in the query. If it matches a plugin trigger, Wox sends the rest of the query to that plugin.

```text
wpm install browser
```

| Part | Meaning |
| --- | --- |
| `wpm` | Plugin Manager trigger |
| `install` | Plugin Manager command |
| `browser` | Search term passed to the command |

Common built-in keywords:

| Keyword | Plugin |
| --- | --- |
| `f` | File search |
| `cb` | Clipboard history |
| `emoji` | Emoji search |
| `chat` | AI chat |
| `wpm`, `store`, `pm` | Plugin Manager |
| `calculator` | Calculator / converter explicit mode |

## Fallback Results

Some plugins listen to normal text without a keyword. That is why typing `Chrome`, `100 + 20`, `1km to m`, or a web search can produce useful results immediately.

Use a keyword when fallback results are noisy or when you know exactly which plugin should answer.

## Shortcuts

| Shortcut | Description |
| --- | --- |
| Windows: `Alt + Space` | Toggle Wox visibility |
| macOS: `Command + Space` | Toggle Wox visibility |
| Linux: `Ctrl + Space` | Toggle Wox visibility |
| `Esc` | Hide Wox or go back from a nested view |
| `Up` / `Down` | Move through results |
| `Enter` | Run the selected result's primary action |
| `Alt + J` / `Command + J` | Open the Action Panel |
| `Tab` | Complete the suggested query when available |

## Hotkey Settings

Open **Settings -> General** to change the main Wox hotkey. You can also create Query Hotkeys with presets such as **Normal Query**, **Preview Query**, **Silent Run**, or **Custom**. Presets give you sensible defaults first, and you can still override position, width, result count, or chrome visibility when needed.
