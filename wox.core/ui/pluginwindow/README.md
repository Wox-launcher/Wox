# Plugin windows

`API.CreateWindow(ctx, CreateWindowOption)` returns a hidden `PluginWindow` instance.
`Show`, `Hide`, `Close`, `SetOptions`, `Content.Set`, and `Content.PostMessage` each take
context plus a named option and return a named result containing `Success`/`ErrMsg`.
The Python names use snake case; Node.js and Go use PascalCase. New options can be
added without changing method signatures. There is no public script execution API.

## Content and presentation

`WindowContent{Type, Data}` supports `text`, `markdown`, and inline `html` (raw HTML,
not a serialized Preview object). Text/Markdown reuse the Preview widgets. HTML and
query previews share `previewcontent.WebViewSurface` and the same native WebView
controller; plugin windows own their documents and lifetime independently.

The initial `Presentation` is `overlay`. Defaults: hidden, 360 × 160 logical units,
bottom-center on the display selected by Wox's window-position setting, offset `(0, -80)`, topmost, closable,
not focusable, not movable and not resizable. Wox supplies a 30-unit close/drag
strip when needed. Set `Closable: false` for a chrome-free indicator. Geometry
uses the shared overlay work-area and DPI conversions, including negative desktop
origins. Bounds are clamped to the work area.

HTML pages control their own background. On Windows and macOS, a transparent `html` and
`body` allows the window's themed wash and native material to show through the
WebView. An opaque page background covers that material. The HTML author is
responsible for choosing readable text colors; the plugin theme color API returns
launcher colors, which may differ from a custom overlay palette.

`Show({})` does not steal focus. Set `Focusable: true` at creation for keyboard
interaction; `Show({Activate: true})` then explicitly activates the window.
For HTML, a show request made before the document is ready is accepted immediately
and presentation is deferred until the current document sends its ready event. Hide
cancels a pending show. Loading failures are reported through the window error event;
the ready event indicates DOM readiness, not completion of arbitrary asynchronous page work.
Set `HideChrome: true` to hide the Wox-drawn control strip; the page can provide
its own controls. This does not change whether the window can be closed.
Set both `Movable: true` and `DragContent: true` to drag from HTML content.
Primary-button presses become clicks if released without dragging; a drag moves
the window instead of selecting page content. Other pointer events are forwarded.
Both options default to false.
`SetOptions` patches size, anchor, offsets and topmost status; omitted fields stay
unchanged, including explicit false/zero values. Size-only updates preserve the
user-dragged position. Presentation, focusability, movability, resizability and
closability are creation options. Other options can be added in future releases.

`Hide` keeps the HTML DOM and its state. `Content.Set` replaces the document and
discards pending messages from the previous document. `Close` is idempotent.
Closed windows cannot be reopened. Plugin unload, disable, offline suspension and
host loss close the plugin's windows and release its callbacks.

## HTML messages

The bridge is available before the page's scripts:

```js
const unsubscribe = woxWindow.onMessage(state => {
  document.querySelector("#remaining").textContent = state.remaining
})
document.querySelector("#cancel").onclick = () => {
  woxWindow.postMessage({ action: "cancel" })
}
```

Plugin → page: `window.Content.PostMessage(ctx, {Data: jsonValue, DocumentID?})`.
Page → plugin: creation's `OnEvent(ctx, event)` receives `Type: "message"` and
`Data`. Other events are `ready`, `error`, and `closed`. Events include `WindowID`
and, for document events, `DocumentID`. The optional message `DocumentID` rejects
a delayed update intended for a replaced document; omitting it targets the current
document. Callbacks run serially per window, outside the UI thread, and may call
window methods themselves. Install `OnEvent` at creation: ready can arrive before
the creation RPC reply, so callbacks should not assume a local result is assigned.

Success means accepted, not acknowledged by the page. Up to 64 messages queue
before ready; message payloads are limited to 256 KiB and content to 4 MiB.
Page-side messages include bridge-envelope overhead in the limit. Use JSON values,
not functions, cyclic objects, `undefined`, or binary objects. `Content.Set` is for
document replacement; use messages to update live DOM without reloading.
Listener exceptions and a 15-second HTML readiness timeout emit `error` events.
The event inbox is bounded at 128; a slow plugin callback may lose excess events,
with a Core warning. This API is for UI interaction, not durable transport.

The native bridge accepts the active WebView and the current document token only.
The page bridge is installed only in the top frame; it exposes no plugin API or
native window handle. As with ordinary web pages, same-origin children can access
their parent's JavaScript and should be treated as equally trusted content.
Old documents and stale queued events are discarded. Remote URL content is not
supported. Anchor links open in the system browser; navigating the page directly
does not install a bridge into the destination document.

## Node.js

```ts
const created = await api.CreateWindow(ctx, {
  Window: { Width: 360, Height: 160, Focusable: true },
  Content: { Type: "html", Data: html },
  OnEvent: async (eventCtx, event) => {
    if (event.Type === "message") {
      // Handle cancel/pause commands from the page here.
      await api.Log(eventCtx, "Info", JSON.stringify(event.Data))
    }
  }
})
if (!created.Success || !created.Window) throw new Error(created.ErrMsg)
const window = created.Window
await window.Show(ctx, {})
await window.Content.PostMessage(ctx, { Data: { remaining: 60 } })
await window.SetOptions(ctx, { Topmost: false, OffsetY: 0 })
// When the workflow finishes:
await window.Close(ctx, {})
```

## Python

```python
from wox_plugin import (
    CreateWindowOption, WindowContent, WindowContentType, WindowOptions,
    WindowShowOption, WindowMessageOption, WindowCloseOption, LogLevel,
)

async def on_event(event_ctx, event):
    if event.type == "message":
        await api.log(event_ctx, LogLevel.INFO, str(event.data))

created = await api.create_window(ctx, CreateWindowOption(
    content=WindowContent(WindowContentType.HTML, html),
    window=WindowOptions(width=360, height=160, focusable=True),
    on_event=on_event,
))
if not created.success or created.window is None:
    raise RuntimeError(created.err_msg)
window = created.window
await window.show(ctx, WindowShowOption())
await window.content.post_message(ctx, WindowMessageOption(data={"remaining": 60}))
await window.close(ctx, WindowCloseOption())
```

See `examples/timer.html` and `examples/dictation.html` for self-contained content
to load into either example. These demonstrate presentation and messages only;
microphone access and speech recognition remain plugin responsibilities.

## Ownership and verification

`plugin/window.go` owns plugin-scoped capabilities and callback lifetime;
`ui/pluginwindow` owns native presentation, document identities, queues and bridge;
`runtime` provides generic window and WebView callbacks without plugin imports.
The hosts translate instance calls to private RPC operations and route events by
callback ID within the calling plugin. A window ID from another plugin never grants
access. Existing Preview and plugin APIs retain their signatures.

Run `go test ./plugin ./plugin/host ./ui/pluginwindow ./ui/previewcontent` in Core,
`npm test` in the Node host, and `python -m unittest discover -s tests` in the Python
host. On Windows, `WOX_WINDOW_NATIVE_TEST=1 go test ./ui/pluginwindow -run
TestNativeWindowMessages -count=1` exercises real WebView2; set
`WOX_WEBVIEW2_LOADER_PATH` to `resource/others/webview/WebView2Loader.dll` if the test
binary is in a temporary directory.

On macOS, `make smoke pluginwindow/001` checks a draggable HTML window's WebKit
snapshot transparency, prompt pointer enter and leave (including repeated entry and
leaving after a click), and the page's close-button click. The fixture keeps the
window non-key and rejects unintended pressed buttons, selection, or dragging.
