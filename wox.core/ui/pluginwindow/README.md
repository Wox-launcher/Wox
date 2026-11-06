# Plugin windows

`API.CreateWindow(ctx, CreateWindowOption)` returns a hidden `PluginWindow` instance.
`Show`, `Hide`, `Close`, `SetOptions`, `Content.Set`, `Content.Update`, and `Content.Command` each take
context plus a named option and return a named result containing `Success`/`ErrMsg`.
The Python names use snake case; Node.js and Go use PascalCase. New options can be
added without changing method signatures. There is no public script execution API.

## Content and presentation

`WindowContent{Type, Data}` contains a serialized WoxView JSON tree. Creation,
`Content.Set`, and `Content.Update` validate that tree before changing the window.
`Type` must be `woxview`; other values are rejected. Use WoxView `text` and `markdown` nodes
for those formats; raw text, Markdown, HTML and remote URLs are not window content.

The initial `Presentation` is `overlay`. Defaults: hidden, 360 × 160 logical units,
bottom-center on the display selected by Wox's window-position setting, offset `(0, -80)`, topmost, closable,
not focusable, not movable and not resizable. Wox supplies a 30-unit close/drag
strip when needed. Set `Closable: false` for a chrome-free indicator. Geometry
uses the shared overlay work-area and DPI conversions, including negative desktop
origins. Bounds are clamped to the work area.

Set `Presentation: "window"` for an independent application window that participates
in native window discovery and task switching. This presentation defaults to
focusable and not topmost; explicit `Focusable` and `Topmost` values still win.
Other defaults are unchanged. The WoxView showcase and note examples use this mode.

`Show({})` does not steal focus. Set `Focusable: true` at creation for keyboard
interaction; `Show({Activate: true})` then explicitly activates the window.
Set `HideChrome: true` to hide the Wox-drawn control strip. This does not change
whether the window can be closed. Set `Movable: true` to enable the drag strip.
Both options default to false.
Set `Transparent: true` to remove the window background and native backdrop so
WoxView can draw an arbitrary shape over the desktop. The default is false.
Transparency changes painting only: the native window still receives pointer
events within its rectangular bounds; transparent pixels are not click-through.
On Windows, `PassThroughTransparent: true` also passes pointer input through
fully transparent pixels. It requires `Transparent: true`; other platforms
currently reject this option. The input region follows the painted content.
WoxView's `inputRegion: true` also includes that node's bounds, even when its
background is transparent; this supports intentionally interactive empty space.
The Windows input mask supports up to 4,194,304 physical pixels; larger frames
cannot update the region and log a warning. A fully empty frame retains one
corner pixel so Windows can repaint an entrance animation.
Use WoxView transitions or entry/exit animations on the content, and delay `Hide`
until the exit completes. Showing a hidden window does not remount its document;
remount the animated node or update its state to replay the entry animation.
Set `DragContent: true` together with `Movable: true` to drag from non-interactive
WoxView content, including ordinary text. The gesture starts after pointer movement;
interactive descendants retain their own clicks, selection, scrolling and drag
gestures. It works with or without the control strip and defaults to false.
`SetOptions` patches size, anchor, offsets and topmost status; omitted fields stay
unchanged, including explicit false/zero values. Size-only updates preserve the
user-dragged position. Presentation, focusability, movability, resizability and
closability are creation options. Other options can be added in future releases.

`Hide` keeps the WoxView document and its state. `Content.Set` replaces the document
and discards pending callbacks and commands from the previous document. `Close` is idempotent.
Closed windows cannot be reopened. Plugin unload, disable, offline suspension and
host loss close the plugin's windows and release its callbacks.

## Native declarative content

See the [WoxView protocol](../woxview/README.md) and its generated JSON Schema for
components, form values, revision resets and event arguments. `Content.Update`
(`content.update` in Python) takes `{Data, DocumentID?, Sequence?}` and replaces a
validated tree while keeping its document and keyed state. `Content.Set` starts
a fresh document. Invalid updates leave the previous tree intact. The optional
monotonic `Sequence` rejects out-of-order updates; a nonzero sequence also prevents
later unsequenced updates in that document.

The `woxview_event` callback carries `{Handler, Args, Key}` in `Data`, alongside the
window and document IDs. Core filters queued handlers against the current tree.
Node and Python SDKs expose the same content, events, and update operations.

`Content.Command` (`content.command` in Python) takes
`{DocumentID?, Command: "focus" | "scrollTo" | "selectAll", Key}` and returns a
named result. Keys must be unambiguous across the document. Commands execute in
order after layout, materializing offscreen lazy rows as needed; success means
accepted. Deferred failures emit a document-scoped `error` with `{Command, Key}`
in `Data`. Document replacement and close discard pending commands. Focus and
selection require a focusable window but never activate it. See the WoxView
protocol's command section for target, queue, and modal behavior.

Text-field `changeDebounce` delays only change notifications. Local values and
form snapshots remain immediate; submission flushes pending edits first.

