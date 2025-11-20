# Querying

## Basic Usage

Wox is triggered by a hotkey (defaults vary by platform). Once opened, you can type to search for applications, files, bookmarks, and more.

## Query Structure

A query in Wox typically consists of three parts:

1. **Trigger Keyword**: Used to activate a specific plugin (e.g., `wpm` for plugin management).
2. **Command**: Some plugins support specific commands (e.g., `install` in `wpm install`).
3. **Search Term**: The actual content you want to search for or process.

Example: `wpm install wox`

- `wpm`: Trigger keyword
- `install`: Command
- `wox`: Search term

## Global Trigger

Some plugins support a global trigger `*`, meaning they can be triggered by any query that doesn't match other specific keywords. This is commonly used for file search or application launching.

## Built-in Everything file search (Windows)

Wox ships a built-in file plugin (trigger keyword: `f`) that uses the Everything engine on Windows. To use it:

- Install and run [Everything](https://www.voidtools.com/) so its service is active and indexed.
- Leave Everything running in the background; Wox calls its APIs for instant file results.

## Shortcuts

Wox supports various shortcuts to improve your efficiency:

| Shortcut                   | Description                                                    |
| -------------------------- | -------------------------------------------------------------- |
| Windows: `Alt + Space`     | Toggle Wox visibility (default)                               |
| macOS: `Cmd + Space`       | Toggle Wox visibility (default)                               |
| Linux: `Ctrl + Space`      | Toggle Wox visibility (default)                               |
| `Esc`                      | Hide Wox                                                       |
| `Up` / `Down`              | Navigate through results                                       |
| `Enter`                    | Execute the selected result's default action                   |
| `Alt/Cmd + J`              | Open the result's context menu (Action Panel)                  |
| `Tab`                      | Autocomplete the query                                         |

## Hotkeys

You can customize the global hotkey to toggle Wox in the settings.

1. Open Wox Settings.
2. Go to the **General** tab.
3. Click on the **Hotkey** field and press your desired key combination.
