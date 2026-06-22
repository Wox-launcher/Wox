# Clipboard Plugin

Clipboard keeps recent text and image clipboard items so you can reuse them without switching to a separate clipboard manager.

## Quick Start

```text
cb
cb invoice
cb fav
```

| Query | Result |
| --- | --- |
| `cb` | Show recent clipboard history |
| `cb <keyword>` | Filter by text or alias |
| `cb fav` | Show favorites |

Press `Enter` to run the configured primary action: copy the item back to the clipboard or paste it into the active app.

![Clipboard plugin history results](/images/system-plugin-clipboard.png)

## Actions

Open the Action Panel to favorite an item, edit its alias, delete it, open a copied path, or choose copy/paste explicitly.

## Settings

- Keep text history and retention days.
- Keep image history and retention days.
- Choose whether the primary action copies or pastes.
- Tune behavior if you want Wox to avoid storing sensitive clipboard content.
