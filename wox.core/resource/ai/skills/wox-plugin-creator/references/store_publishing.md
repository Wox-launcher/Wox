# Publishing To The Wox Store

This guide explains how to get a plugin into the official Wox store.
Use it when the goal is not just "publish my package", but "make my plugin appear in Wox store search and install correctly".

## What The Store Actually Installs

Wox store entries point to a public manifest entry plus a downloadable plugin artifact.

- SDK plugins (`nodejs`, `python`) install from a downloadable `.wox` archive.
- Script plugins (`script`) install from a direct script file download.

The store metadata must therefore match the install artifact type.

## Official Store Model

The official store is the `store-plugin.json` file in the Wox repository.
Each store entry contains public metadata plus the download URL used by Wox at install time.

For the official store, the normal flow is:

1. Publish your plugin artifact at a stable public URL.
2. Add your plugin entry to `store-plugin.json`.
3. Submit a PR to the Wox repository.
4. After that first store entry is merged, future plugin releases normally do not need another store PR, as long as the same stable download URL keeps serving the latest release artifact.

Common fields:

```json
{
  "Id": "36fa2371-144c-4f53-98bb-874f77b21849",
  "Name": "Mole",
  "Author": "qianlifeng",
  "Version": "0.0.1",
  "MinWoxVersion": "2.0.0",
  "Runtime": "python",
  "Description": "Dig deep like a mole to optimize your Mac",
  "IconUrl": "https://example.com/icon.png",
  "Website": "https://github.com/example/Wox.Plugin.Mole",
  "DownloadUrl": "https://github.com/example/Wox.Plugin.Mole/releases/latest/download/Wox.Plugin.Mole.wox",
  "ScreenshotUrls": ["https://example.com/screenshot.png"],
  "SupportedOS": ["Darwin"],
  "DateCreated": "2025-11-26 22:15:00",
  "DateUpdated": "2025-11-26 22:15:00"
}
```

Optional fields:

- `IconEmoji` instead of `IconUrl`
- `I18n` for localized `Name` and `Description`

## Store Entry Rules

- `Id` must match the plugin artifact metadata.
- `Version` should match the version used when the plugin is first submitted to the store.
- `Runtime` must reflect the install type:
  - `nodejs`
  - `python`
  - `script`
- `DownloadUrl` must be directly downloadable by Wox over HTTP(S).
- `DownloadUrl` should be stable across future releases.
- `SupportedOS` should use `Windows`, `Darwin`, `Linux`.
- `DateCreated` and `DateUpdated` should use `YYYY-MM-DD HH:MM:SS`.

## SDK Plugin Publishing Flow

Use this for Node.js and Python SDK plugins.

1. Build the plugin release artifact.
2. Produce a `.wox` archive that contains `plugin.json` at the archive root.
3. Upload the `.wox` file to a stable public URL, typically a GitHub release asset.
4. Add the plugin to `store-plugin.json` and set `DownloadUrl` to that `.wox`.
5. Submit a PR to the Wox repository.

Important packaging note:

- Wox local/store install expects a zip-based plugin archive.
- `plugin.json` must exist inside the archive and be readable after unzip.

Recommended `DownloadUrl` pattern:

```text
https://github.com/<owner>/<repo>/releases/latest/download/<plugin>.wox
```

Why this matters:

- The `latest/download` form gives Wox a stable install/update URL.
- After the plugin has been added to `store-plugin.json` once, future GitHub releases can usually be picked up without another store PR.

## Script Plugin Publishing Flow

Use this for single-file script plugins.

1. Keep the plugin as a single `.py` or `.js` file.
2. Put the full metadata block in the script header comments.
3. Host the raw script file at a stable public URL.
4. Add the plugin to `store-plugin.json` with:
   - `Runtime: "script"`
   - `DownloadUrl` pointing directly to the raw script file
5. Submit a PR to the Wox repository.

Important script note:

- Wox downloads the script file directly and parses the header metadata from that file.
- There is no separate `plugin.json` for script store installs.
- The script header `Id` and `Version` should match the store entry.

Recommended `DownloadUrl` patterns:

```text
https://gist.githubusercontent.com/<user>/<gist>/raw/<file>
https://raw.githubusercontent.com/<owner>/<repo>/<branch>/<path>/<file>.py
```

## I18n In Store Entries

Store manifests support inline `I18n`.
This lets `Name` and `Description` use `i18n:` keys.

Example:

```json
{
  "Name": "i18n:plugin_name",
  "Description": "i18n:plugin_desc",
  "I18n": {
    "en_US": {
      "plugin_name": "My Plugin",
      "plugin_desc": "A useful plugin"
    },
    "zh_CN": {
      "plugin_name": "我的插件",
      "plugin_desc": "一个有用的插件"
    }
  }
}
```

## Checklist Before Submitting

- The install artifact is publicly downloadable without auth.
- `Id`, `Version`, and `Runtime` match between store entry and artifact.
- `DownloadUrl` is stable and points to the actual installable file.
- `Website` points to a repo or homepage users can inspect.
- `IconUrl` or `IconEmoji` is present.
- `ScreenshotUrls` are valid and public.
- `SupportedOS` is accurate.
- The first store submission is a PR against `store-plugin.json`.

## How To Reach The Official Store

If the goal is the official Wox store, prepare a store entry and submit it to the official `store-plugin.json` used by Wox.
In practice, that means updating the Wox repository store manifest with your plugin metadata and public download URL, then opening a PR.

If you are working outside the Wox monorepo, do not depend on local repo files existing. Instead:

- prepare the JSON entry using this reference
- host your plugin artifact publicly
- submit the `store-plugin.json` change through the normal repository contribution flow for the official store

In most cases, this PR is only needed once.
You would usually submit another store PR only when store metadata changes, for example:

- `DownloadUrl` changes
- repo or homepage changes
- icon or screenshots change
- supported platforms change
- plugin name/description metadata needs correction

## Authoring Tips For The Skill

- When the user says "publish to npm/PyPI", that is package publishing, not store publishing.
- When the user says "publish to Wox store" or "make it installable in store", ensure the answer covers both:
  - producing the install artifact
  - submitting the initial `store-plugin.json` PR
- For later releases, do not default to telling the user to edit the store manifest again unless store metadata changed.
- Prefer explicit examples for `DownloadUrl`, because that is where store publishing usually fails.
