## What are Query Shortcuts?

Query shortcuts in Wox Launcher are a feature that allows you to simplify your [Query](query.md). This can be done by assigning a shortcut to a longer query.
When this shortcut is entered, Wox Launcher will automatically expand it to the full query and pass it to all plugins. However, this expansion is implicit and not visible to the
user. The user will still see the shortcut in the UI, but the plugins will receive the expanded query.

## How to Use Query Shortcuts

To use query shortcuts, you need to set them up in Wox Launcher. Here's how:

1. Open Wox Launcher and query `wox setting` and execute it.
2. Click on the General menu.
3. Navigate to the "Query Shortcuts".
4. Here, you can add, modify, or remove query shortcuts.

For example, if you frequently use the query `llm shell`, you can create a shortcut for it, such as `sh`. After setting this up, whenever you enter `sh` in Wox Launcher, it will
automatically expand it to `llm shell` and pass it to all plugins. However, in the UI, you will still see `sh`.