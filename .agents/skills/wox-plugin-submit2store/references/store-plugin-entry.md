# Store Plugin Entry

Use this reference when constructing a new object inside `store-plugin.json`.

## Field Mapping

Use this shape as the starting point and adapt it to the current plugin:

```json
{
  "Id": "<plugin-id>",
  "Name": "<plugin-name>",
  "Author": "<author>",
  "Version": "<version>",
  "MinWoxVersion": "<min-wox-version>",
  "Runtime": "<runtime>",
  "Description": "<plain description or i18n key>",
  "IconUrl": "<public raw icon url>",
  "Website": "<github repository url>",
  "DownloadUrl": "<latest release asset url>",
  "ScreenshotUrls": ["<public raw screenshot url>"],
  "SupportedOS": ["Windows", "Darwin", "Linux"],
  "DateCreated": "YYYY-MM-DD HH:MM:SS",
  "DateUpdated": "YYYY-MM-DD HH:MM:SS"
}
```

Add `I18n` only when localized strings already exist and are ready to publish:

```json
"I18n": {
  "en_US": {
    "plugin_description": "<english description>"
  },
  "zh_CN": {
    "plugin_description": "<simplified chinese description>"
  }
}
```

## Sourcing Rules

- `Id`: read from local `plugin.json`.
- `Name`: read from local `plugin.json`.
- `Author`: read from local `plugin.json`.
- `Version`: prefer local `plugin.json`; use `package.json` only to confirm consistency.
- `MinWoxVersion`: read from local `plugin.json`.
- `Runtime`: read from local `plugin.json`.
- `Description`: use the local plugin description when no i18n block is needed.
- `IconUrl`: point at a public raw image in the plugin repository.
- `Website`: use the canonical GitHub repository URL, not the raw URL.
- `DownloadUrl`: point at the release asset users actually install, typically `releases/latest/download/<asset>.wox`.
- `ScreenshotUrls`: omit the property only when the plugin truly has no screenshot to publish.
- `SupportedOS`: normalize to store casing even if local metadata uses lowercase values.
- `DateCreated` and `DateUpdated`: use the current local time when creating a fresh entry.

## Validation Checklist

- Verify that `Id` is absent from the current upstream `store-plugin.json` before adding the object.
- Verify that every URL is public and stable.
- Verify that the download asset name matches the published `.wox` file.
- Verify that the entry matches the existing JSON formatting in the cloned Wox repository.
