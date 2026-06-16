# Deep Link

Deep links let another app or script open Wox with a prepared query. This is useful for terminal aliases, browser shortcuts, automation tools, and app-specific launch buttons.

## Format

```text
wox://query?q=<url-encoded-query>
```

The `q` value is the exact query text Wox should place into the launcher. URL-encode spaces and symbols.

## Examples

Open Plugin Manager with an install query:

```text
wox://query?q=wpm%20install
```

Open file search:

```text
wox://query?q=f%20invoice
```

Run a calculation:

```text
wox://query?q=100%20%2B%2020
```

Start an AI chat:

```text
wox://query?q=chat%20summarize%20this
```

## From Scripts

macOS:

```bash
open "wox://query?q=f%20invoice"
```

Windows PowerShell:

```powershell
Start-Process "wox://query?q=f%20invoice"
```

Linux:

```bash
xdg-open "wox://query?q=f%20invoice"
```

## Encoding Notes

- Encode spaces as `%20`.
- Encode `+` as `%2B` when it is part of a calculation.
- Keep secrets out of deep links; URLs can be recorded by shells, browsers, or automation logs.
