# Deep Links

Deep links let other apps and scripts call Wox through a `wox://` URL to prepare a query, open an installer, or invoke a plugin. Use them in terminal commands, automation tools, and app buttons.

| Link | Purpose |
| --- | --- |
| `wox://query` | Fill in a query in the primary launcher |
| `wox://install` | Open the installer for a local plugin or theme package |
| `wox://toggle` | Show or hide the primary launcher |
| `wox://confetti` | Play a confetti animation |
| `wox://plugin/<plugin-id>` | Invoke a plugin's deep-link callback |

## query: Prepare a Query

### Purpose

Fill in the given text in the primary launcher and show its results. This does not automatically execute a result. The link reuses the primary launcher without creating a secondary window.

### Format and Parameters

```text
wox://query?q=<url-encoded-query>
```

| Parameter | Required | Description |
| --- | --- | --- |
| `q` | Yes | Non-empty query text, exactly as you would type it in Wox. URL-encode the parameter value. |

### Examples

Search for files whose names contain `invoice`, using the query `f invoice`:

```text
wox://query?q=f%20invoice
```

Calculate `100 + 20`, encoding the plus sign as `%2B`:

```text
wox://query?q=100%20%2B%2020
```

Open Plugin Manager's install query, preserving the trailing space in `wpm install `:

```text
wox://query?q=wpm%20install%20
```

## install: Install a Plugin or Theme

### Purpose

Open the installer for a local `.wox` plugin package or `.wox-theme` theme package so the user can confirm installation, upgrade, or reinstallation. Double-clicking either file type uses this route after Wox registers the file association.

The installer reuses the primary launcher and stays visible when focus returns to the file manager.

### Format and Parameters

```text
wox://install?path=<url-encoded-file-path>
```

| Parameter | Required | Description |
| --- | --- | --- |
| `path` | Yes | An absolute path to a local plugin or theme package, or a `file://` URL. URL-encode the entire parameter value. |

### Examples

Install `C:\Users\demo\Downloads\example.wox` on Windows:

```text
wox://install?path=C%3A%5CUsers%5Cdemo%5CDownloads%5Cexample.wox
```

Install `/Users/demo/Downloads/example.wox-theme` on macOS:

```text
wox://install?path=%2FUsers%2Fdemo%2FDownloads%2Fexample.wox-theme
```

Replace these example paths with paths to files that exist on your computer.

## toggle: Show or Hide the Launcher

### Purpose

Toggle the primary launcher's visibility: show it when hidden, or hide it when visible. This is useful for external keyboard shortcuts and automation buttons.

### Format and Parameters

```text
wox://toggle
```

No parameters.

### Example

Toggle the Wox launcher from PowerShell:

```powershell
Start-Process "wox://toggle"
```

## confetti: Play an Animation

### Purpose

Play confetti on the display under the mouse without opening the launcher or taking focus. The animation ends after about six seconds; calling it again restarts it.

### Format and Parameters

```text
wox://confetti
```

No parameters.

### Example

Play the animation from a macOS terminal:

```bash
open "wox://confetti"
```

## plugin: Invoke a Plugin

### Purpose

Pass the URL's query parameters to a plugin's registered deep-link callback. The plugin must be installed and support this callback. The plugin defines the actions, parameter meanings, and whether to open its own UI.

### Format and Parameters

```text
wox://plugin/<plugin-id>?<key>=<url-encoded-value>
```

| Field | Description |
| --- | --- |
| `<plugin-id>` | The plugin's metadata `ID`, not its display name. |
| Query parameters | Defined by the plugin. Separate parameters with `&` and URL-encode each value individually. |

### Example

Suppose a plugin has the ID `12345678-1234-1234-1234-123456789abc` and supports opening an item with `action=open` and `id`:

```text
wox://plugin/12345678-1234-1234-1234-123456789abc?action=open&id=123
```

Its callback receives `action` set to `open` and `id` set to `123`. This ID and these parameters are illustrative; replace them according to the target plugin's documentation.

## Open Links from Scripts

Each command below fills in `f invoice` in Wox. Replace the URL with another deep link to use a different feature.

### Windows PowerShell

```powershell
Start-Process "wox://query?q=f%20invoice"
```

### macOS

```bash
open "wox://query?q=f%20invoice"
```

### Linux

```bash
xdg-open "wox://query?q=f%20invoice"
```

## Parameter Encoding

URL-encode parameter values, preserving the URL structure's `://`, `?`, `=`, and the `&` between parameters.

| Character in a value | Encoding |
| --- | --- |
| Space | `%20` |
| `+` | `%2B` |
| `&` | `%26` |
| `#` | `%23` |

For example, the query `A&B` becomes `wox://query?q=A%26B`. When generating links dynamically, use your language's URL-encoding function instead of concatenating unencoded text.

Keep secrets out of deep links; URLs can be recorded by shells, browsers, or automation logs.
