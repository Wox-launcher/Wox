# Third-party launcher import

Each launcher under this folder registers one `migrate.Source`. Onboarding discovers installations through that API and does not name a product.

## Main hotkey

When the source launcher has a main shortcut that can be written as a Wox chord, the catalog always includes it and import always stages it.

- Catalog id is `setting:main_hotkey`, title key is `main_hotkey`, and the row stays selectable.
- `Import` returns one `SettingWrite` with key `MainHotkey` and that chord. Do not register the shortcut with the operating system here.
- Onboarding keeps the chord and shows it on the next hotkey step, where the user confirms it.
- A chord that matches Wox's current main hotkey, or that another program already holds, is still listed and still imported. The hotkey step shows the imported chord and reports the conflict there. Do not drop the row, mark it unsupported, or leave the previous shortcut on that page.

Omit the row only when the source shortcut cannot be expressed as a Wox chord. There is then nothing for the hotkey step to confirm.

## Quicklinks

A link quicklink becomes a Web Search row. A file or folder quicklink becomes a Folder favorite.

- An `http` or `https` target stays a quicklink: keyword from the name, URL with Wox parameter placeholders, optional browser.
- An absolute path, a `file:` URL, or a `~/` path stays in the Quicklinks catalog and imports as a Folder favorite. The favorite name is the quicklink name and the path is the file or folder. Folder search opens either one by that name.
- Hide a path that is missing, a name that already points at the same path, and a name that already points at a different path. Do not overwrite an existing favorite.
