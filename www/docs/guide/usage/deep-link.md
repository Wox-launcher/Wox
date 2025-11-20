# Deep Link

Wox supports deep linking, allowing you to trigger Wox queries from external applications or scripts.

## URL Scheme

The URL scheme for Wox is `wox://`.

## Format

```
wox://query?q=<query>
```

- `q`: The query string you want to execute. It should be URL-encoded.

## Examples

### Open Wox with a pre-filled query

```
wox://query?q=wpm%20install
```

This will open Wox and type `wpm install` into the search box.

### Trigger a specific plugin

```
wox://query?q=calc%201%2B1
```

This will open Wox and calculate `1+1` using the calculator plugin (assuming `calc` is the trigger).

## Usage in Scripts

You can use deep links in your shell scripts or other automation tools.

**macOS:**

```bash
open "wox://query?q=test"
```

**Windows:**

```powershell
start "wox://query?q=test"
```

**Linux:**

```bash
xdg-open "wox://query?q=test"
```
