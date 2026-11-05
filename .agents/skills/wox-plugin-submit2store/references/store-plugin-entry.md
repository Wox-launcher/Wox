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
  "DownloadUrl": "<gist raw .js/.py url or github latest-release .wox asset url>",
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
- `DateCreated`: use the current local time. `DateUpdated`: use the latest release's publication time in UTC for packaged plugins, or the current UTC time for single-file plugins, in `YYYY-MM-DD HH:MM:SS` format.
- `I18n`: add only when localized `plugin_name` / `plugin_description` strings are ready.

### Single-file SDK (gist)

Prefer a public GitHub gist. Example: https://gist.github.com/qianlifeng/04a9609de66eaa582a473f5852450ede

- `Website`: gist HTML URL, `https://gist.github.com/<user>/<gist-id>`
- `DownloadUrl`: unversioned gist raw URL **including the filename**, `https://gist.githubusercontent.com/<user>/<gist-id>/raw/Wox.Plugin.<Name>.js` (or `.py`). Do not include a revision hash; the URL must track the current gist content. The GitHub Release `latest` URL rule does not apply to gists.
- `IconUrl` and `ScreenshotUrls`: GitHub user-attachment URLs after uploading the images, `https://gist.github.com/user-attachments/assets/<id>` (for example `https://gist.github.com/user-attachments/assets/7502acdc-1ea5-4ef6-a3fb-31875353dabe`). `ScreenshotUrls` is required and must contain at least one image of the plugin running in Wox.
- Keep exactly one gist file, named `Wox.Plugin.<Name>.js` or `.py`. Do not use a `.wox` package.

### Packaged SDK (GitHub repo)

- `IconUrl`: public raw image in the plugin repository.
- `Website`: canonical GitHub repository URL, not the raw URL.
- `DownloadUrl`: use `https://github.com/<owner>/<repo>/releases/latest/download/<asset>.wox`, never a version-pinned release URL. The latest release must contain the asset, and its filename must be stable across releases rather than embedding a version. If there is no stable asset, stop and explain that the release needs one before submission.
- Verify the published asset's `plugin.json` matches the entry's `Id`, `Version`, and `MinWoxVersion` (defaulting the latter to `2.0.0` if omitted).
- `ScreenshotUrls`: required for every plugin. Include at least one public URL that shows the plugin running in Wox. Do not omit the field or leave it empty. Use a cropped Wox launcher screenshot that shows the query and the plugin's real results, like https://raw.githubusercontent.com/Wox-launcher/Wox.Plugin.ColorPicker/main/images/screenshot1.png. Do not submit an icon, a README image, source code, or a full-desktop capture.

## Validation Checklist

- Verify that `Id` is absent from the current upstream `store-plugin.json` before adding the object.
- Verify that every URL is public and stable.
- Verify that `ScreenshotUrls` contains at least one public image URL. Do not submit the entry without it.
- For single-file plugins, verify that `DownloadUrl` is a public unversioned gist raw URL, its path ends with `.js` or `.py`, and the gist file name matches.
- For packaged plugins, verify that the latest-release URL downloads the published `.wox` file and its stable asset name and metadata meet the rules above.
- Verify that the entry matches the existing JSON formatting in the cloned Wox repository.
