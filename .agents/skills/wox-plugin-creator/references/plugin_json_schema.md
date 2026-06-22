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
| `QueryRequirements`  | `QueryRequirements`    | Static query preconditions for required settings. See [QueryRequirements](#queryrequirements). |

### QueryRequirements

Use `QueryRequirements` when a plugin query cannot run until one or more settings are configured. Wox checks these requirements before calling the plugin `query()` method. If a requirement fails, Wox returns a system setup result with the `query_requirement_settings` preview and shows a focused settings form for only the missing fields.

Do not return a custom "please configure this plugin" result from `query()` for these cases. Static metadata gives grid results and normal list results the same setup flow.

#### Scopes

| Field                 | Type                                  | Checked when                                                                 |
| --------------------- | ------------------------------------- | ---------------------------------------------------------------------------- |
| `AnyQuery`            | `QueryRequirement[]`                  | Every query for the plugin.                                                  |
| `QueryWithoutCommand` | `QueryRequirement[]`                  | Query has no command. Wox checks `AnyQuery + QueryWithoutCommand`.           |
| `QueryWithCommand`    | `map[command]QueryRequirement[]`      | Query has that command. Wox checks `AnyQuery + QueryWithCommand[command]`.   |

If a query has a command that is not declared in `QueryWithCommand`, Wox checks only `AnyQuery`.

#### Requirement Shape

```json
{
  "SettingKey": "accessKey",
  "Validators": [{ "Type": "not_empty", "Value": {} }],
  "Message": "i18n:access_key_required"
}
```

Rules:

- `SettingKey` must match a configurable setting key from `SettingDefinitions`.
- `Validators` is optional. If omitted or empty, Wox reuses the validators from the matching setting definition.
- If both the requirement and setting definition have no validators, Wox logs a metadata issue and does not block the query.
- `Message` is optional and supports `i18n:` keys. Use it for field-specific setup guidance.

Example:

```json
{
  "SettingDefinitions": [
    {
      "Type": "textbox",
      "Value": {
        "Key": "accessKey",
        "Label": "i18n:access_key",
        "DefaultValue": "",
        "Tooltip": "i18n:access_key_tooltip",
        "Validators": [{ "Type": "not_empty", "Value": {} }],
        "Style": { "Width": 400 }
      }
    }
  ],
  "QueryRequirements": {
    "AnyQuery": [
      {
        "SettingKey": "accessKey",
        "Message": "i18n:access_key_required"
      }
    ],
    "QueryWithoutCommand": [],
    "QueryWithCommand": {
      "download": [
        {
          "SettingKey": "downloadDirectory",
          "Validators": [{ "Type": "not_empty", "Value": {} }],
          "Message": "i18n:download_directory_required"
        }
      ]
    }
  }
}
```

### SettingDefinitions

Define a list of UI controls for user configuration. Each item is an object with `Type` and `Value`.
Use this reference as the source of truth when authoring `SettingDefinitions` in a published plugin or skill.

#### Common Properties

| Property              | Type       | Description                                                                                                       |
| --------------------- | ---------- | ----------------------------------------------------------------------------------------------------------------- |
| `Type`                | `string`   | Component type. Enum: `head`, `textbox`, `checkbox`, `select`, `label`, `newline`, `table`, `selectAIModel`, `dynamic` |
| `Value`               | `object`   | Configuration object specific to the type.                                                                        |
| `DisabledInPlatforms` | `string[]` | Optional. Platforms where this setting is disabled. Uses SDK platform names such as `windows`, `darwin`, `linux`. |
| `IsPlatformSpecific`  | `boolean`  | Optional. If `true`, Wox stores different values per platform. Normal plugin settings are eligible for cloud sync, so use this for values that should not be shared across Windows, macOS, and Linux. |

#### Cloud Sync And Platform-Specific Settings

Before adding a setting, decide whether cloud sync should share one value across all devices or keep a separate value per platform.

- Use `IsPlatformSpecific: false` for account IDs, API keys, remote service hosts, feature toggles, and user preferences that mean the same thing on every platform.
- Use `IsPlatformSpecific: true` for local paths, executable paths, shell commands, hotkeys, system integrations, browser profiles, application paths, and any value that is likely to be invalid on another OS.
- `DisabledInPlatforms` only controls where a setting is disabled in the UI. It does not create separate stored values and should not be used as a substitute for `IsPlatformSpecific`.
- When code calls `SaveSetting` / `save_setting`, pass the same platform-specific decision used by the setting metadata. Do not hardcode `false` for dynamically saved platform-specific settings.

Shared value example:

```json
{
  "Type": "textbox",
  "Value": {
    "Key": "api_key",
    "Label": "API Key",
    "DefaultValue": ""
  },
  "IsPlatformSpecific": false
}
```

Platform-specific value example:

```json
{
  "Type": "textbox",
  "Value": {
    "Key": "executable_path",
    "Label": "Executable Path",
    "DefaultValue": ""
  },
  "IsPlatformSpecific": true
}
```

#### Validators

Validators are used inside textbox/select values and table columns.

Supported validator types:

- `not_empty`
- `is_number`

Examples:

```json
{
  "Type": "textbox",
  "Value": {
    "Key": "timeout_ms",
    "Label": "Timeout",
    "DefaultValue": "300",
    "Validators": [
      {
        "Type": "is_number",
        "Value": { "IsInteger": true, "IsFloat": false }
      }
    ]
  }
}
```

```json
{
  "Type": "textbox",
  "Value": {
    "Key": "api_key",
    "Label": "API Key",
    "DefaultValue": "",
    "Validators": [
      {
        "Type": "not_empty",
        "Value": {}
      }
    ]
  }
}
```

#### 1. Textbox

```json
{
  "Type": "textbox",
  "Value": {
    "Key": "apiKey",
    "Label": "i18n:api_key",
    "DefaultValue": "",
    "Tooltip": "Enter your API key",
    "MaxLines": 1,
    "Suffix": "",
    "Validators": [],
    "Style": { "Width": 400 }
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
    "DefaultValue": "true",
    "Tooltip": "Toggle this feature",
    "Style": {}
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
    "Suffix": "",
    "Tooltip": "Theme selection",
    "IsMulti": false,
    "Options": [
      { "Label": "Light", "Value": "light" },
      { "Label": "Dark", "Value": "dark" },
      { "Label": "All", "Value": "all", "IsSelectAll": true }
    ],
    "Validators": [],
    "Style": {}
  }
}
```

`select` notes:

- `IsMulti` enables multi-select.
- `Options[].Icon` is supported.
- `Options[].IsSelectAll` is supported for multi-select flows.

#### 4. SelectAIModel

```json
{
  "Type": "selectAIModel",
  "Value": {
    "Key": "default_model",
    "Label": "Default model",
    "DefaultValue": "",
    "Tooltip": "Choose one configured AI model",
    "Validators": [],
    "Style": {}
  }
}
```

#### 5. Table (List of Items)

```json
{
  "Type": "table",
  "Value": {
    "Key": "shortcuts",
    "DefaultValue": "[]",
    "Title": "Shortcuts",
    "Tooltip": "Editable shortcut list",
    "SortColumnKey": "name",
    "SortOrder": "asc",
    "MaxHeight": 500,
    "Columns": [
      {
        "Label": "Name",
        "Key": "name",
        "Type": "text",
        "Width": 100,
        "Validators": [{ "Type": "not_empty", "Value": {} }]
      },
      { "Label": "Enabled", "Key": "enabled", "Type": "checkbox", "Width": 50 },
      {
        "Label": "Model",
        "Key": "model",
        "Type": "selectAIModel",
        "Width": 120
      }
    ],
    "Style": {}
  }
}
```

Supported table column types include:

- `text`
- `textList`
- `checkbox`
- `dirPath`
- `select`
- `selectAIModel`
- `aiModelStatus`
- `aiMCPServerTools`
- `aiSelectMCPServerTools`
- `woxImage`

Table column notes:

- `Validators` are supported on columns.
- `SelectOptions` are only used when column type is `select`.
- `TextMaxLines` is only used when column type is `text`.
- `HideInTable` hides the column in the list but keeps it in the edit dialog.
- `HideInUpdate` hides the column in the edit dialog but keeps it in the list.

#### 6. Dynamic

`dynamic` is a placeholder setting that must be resolved at runtime through the plugin API callback:

- Node.js: `API.OnGetDynamicSetting(...)`
- Python: `api.on_get_dynamic_setting(...)`

Example:

```json
{
  "Type": "dynamic",
  "Value": {
    "Key": "separatorPreview"
  }
}
```

#### 7. Other Types

- `head`: Section header using `Content`, `Tooltip`, and `Style`
- `label`: Static text using `Content`, `Tooltip`, and `Style`
- `newline`: Vertical spacer with optional `Style`

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
| `url:`      | Remote image URL.                            | `"url:https://example.com/icon.png"`  |
| `lottie:`   | Lottie animation JSON.                       | `"lottie:{...}"`                      |

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

### 6. Grid Layout (Deprecated)

Displays results in a grid instead of a list.

Prefer returning `QueryResponse.Layout.GridLayout` from the SDK query method.
The metadata feature is deprecated because it can only express static plugin or
command defaults, while QueryResponse can choose the layout for each query.

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
- `resultPreviewWidthRatio` (deprecated): Customize the split ratio between result list and preview panel (0.0 - 1.0). Prefer `QueryResponse.Layout.ResultPreviewWidthRatio` so the ratio can change per query.

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
