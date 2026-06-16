# File Plugin

The File plugin searches files and folders from configured roots. Use the `f` keyword when you want file results instead of global app, web, or calculator results.

## Quick Start

```text
f report
f README.md
f Documents invoice
f .pdf
```

The first run may show indexing status while Wox scans configured roots. Results become faster and more complete after the initial index is ready.

![File plugin search results](/images/system-plugin-filesearch.png)

## What Gets Indexed

Open **Settings -> Plugins -> File** to manage roots. Wox indexes those roots and skips common system/generated directories that are not useful for launcher search.

Use a small, deliberate root list for best results:

- Good: home subfolders, project folders, document folders, mounted work directories.
- Avoid: whole system volumes, dependency caches, build output directories, VM images, and folders with huge generated trees.

## Actions

| Action | Use |
| --- | --- |
| Open | Open the file or folder with the default app |
| Open containing folder | Reveal the result in the file manager |
| Delete | Move the item to trash |
| Show context menu | Use the platform file menu when available |

## Search Tips

- Use file name fragments: `f invoice q1`.
- Add an extension when you know it: `f proposal .pdf`.
- Add a folder hint when names are common: `f design README`.
- If results feel too broad, narrow the configured roots instead of relying only on longer queries.

## Index Status

The File plugin can show toolbar status while it prepares, scans, or syncs roots. This is normal after first launch, after root changes, or after a large file move.

If search status reports a permission problem, grant Wox access to the affected folder and let the index refresh.

## Troubleshooting

### A file is missing

1. Confirm the file is under a configured root.
2. Confirm Wox has permission to read the folder.
3. Wait for indexing to finish if the file was just created.
4. Restart Wox if a removable drive or network mount was attached after Wox started.

### Search is slow

- Remove very large generated folders from roots.
- Keep roots focused on places you actually search.
- Check logs under the Wox data directory if toolbar status stays in one phase for too long.
