# Wox Plugin JSON Schema Specification

This document defines the schema for `plugin.json`, the manifest file required for SDK plugins (Node.js/Python).

## File Structure

The `plugin.json` file must be a valid JSON object located in the root of your plugin directory.

> **Note**: Script plugins do **not** use `plugin.json`. They embed a JSON metadata block inside the script file comments.

## Fields Specification

### Required Fields

| Field             | Type       | Description                                                                                        | Example                                    |
| ----------------- | ---------- | -------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| `Id`              | `string`   | Unique identifier (UUID v4 recommended).                                                           | `"570997a3-e47f-4796-9ee8-7dc5df649419"`   |
| `Name`            | `string`   | Display name of the plugin. Supports i18n prefixes (e.g., `i18n:plugin_name`).                     | `"Calculator"`                             |
| `Description`     | `string`   | Short description of the plugin. Supports i18n.                                                    | `"Calculator plugin"`                      |
| `Author`          | `string`   | Name of the author.                                                                                | `"Wox-Launcher"`                           |
| `Website`         | `string`   | URL to the plugin's website or repository.                                                         | `"http://www.github.com/Wox-launcher/Wox"` |
| `Version`         | `string`   | Semantic version string.                                                                           | `"1.0.0"`                                  |
| `MinWoxVersion`   | `string`   | Minimum Wox version required (SemVer).                                                             | `"2.0.0"`                                  |
| `Runtime`         | `string`   | Runtime environment. Enum: `PYTHON`, `NODEJS`.                                                     | `"PYTHON"`                                 |
| `Entry`           | `string`   | Main entry file path relative to plugin root.                                                      | `"main.py"` or `"dist/index.js"`           |
| `Icon`            | `Icon`     | Plugin icon. See [Icon Formats](#icon-formats) below.                                              | `"emoji:🧮"`                               |
| `TriggerKeywords` | `string[]` | Array of keywords to trigger the plugin. Use `"*"` for global triggers (careful!). Can't be empty. | `["calc", "math"]`                         |
| `SupportedOS`     | `string[]` | Array of supported operating systems. Enum: `Windows`, `Linux`, `Darwin`. Can't be empty.          | `["Windows", "Darwin"]`                    |

### Optional Fields

| Field                | Type                   | Description                                                                                    |
| -------------------- | ---------------------- | ---------------------------------------------------------------------------------------------- |
| `I18n`               | `map[lang][key]string` | Inline translations (Recommended). e.g., `{"en_US": {"key": "val"}}`                           |
| `Commands`           | `Command[]`            | List of specific commands provided by the plugin.                                              |
| `Features`           | `Feature[]`            | List of advanced features enabled for the plugin.                                              |
| `SettingDefinitions` | `Setting[]`            | Definition of user-configurable settings. See [SettingDefinitions](#settingdefinitions) below. |

### SettingDefinitions

Define a list of UI controls for user configuration. Each item is an object with `Type` and `Value`.

#### Common Properties

| Property | Type     | Description                                                                                                 |
| -------- | -------- | ----------------------------------------------------------------------------------------------------------- |
| `Type`   | `string` | Component type. Enum: `head`, `textbox`, `checkbox`, `select`, `label`, `newline`, `table`, `selectAIModel` |
| `Value`  | `object` | Configuration object specific to the type.                                                                  |

#### 1. Textbox

```json
{
  "Type": "textbox",
  "Value": {
    "Key": "apiKey",
    "Label": "i18n:api_key",
    "DefaultValue": "",
    "Tooltip": "Enter your API key",
    "MaxLines": 1 // Optional, default 1
  }
}
```

#### 2. Checkbox

```json
{
  "Type": "checkbox",
  "Value": {
    "Key": "enableFeature",
    "Label": "Enable Feature",
    "DefaultValue": "true", // Note: String type "true"/"false"
    "Tooltip": "Toggle this feature"
  }
}
```

#### 3. Select (Dropdown)

```json
{
  "Type": "select",
  "Value": {
    "Key": "theme",
    "Label": "Theme",
    "DefaultValue": "light",
    "Options": [
      { "Label": "Light", "Value": "light" },
      { "Label": "Dark", "Value": "dark" }
    ]
  }
}
```

#### 4. Table (List of Items)

```json
{
  "Type": "table",
  "Value": {
    "Key": "shortcuts",
    "DefaultValue": "[]", // JSON string of list
    "Title": "Shortcuts",
    "Columns": [
      { "Label": "Name", "Key": "name", "Type": "text", "Width": 100 },
      { "Label": "Enabled", "Key": "enabled", "Type": "checkbox", "Width": 50 }
    ]
  }
}
```

#### 5. Other Types

- `head`: Section header `{ "Label": "Section Name" }`
- `label`: Static text `{ "Label": "Some text" }`
- `newline`: Vertical spacer `{}`
- `selectAIModel`: Special dropdown for selecting AI models configured in Wox.
  |

## Icon Formats

The `Icon` string uses a `prefix:data` format. Supported prefixes:

| Prefix      | Description                                  | Example                               |
| ----------- | -------------------------------------------- | ------------------------------------- |
| `emoji:`    | Use a unicode emoji.                         | `"emoji:🚀"`                          |
| `relative:` | Path relative to the plugin directory.       | `"relative:assets/icon.png"`          |
| `absolute:` | Absolute file path.                          | `"absolute:/usr/local/bin/python"`    |
| `fileicon:` | Use the system icon for a specific file/app. | `"fileicon:/Applications/Safari.app"` |
| `base64:`   | Base64 encoded image data (PNG), use data URI format. | `"base64:data:image/png;base64,iVBORw0KGgoAAAANSUhEUg..."`  |
| `svg:`      | raw SVG string.                              | `"svg:<svg>...</svg>"`                |

## Features Specification

Enable optional capabilities by adding them to the `Features` array.

### 1. AI Integration

Allows the plugin to provide LLM-based chat interactions.

```json
{ "Name": "ai" }
```

### 2. Query Selection

Allows the plugin to access the user's currently selected text or files from the active application.

```json
{ "Name": "querySelection" }
```

### 3. Query Environment

Allows the plugin to access context about the active window or browser.

```json
{
  "Name": "queryEnv",
  "Params": {
    "requireActiveWindowName": true,
    "requireActiveWindowPid": true,
    "requireActiveWindowIcon": false,
    "requireActiveWindowIsOpenSaveDialog": false,
    "requireActiveBrowserUrl": false
  }
}
```

### 4. Debounce

Delays query execution until the user stops typing for `IntervalMs`.

```json
{
  "Name": "debounce",
  "Params": { "IntervalMs": 300 }
}
```

### 5. MRU (Most Recently Used)

Enables automatic boosting of frequently used results.

```json
{
  "Name": "mru",
  "Params": { "HashBy": "title" } // Options: "title", "rawQuery", "search"
}
```

### 6. Grid Layout

Displays results in a grid instead of a list.

```json
{
  "Name": "gridLayout",
  "Params": {
    "Columns": 4,
    "ShowTitle": true,
    "ItemPadding": 10,
    "ItemMargin": 5,
    "Commands": [] // Empty = apply to all, or list specific trigger keywords
  }
}
```

### 7. Other Features

- `deepLink`: Handle custom URI schemes (e.g., `wox://plugin/myplugin?arg=value`).
- `ignoreAutoScore`: Disable Wox's default frequency-based learning for this plugin.
- `resultPreviewWidthRatio`: Customize the split ratio between result list and preview panel (0.0 - 1.0).

## Complete Example

```json
{
  "Id": "a1b2c3d4-e5f6-7g8h-9i0j-k1l2m3n4o5p6",
  "Name": "Super Search",
  "Version": "1.2.0",
  "MinWoxVersion": "2.0.0",
  "Runtime": "NODEJS",
  "Entry": "dist/index.js",
  "Icon": "emoji:🔍",
  "TriggerKeywords": ["ss", "search"],
  "SupportedOS": ["Windows", "Darwin"],
  "Features": [{ "Name": "querySelection" }, { "Name": "debounce", "Params": { "IntervalMs": 200 } }]
}
```
