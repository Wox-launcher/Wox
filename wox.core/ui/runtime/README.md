# Runtime layout

The top-level package is the stable Go UI entry point. It contains type aliases
and small forwarding functions grouped by capability. Importers continue to use
`wox/ui/runtime`; implementation packages do not import that facade.

| Directory | Responsibility |
| --- | --- |
| `internal/input` | Portable key, pointer and IME events, hotkey adaptation, text editing, Unicode graphemes and undo history |
| `internal/graphics` | Logical geometry and physical surface sizes, immutable image buffers, decoding, animation and native packed-pixel conversion |
| `internal/capture` | Windows desktop/window capture, GDI/DXGI/WinRT resources, capture diagnostics, native bridges and capture fixtures |
| `internal/window` | Native event loops, window lifecycle, focus, DPI conversion, render submission, native surfaces, accessibility providers and GUI-thread adapters |
| `internal/webview` | Portable embedded-browser contract and lifecycle, document cache policy and browser-page actions |

Dependencies point from the facade to implementations, from `window` to the
other capability packages, and from `input` and `capture` to `graphics`.
`graphics` and `webview` have no window dependency. Platform adapters use utility
packages for hotkey syntax, clipboard publication, external links and reusable
desktop-window capabilities.

Native window rendering and surface adapters stay with the window backend because
they share its owning GUI thread, renderer, native handle and close transitions.
The display list and software reference renderer stay with that rendering
contract. They must not depend on launcher widgets or screenshot selection state.
Platform suffixes select implementations inside the owning package; platforms do
not become competing feature owners.

Move tests, fixtures and native translation units with their implementation.
The capture package has no Wox window state or native window-header dependency.
Its hardware experiments remain opt-in; pixel, crop and DPI-independent tests
run without desktop capture. Screenshot selection remains in `ui/screenshot`
and consumes generic runtime capabilities. Its native adapters use the shared
platform headers under `internal/window`, which do not include screenshot code.

Pure input and graphics tests can run independently. Run window/backend tests
on their native platform, then build the core to validate the facade and all
consumers.
