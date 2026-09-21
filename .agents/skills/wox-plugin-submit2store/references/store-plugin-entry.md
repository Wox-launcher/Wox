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
  "IconUrl": "<public icon url>",
  "Website": "<gist html url or github repository url>",
  "DownloadUrl": "<gist raw .js/.py url or .wox release asset url>",
  "ScreenshotUrls": ["<public screenshot url>"],
  "SupportedOS": ["Windows", "Darwin", "Linux"],
  "DateCreated": "YYYY-MM-DD HH:MM:SS",
  "DateUpdated": "YYYY-MM-DD HH:MM:SS"
}
```

Add `I18n` only when localized strings already exist and are ready to publish:

```json
"I18n": {
  "en_US": {
    "plugin_name": "<english name>",
    "plugin_description": "<english description>"
  },
  "zh_CN": {
    "plugin_name": "<simplified chinese name>",
    "plugin_description": "<simplified chinese description>"
  }
}
```

## Sourcing Rules

Read fields from the local `plugin.json` for packaged plugins, or from the single-file `.js`/`.py` header.

- `Id`, `Name`, `Author`, `Version`, `MinWoxVersion`, `Runtime`, `Description`: local metadata. Store `Runtime` is lowercase (`nodejs`, `python`).
- `SupportedOS`: normalize to `Windows`, `Darwin`, `Linux` even if local metadata uses `Macos` or lowercase values.
- `DateCreated` and `DateUpdated`: use the current local time when creating a fresh entry.
- `I18n`: add only when localized `plugin_name` / `plugin_description` strings are ready.

### Single-file SDK (gist)

Prefer a public GitHub gist. Example: https://gist.github.com/qianlifeng/04a9609de66eaa582a473f5852450ede

- `Website`: gist HTML URL, `https://gist.github.com/<user>/<gist-id>`
- `DownloadUrl`: gist raw URL **including the filename**, `https://gist.githubusercontent.com/<user>/<gist-id>/raw/Wox.Plugin.<Name>.js` (or `.py`). The path must end with `.js` or `.py`.
- `IconUrl` and `ScreenshotUrls`: GitHub user-attachment URLs after uploading the images, `https://gist.github.com/user-attachments/assets/<id>` (for example `https://gist.github.com/user-attachments/assets/7502acdc-1ea5-4ef6-a3fb-31875353dabe`)
- Keep exactly one gist file, named `Wox.Plugin.<Name>.js` or `.py`. Do not use a `.wox` package.

### Packaged SDK (GitHub repo)

- `IconUrl`: public raw image in the plugin repository.
- `Website`: canonical GitHub repository URL, not the raw URL.
- `DownloadUrl`: the installable `.wox` release asset, typically `releases/latest/download/<asset>.wox`.
- `ScreenshotUrls`: omit the property only when the plugin truly has no screenshot to publish.

## Validation Checklist

- Verify that `Id` is absent from the current upstream `store-plugin.json` before adding the object.
- Verify that every URL is public and stable.
- For single-file plugins, verify that `DownloadUrl` path ends with `.js` or `.py` and that the gist file name matches.
- For packaged plugins, verify that the download asset name matches the published `.wox` file.
- Verify that the entry matches the existing JSON formatting in the cloned Wox repository.
