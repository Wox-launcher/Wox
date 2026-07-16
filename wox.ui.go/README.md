# Wox Go UI

`wox.ui.go` is a standalone cross-platform UI process for Wox. It owns the native Windows, AppKit, or GTK event loop and connects to `wox.core` through the same HTTP/WebSocket process boundary used by the Flutter UI.

Build the UI into Wox's embedded resource tree:

```sh
make -C wox.ui.go build
```

Set `WOX_UI_IMPLEMENTATION=go` when starting `wox.core` to select this UI without changing the default Flutter path during migration. For development, `go run ./cmd/wox-ui` connects to core's default port `34987`; production startup passes the same three arguments as Flutter: server port, core PID, and development flag.

## Architecture contract

The portable Go layer owns widget layout, focus routing, text editing state, scrolling, Wox protocol DTOs, query behavior, previews, actions, and settings pages. Platform files are deliberately thin and own only the native window/event loop, GPU command submission, font measurement, clipboard, file dialogs, external browser dispatch, and IME integration:

- Windows: Win32 + Direct2D/DirectWrite.
- macOS: AppKit + Metal/CoreText.
- Linux: GTK3 + GtkGLArea/OpenGL/Pango, with layer-shell and WebKitGTK used when available.

Display-list and widget changes must compile unchanged on all three platforms. A platform-specific feature should first expose a small capability on `Window`; business widgets must not import Win32, AppKit, GTK, or renderer APIs.

## Current migration surface

The Go process currently supports the Wox query protocol, list and grid results, refinements, completion hints, toolbar messages, actions and action forms, live Glance items and actions, GPU raster/SVG images, text/image/file/list/structured previews, system WebView previews, streaming terminal output with cursor-based history loading and local find, media controls, and a native-GPU chat surface with multiline IME input, streamed snapshots, stop, history, model and skill catalogs, message copy/edit/retry actions, a copyable debug trace, tool-call cards, and `ask_user` answers. Query rows render multiple plugin and development tails, including the Go UI receive-time marker backed by core's shared query timestamp. WebView previews share one portable URL/HTML/CSS/cache contract and use WebView2, WKWebView, or runtime-detected WebKitGTK without exposing those engines to query widgets. Query-owned requirement settings and trigger-keyword conflict editing reuse the same portable form engine. Theme color editing is one shared live-preview component mounted by both query previews and the settings route. The core-backed settings window includes general, appearance, hotkeys, network, live runtime-host diagnostics, theme, updates, privacy with a copyable telemetry sample, development diagnostics, plugin management, AI provider/MCP/skill management, local data and backup management, Cloud Sync, a real local usage dashboard, and an About page backed by the running core version; text settings reuse the common IME editor, executable paths use the cross-platform native file picker, and searchable choice overlays handle system font and Glance catalogs. The flat Go settings rail is scrollable and keeps keyboard-selected pages visible. Runtime diagnostics expose version, executable, loaded-plugin, install/upgrade, and host-restart state through the existing core protocol. Cloud Sync covers login, registration and legal consent, email verification, password reset/change, encrypted bootstrap/restore, live progress, enable/disable/manual sync, subscription links, device management, and plugin exclusions without platform-specific page code. The configured application font is applied by DirectWrite, CoreText, or Pango through one window-level API. Hotkey recording continues to use core's native cross-platform recorder, while the Go layer provides shared query/tray editors, emoji or structured image values, and a core-backed ignored-application picker. Plugin and theme pages share installed/store catalogs with install, upgrade, apply, enable, disable, and uninstall actions as appropriate, while the theme page reuses the live GPU preview editor. AI settings reuse the JSON table editor, load provider choices from core, validate transport-specific fields, and support both local skill directories and remote repository cloning. The Data page uses core-owned cross-platform routes for storage migration, backup/restore, shell opening, and logs, while the Go layer owns responsive progress and confirmation state. The plugin editor consumes the same translated, platform-filtered `SettingDefinitions` DTO as Flutter. Shared form components include JSON table add/edit/delete flows with cross-platform directory picking; unsupported specialized table columns are preserved without mutation.

Flutter remains the default while the remaining specialized plugin table controls, provider and MCP health indicators, downloadable model managers, and remaining management pages are migrated. Windows packages must place `WebView2Loader.dll` beside `wox-ui.exe` (the Wox-local Makefile copies the existing Flutter artifact during migration, and standalone builds can override `WEBVIEW2_LOADER`). Linux builds do not require WebKitGTK headers, but WebView previews need a WebKitGTK 4.1 or 4.0 runtime installed.
