# WebView runtime

This package owns the platform-neutral contract and lifecycle for WebView previews embedded in a Wox window.

## Architecture

```text
launcher preview and widget Host
              |
              v
       runtime.Window facade
              |
              v
 platformWindow / UI-thread bridge
              |
              v
 internal/webview.Controller
              |
              v
      platform Driver adapter
              |
              v
 WebView2 / WKWebView / WebKitGTK
```

The layers have distinct responsibilities:

- `runtime.Window` keeps the public WebView API and converts public runtime types to this package's portable types.
- `Controller` validates content, tracks visibility, forwards navigation and pointer operations, and owns reset and close transitions.
- `Driver` is the narrow contract implemented by the Windows, macOS, and Linux host adapters.
- Platform host adapters remain in the parent `runtime` package because they must access its private native window, renderer, UI-thread command queue, and window callbacks. Moving them into this package would reverse the dependency and create an import cycle.
- Native WebView engines remain owned by their platform window trees. The controller coordinates their lifecycle but does not own launcher layout or focus policy.

## Ownership boundaries

The widget Host remains responsible for layout, hit testing, surface-local pointer coordinates, focus transfer, and shortcut routing. It passes only validated WebView bounds and already-translated pointer events through the `runtime.Window` facade.

When a page does not consume Escape, the platform adapter first returns native keyboard focus to the host view and then forwards Escape to the WebView focus owner. A visible query box becomes the next focus target; a preview with no visible query box applies the launcher's outer Escape behavior and closes or hides that preview window.

The injected page listener treats a change in `document.activeElement` or a DOM mutation during Escape dispatch as page-owned handling. `preventDefault()` and `stopPropagation()` alone do not claim the key because framework-level event routers may apply them even when no visible page state handles Escape.

This package must not import launcher, widget, or the parent runtime package. Platform-native types must not appear in its public contract. These constraints keep the lifecycle logic independently testable and prevent WebView implementation details from leaking into UI composition.

## Lifecycle

One controller represents the WebView capability of one native window:

1. `Show` validates the content and bounds, asks the driver to attach or update the native surface, and marks it visible only after the driver succeeds.
2. `Hide` removes the surface from the visible composition and focus domain without discarding cached browser state.
3. `Reset` destroys active and cached browser state while keeping the controller reusable.
4. `Close` permanently releases the driver when the owning native window is destroyed.

All native operations must continue to follow the owning platform window's UI-thread rules.
