# Screenshot Native Clipboard Handoff Design

## Summary

Move the screenshot plugin's clipboard write from `wox.core` into the screenshot session that already runs in Flutter and the platform runners. Flutter will still compose the final annotated image and persist the exported PNG, but it will immediately ask the platform runner to write that exported image to the system clipboard instead of returning control to Go for a second read/decode/write pass.

This change applies only to the screenshot flow. The existing Go clipboard utilities remain in place for clipboard history, plugin actions, and other non-screenshot features.

## Goals

- Remove the extra Go-side PNG read/decode/write step from the screenshot completion path.
- Keep `screenshotPath` as the durable exported artifact for later reuse.
- Keep screenshot export ownership in the screenshot session while moving platform clipboard format handling to the native runners.
- Treat "file exported successfully but clipboard write failed" as a completed screenshot with a warning instead of a failed screenshot.
- Store screenshot exports under `woxDataDirectory`.

## Non-Goals

- Do not migrate the general clipboard subsystem out of `wox.core`.
- Do not redesign screenshot annotations, selection UX, or display capture behavior.
- Do not add automated tests as part of this change. Verification for this work is manual.

## Current Problem

The current screenshot flow renders the final image in Flutter, writes a PNG to disk, returns the PNG path to Go, and then asks Go to reopen the file and write the image to the clipboard. That creates an unnecessary handoff for the screenshot-only path:

1. Flutter renders the final annotated screenshot.
2. Flutter exports the PNG to disk.
3. Go reads the PNG back from disk.
4. Go decodes the PNG into an image.
5. Go writes the image to the clipboard through the platform-specific clipboard package.

The duplicated file read and PNG decode are not needed when the screenshot image is already finalized in the screenshot session and the platform runners are the correct place to handle native clipboard formats anyway.

## Chosen Approach

Use a screenshot-only native clipboard handoff:

- Go allocates the export file path in `woxDataDirectory`.
- Flutter writes the final screenshot PNG to that path.
- Flutter immediately calls a platform-runner clipboard method with the exported file path.
- The platform runner writes the image to the clipboard using the platform-native API and format expectations.
- Flutter returns a completed screenshot result that always includes `screenshotPath`.
- If clipboard write fails after the file export succeeds, Flutter still returns `completed` and includes clipboard warning fields.
- Go stops writing screenshot images to the clipboard and only reacts to the returned result.

This keeps the screenshot result generation close to the annotation pipeline, removes the extra Go-side image decode, and confines screenshot clipboard format decisions to the native layer that already owns platform-specific screenshot behavior.

## Architecture Changes

### 1. Go owns the export path

`wox.core` will stop relying on Flutter's hard-coded screenshot directory selection. Before the screenshot session starts, Go will allocate the target export path under:

- `path.Join(util.GetLocation().GetWoxDataDirectory(), "screenshots", <timestamped-file>.png)`

That path will be passed to Flutter as part of the screenshot request contract.

Reason:

- `wox.core` already owns the canonical location model for Wox data.
- Export path policy should not be duplicated inside Flutter.
- The screenshot artifact is operational Wox data, not user-configurable user-data content.

### 2. Flutter owns final image export

Flutter will keep the existing responsibilities that already belong to the screenshot editor:

- compose the final image
- apply annotations
- write the final PNG file to the export path supplied by Go

Flutter will stop deciding the screenshot directory itself. Its export code will consume the provided `exportFilePath` and write only to that location.

### 3. Flutter initiates clipboard write immediately

Once Flutter has successfully written the final PNG to disk, it will immediately call a screenshot platform bridge method dedicated to screenshot clipboard export, using the exported file path.

Recommended bridge shape:

- `writeClipboardImageFile(filePath)`

This keeps the bridge aligned with the actual artifact that Flutter already produced and avoids reintroducing a large byte payload over the Flutter method channel.

### 4. Native runners own screenshot clipboard formats

Each platform runner will implement the screenshot clipboard write using its own native APIs:

- macOS: write the exported image through `NSPasteboard`
- Windows: decode the exported PNG in the runner and publish the required native clipboard formats such as `CF_DIB` plus PNG where supported
- Linux: decode the exported PNG in the runner and write it through the local Linux clipboard implementation

The runner implementation is allowed to use the exported PNG file as the interchange artifact, because the expensive cross-layer detour being removed is the Go-side decode/write pass, not every native image decode that the platform clipboard API may still require.

## Data Contract Changes

### CaptureScreenshotRequest

Add a required export destination field:

- `exportFilePath string`

This field is populated by Go and consumed by Flutter.

### CaptureScreenshotResult

Keep the existing status model and add explicit clipboard warning fields:

- `status`
- `screenshotPath`
- `clipboardWriteSucceeded`
- `clipboardWarningMessage`

Behavior:

- If export fails, return `failed`.
- If export succeeds, always return `completed`.
- If export succeeds but clipboard write fails, still return `completed`, include `screenshotPath`, set `clipboardWriteSucceeded=false`, and populate `clipboardWarningMessage`.
- If export succeeds and clipboard write succeeds, return `completed` with `clipboardWriteSucceeded=true`.

This avoids overloading the existing failure fields with a partial-success case.

## Go Plugin Behavior

The screenshot system plugin in `wox.core` will no longer reopen the PNG and write it to the clipboard.

After the change, Go will:

- start the screenshot session
- pass the preallocated export path
- receive the result
- notify success or failure
- surface clipboard warnings when present

User-visible notification behavior:

- `completed` with successful clipboard write: screenshot success notification
- `completed` with clipboard warning: screenshot success notification plus an extra warning that the screenshot was saved but clipboard copy failed
- `failed`: screenshot failure notification

This preserves the screenshot artifact even when clipboard export is partially degraded.

## Manual Verification Scope

Manual verification is sufficient for this change. No automated tests are part of the planned work.

Manual verification should confirm:

- the screenshot PNG is written under `woxDataDirectory/screenshots`
- successful captures still produce the expected success notification
- clipboard write failures still leave a valid `screenshotPath`
- clipboard write failures surface a warning instead of converting the screenshot session into a full failure
- Go no longer performs screenshot clipboard writes or PNG decode work for this flow

## Files Expected To Change

- `wox.core/common/ui.go`
- `wox.core/plugin/system/screenshot.go`
- `wox.ui.flutter/wox/lib/entity/screenshot_session.dart`
- `wox.ui.flutter/wox/lib/controllers/wox_screenshot_controller.dart`
- `wox.ui.flutter/wox/lib/utils/screenshot/screenshot_platform_bridge.dart`
- platform runner files for screenshot clipboard export on macOS, Windows, and Linux

## Risks And Constraints

- Platform runners now need to provide screenshot clipboard export parity, so the change is only complete once all supported screenshot platforms handle the new bridge method.
- A warning-based partial success must remain explicit in the contract so UI and Go do not silently treat clipboard failure as full success.
- Export path allocation must remain deterministic and writable before Flutter begins the export.

## Result

After this change, the screenshot pipeline will remain:

- Go orchestrates the session and allocates the export path.
- Flutter renders the final screenshot and writes the PNG.
- Flutter immediately asks the native runner to write the exported image to the clipboard.
- Go only consumes the result and presents notifications.

That removes the current Go-side screenshot clipboard handoff without widening the scope into a general clipboard subsystem rewrite.
