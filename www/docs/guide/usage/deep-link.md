# Deep Link

Deep links let another app or script open Wox with a prepared query. This is useful for terminal aliases, browser shortcuts, automation tools, and app-specific launch buttons.

## Format

```text
wox://query?q=<url-encoded-query>
wox://install?path=<url-encoded-file>
```

The `q` value is the exact query text Wox should place into the launcher. URL-encode spaces and symbols.

`wox://install?path=` opens the local plugin installer for a `.wox` package. Double-clicking a `.wox` file uses this same path after Wox registers the file association.

## Examples

Open Plugin Manager with an install query:

<WoxRunQuery query="wpm install " />

Open file search:

<WoxRunQuery query="f invoice" />

Run a calculation:

<WoxRunQuery query="100 + 20" />

Start an AI chat:

<WoxRunQuery query="chat summarize this" />

Create a note:

<WoxRunQuery query="note new " />

Open a downloaded plugin package in the installer:

<WoxRunQuery href="wox://install?path=/Users/demo/Downloads/wox.plugin.example.wox" />

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