WoxView display props prefixed with `i18n:` use the creating plugin's translation
dictionary and existing Wox fallback. Translation is a Core-owned capability
passed through the internal factory, not a new plugin callback or wire option.
Field values, names, handler IDs and event arguments remain unchanged. Core callers can inspect WoxView
validation errors with `errors.As`; public window results retain their existing
`ErrMsg` text.

## Events

`OnEvent(ctx, event)` receives `ready`, `woxview_event`, `theme_changed`, `error`,
and `closed`. Events include `WindowID` and, for document events, `DocumentID`.
Callbacks run serially per window, outside the UI thread, and may call window
methods themselves. Install `OnEvent` at creation: `ready` can arrive before the
creation RPC reply, so callbacks should not assume a local result is assigned.
Content is limited to 4 MiB. The event inbox is bounded at 128; a slow plugin
callback may lose excess events, reported through an overflow error event.

## Node.js

```ts
const data = JSON.stringify({
  version: 1,
  root: { type: "text", props: { text: "Hello from WoxView" } }
})
const created = await api.CreateWindow(ctx, {
  Window: { Width: 360, Height: 160, Focusable: true },
  Content: { Type: "woxview", Data: data }
})
if (!created.Success || !created.Window) throw new Error(created.ErrMsg)
const window = created.Window
await window.Show(ctx, {})
await window.SetOptions(ctx, { Topmost: false, OffsetY: 0 })
// When the workflow finishes:
await window.Close(ctx, {})
```

## Python

```python
import json
from wox_plugin import (
    CreateWindowOption, WindowContent, WindowContentType, WindowOptions,
    WindowShowOption, WindowCloseOption,
)

data = json.dumps({
    "version": 1,
    "root": {"type": "text", "props": {"text": "Hello from WoxView"}},
})
created = await api.create_window(ctx, CreateWindowOption(
    content=WindowContent(WindowContentType.WOXVIEW, data),
    window=WindowOptions(width=360, height=160, focusable=True),
))
if not created.success or created.window is None:
    raise RuntimeError(created.err_msg)
window = created.window
await window.show(ctx, WindowShowOption())
await window.close(ctx, WindowCloseOption())
```

Use the SDK's `createWoxView(...).asWindow()` or
`create_wox_view(...).as_window()` for callback registration and state-driven
updates. See the [WoxView SDK guide](../woxview/README.md).

## Ownership and verification

`plugin/window.go` owns plugin-scoped capabilities and callback lifetime;
`ui/pluginwindow` owns native presentation, document identities and event adaptation;
`woxview` validates and renders trees; `runtime` provides generic window capabilities
without plugin imports.
The hosts translate instance calls to private RPC operations and route events by
callback ID within the calling plugin. A window ID from another plugin never grants
access. Query-preview APIs are independent of this window contract.

Run `go test ./plugin ./plugin/host ./ui/pluginwindow ./ui/woxview` in Core,
`pnpm test` in the Node SDK and host, and `uv run python -m unittest discover -s tests`
in the Python SDK and host. Run `pnpm exec tsc --noEmit` in the Node host too.
On Windows, `WOX_WINDOW_NATIVE_TEST=1 go test ./ui/pluginwindow -run
TestNativeWoxView -count=1` exercises a real native window, WoxView interaction,
updates, and document isolation.

WoxView event `Key` is the triggering node's explicit key, or an empty string for
an unkeyed node. Handler registration and document checks still apply.

`theme_changed` is a document-scoped window event with `Data: {"Dark": bool}`.
It follows Wox theme notifications, including palette changes that keep the same
light/dark appearance, and is delivered to hidden windows too. `Dark` describes
the window's overlay surface rather than the OS appearance or launcher background.
No initial event is emitted on creation; theme color APIs can provide initial
appearance. Token and dual-color rendering updates locally without a plugin reply.
Plugins can use the event to replace theme-specific image assets through WoxView updates.

Replacing content discards the old document's queued theme events; closing
or unloading cancels its theme subscription. Pending theme changes can coalesce
within the same document and discrete-event ordering segment. Node exports
`WindowThemeChangedData`; Python exports the matching `TypedDict` and
`WindowEventType.THEME_CHANGED`. Both hosts preserve the opaque PascalCase `Dark`
payload, including false.

`WOX_WINDOW_THEME_NATIVE_TEST=1 go test ./ui/pluginwindow -run
TestNativeHiddenWindowThemeChanges -count=1` runs a Windows integration fixture
that creates only a hidden window and checks theme notification, document guards,
and subscription cleanup without showing or activating Wox.

The UI marks value-change events with an internal opaque coalescing key. The plugin
queue combines pending changes within the same document and between discrete-event
ordering barriers, retaining the latest value even when the bounded queue is full.
The plugin package does not parse WoxView payloads or depend on UI components to
make this decision. Nonmergeable overflow remains observable through error events.
