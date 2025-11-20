# Query Model

Wox normalizes every user interaction into a `Query` object that is sent to plugins. Understanding how it is split helps you write predictable plugins and validation.

## Query types

- `input` – standard text input such as `wpm install wox`.
- `selection` – selection/drag-drop payloads (text/files/images). Only delivered when the plugin declares the `querySelection` feature.

## Query shape

| Field | Notes |
| --- | --- |
| `RawQuery` | Original text the user typed, including the trigger keyword if present. |
| `TriggerKeyword` | One of the keywords declared in `plugin.json`. `"*"` means global trigger. Empty means a global query for plugins that registered `*`. |
| `Command` | Optional command segment following the trigger keyword. Comes from `Commands` in `plugin.json`. |
| `Search` | Remainder of the query after trigger keyword + command. |
| `Selection` | When `Type=selection`, includes `Type`, `Text`, `FilePaths`. Available only with `querySelection`. |
| `Env` | Optional environment data such as active window info or browser URL. Available only with the `queryEnv` feature. |

Example split for `wpm install wox`:

- `TriggerKeyword`: `wpm`
- `Command`: `install`
- `Search`: `wox`
- `RawQuery`: `wpm install wox`

## Environment context (`queryEnv` feature)

When `Features` includes `queryEnv`, Wox will attach:

- `ActiveWindowTitle`
- `ActiveWindowPid`
- `ActiveWindowIcon` (as `WoxImage`)
- `ActiveBrowserUrl` (when the Wox Chrome extension is installed and the browser is active)

Use feature params to only request the fields you need (see [Specification](./specification.md)).

## Special query variables

Wox expands the following placeholders in user queries before sending them to plugins:

- `{wox:selected_text}`
- `{wox:active_browser_url}`
- `{wox:file_explorer_path}`

These are useful for plugins that want to seed searches from the current selection or file manager context.
