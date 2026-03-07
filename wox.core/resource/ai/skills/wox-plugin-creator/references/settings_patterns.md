# Wox Plugin Settings Patterns

This document contains self-contained settings examples.
Use it when `plugin.json` needs `SettingDefinitions`, validators, dynamic settings, or advanced table/select controls.

## Choose The Right Pattern

- Use a `textbox` for free-form string input.
- Add validators when empty or non-numeric values should be rejected.
- Use a `select` for a fixed option list.
- Use `selectAIModel` when the value should come from Wox-configured AI models.
- Use a `table` when users need to add, edit, or delete rows.
- Use a `dynamic` placeholder when the visible setting depends on current runtime state.

## Pattern 1: Textbox With Validators

```json
{
  "Type": "textbox",
  "Value": {
    "Key": "timeout_ms",
    "Label": "Timeout",
    "DefaultValue": "300",
    "Tooltip": "Delay before query execution",
    "Suffix": "ms",
    "MaxLines": 1,
    "Validators": [
      {
        "Type": "is_number",
        "Value": {
          "IsInteger": true,
          "IsFloat": false
        }
      }
    ],
    "Style": {
      "Width": 220
    }
  }
}
```

Use `not_empty` for required strings:

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

## Pattern 2: Select And Multi-Select

```json
{
  "Type": "select",
  "Value": {
    "Key": "theme",
    "Label": "Theme",
    "DefaultValue": "dark",
    "Tooltip": "Choose one theme",
    "IsMulti": false,
    "Options": [
      { "Label": "Dark", "Value": "dark" },
      { "Label": "Light", "Value": "light" }
    ],
    "Validators": [],
    "Style": {}
  }
}
```

Multi-select example:

```json
{
  "Type": "select",
  "Value": {
    "Key": "providers",
    "Label": "Providers",
    "DefaultValue": "",
    "IsMulti": true,
    "Options": [
      { "Label": "All", "Value": "all", "IsSelectAll": true },
      { "Label": "OpenAI", "Value": "openai" },
      { "Label": "Anthropic", "Value": "anthropic" }
    ]
  }
}
```

## Pattern 3: AI Model Selector

```json
{
  "Type": "selectAIModel",
  "Value": {
    "Key": "default_model",
    "Label": "Default Model",
    "DefaultValue": "",
    "Tooltip": "Choose one configured AI model",
    "Validators": [],
    "Style": {}
  }
}
```

## Pattern 4: Editable Table

Use tables when a plugin needs a list of structured items.

```json
{
  "Type": "table",
  "Value": {
    "Key": "shortcuts",
    "DefaultValue": "[]",
    "Title": "Shortcuts",
    "Tooltip": "Manage saved shortcuts",
    "SortColumnKey": "name",
    "SortOrder": "asc",
    "MaxHeight": 420,
    "Columns": [
      {
        "Key": "name",
        "Label": "Name",
        "Type": "text",
        "Width": 120,
        "Validators": [{ "Type": "not_empty", "Value": {} }]
      },
      {
        "Key": "enabled",
        "Label": "Enabled",
        "Type": "checkbox",
        "Width": 70
      },
      {
        "Key": "model",
        "Label": "Model",
        "Type": "selectAIModel",
        "Width": 130
      }
    ]
  }
}
```

Useful table column options:

- `TextMaxLines`: multi-line text editor for `text` columns
- `SelectOptions`: option list for `select` columns
- `HideInTable`: keep field in the edit dialog only
- `HideInUpdate`: show field in the table only

## Pattern 5: Dynamic Setting

Use `dynamic` when the displayed setting should be computed at runtime.
The `dynamic` entry itself is only a placeholder; the plugin API callback must return the actual setting item.

Manifest:

```json
{
  "Type": "dynamic",
  "Value": {
    "Key": "separator_preview"
  }
}
```

Node.js callback pattern:

```typescript
await api.OnGetDynamicSetting(ctx, async (_ctx, key) => {
  if (key === "separator_preview") {
    return {
      Type: "label",
      Value: {
        Content: "Preview: 1,234.56",
        Tooltip: "",
        Style: {},
      },
    };
  }

  return {
    Type: "label",
    Value: {
      Content: "Unknown setting",
      Tooltip: "",
      Style: {},
    },
  };
});
```

Python callback pattern:

```python
from wox_plugin import (
    PluginSettingDefinitionItem,
    PluginSettingDefinitionType,
    PluginSettingValueLabel,
)

async def _on_get_dynamic_setting(ctx, key):
    if key == "separator_preview":
        return PluginSettingDefinitionItem(
            type=PluginSettingDefinitionType.LABEL,
            value=PluginSettingValueLabel(content="Preview: 1,234.56"),
        )

    return PluginSettingDefinitionItem(
        type=PluginSettingDefinitionType.LABEL,
        value=PluginSettingValueLabel(content="Unknown setting"),
    )
```

## Pattern 6: Platform-Specific Settings

Use these top-level properties on any setting item:

```json
{
  "Type": "textbox",
  "Value": {
    "Key": "executable_path",
    "Label": "Executable Path",
    "DefaultValue": ""
  },
  "DisabledInPlatforms": ["linux"],
  "IsPlatformSpecific": true
}
```

- `DisabledInPlatforms`: disable the control on selected platforms
- `IsPlatformSpecific`: store a different value per platform

## Node.js Notes

- `SaveSetting` requires `isPlatformSpecific`.
- `OnGetDynamicSetting` is the runtime hook for `dynamic`.
- Keep settings JSON-compatible; avoid relying on repo-local type files being present.

## Python Notes

- Helper builders are intentionally limited.
- For `select`, `table`, validators, or `dynamic`, build `PluginSettingDefinitionItem` and value objects directly.
- `save_setting` requires `is_platform_specific`.
