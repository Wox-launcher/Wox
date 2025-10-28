## What are Query Hotkeys?

Query hotkeys in Wox Launcher are a specific type of hotkeys that allow you to quickly perform a [Query](query.md). This can be done by assigning a query to
a hotkey. When this hotkey is pressed, Wox Launcher will automatically perform this query.

## How to Use Query Hotkeys

To use query hotkeys, you need to set them up in Wox Launcher. Here's how:

1. Open Wox Launcher and query `wox setting` and execute it.
2. Click on the Hotkey menu.
3. Navigate to the "Query Hotkeys".
4. Here, you can add, modify, or remove query hotkeys.

For example, if you frequently use the query `llm shell`, you can create a hotkey for it, such as `Ctrl+Shift+S`. After setting this up, whenever you press `Ctrl+Shift+S` in Wox
Launcher, it will automatically perform the `llm shell` query.

## Query Variables

Wox Launcher allows you to use variables in your queries. These variables can be used to represent specific pieces of information. Available query variables include:

- `{wox:selected_text}`: This variable represents the text currently selected by the user.
- `{wox:active_browser_url}`: This variable represents the URL of the currently active browser window.
- `{wox:file_explorer_path}`: This variable represents the path of the currently open folder in the file explorer (if available).


To use a variable in a query, simply include it in the query string. Wox Launcher will automatically replace the variable with the corresponding information when the query is
performed.

## Silent Mode

Wox Launcher also supports a silent mode for queries. When silent mode is enabled, Wox Launcher will not display the query interface when a query is performed. Instead, if the
query returns exactly one result, it will directly execute that result. If the query returns more than one result or no results, the query will not be executed and no UI will be
displayed, but a notification will be shown to inform the user that the query failed.

To enable silent mode, go to the settings menu and toggle the "Silent" option.