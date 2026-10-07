# Native window backend

This package owns the event loop, native window lifetime, focus and visibility
transitions, display-coordinate conversion, rendering pipeline and native child
surfaces. Window methods use the owning GUI thread for native work. Native
renderers and embedded surfaces share the window's close and resource-release
sequence, so their adapters and tests remain together here.

The runtime facade aliases public types and forwards free functions. Input
events/editing, immutable images, capture resources and WebView lifecycle are
owned by sibling packages. Local `api_*` files adapt those contracts without
importing the facade or adding a second implementation.

Native headers are owned here with their translation units and platform
fixtures. Shared platform declarations can be used by screenshot native adapters;
they never include screenshot headers or retain screenshot sessions. Clipboard
and external-link methods only supply native owner handles and thread dispatch
to their utility packages.
