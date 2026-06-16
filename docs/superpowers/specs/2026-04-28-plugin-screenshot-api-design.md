# Plugin Screenshot API Design

## Goal

Expose Wox screenshot capture to third-party plugins through a small, stable API that does not leak the internal UI screenshot protocol.

## Public Contract

Plugins call `screenshot(ctx, option)` in Python or `Screenshot(ctx, option)` in TypeScript/JavaScript.

`option` is an object reserved for future expansion. The first field is `WriteToClipboard bool`, which asks Wox to write the completed screenshot image to the system clipboard in addition to saving the PNG file.

`result` is intentionally narrow:

```json
{
  "success": true,
  "screenshotPath": "/path/to/file.png",
  "errmsg": ""
}
```

When the user cancels or capture fails, `success` is false, `screenshotPath` is empty, and `errmsg` contains a readable reason. If the screenshot file is saved but clipboard writing fails, the result remains successful and `errmsg` carries the clipboard warning so plugins can decide whether to surface it.

## Architecture

The existing internal `common.CaptureScreenshotRequest` and `common.CaptureScreenshotResult` remain the Go-to-Flutter bridge contract. A new plugin-level adapter in `wox.core/plugin` translates the public `ScreenshotOption` into the internal request and maps the internal result into the public `ScreenshotResult`.

The websocket plugin host accepts a new `Screenshot` method and returns only the narrowed public result to host runtimes. Node and Python host implementations serialize the option object to that method and SDK type definitions expose matching typed models.

## Error Handling

Transport or UI bridge errors return `success=false` with the wrapped error text. Internal screenshot status `cancelled` maps to `success=false, errmsg="cancelled"`. Internal status `failed` maps to `success=false` with the native error message when present.

## Testing And Verification

This change reuses the existing screenshot smoke coverage for the actual UI capture workflow. Verification should include formatting touched Go/Dart-free files as needed, TypeScript/Python type edits by inspection, and `make build` in `wox.core`.
