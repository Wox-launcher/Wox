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

Linux UI-call diagnostics track the current native stage, continuous synchronous
call backlog, and completed calls per watchdog sample. Stage changes alone do not
prove calls are being serviced: GTK rendering can keep idle-priority callbacks
waiting. A persistent backlog with no completed calls and fresh stages points to
dispatch starvation; an old stage helps locate a native stall. These are diagnostic
signals, not a replacement for a native stack when identifying the cause.

Run `python3 testdata/test_linux_startup.py` on Linux with GLib development files
to exercise native startup, deferred dispatch, and backlog accounting without a
display. The scheduled UI workflow runs this fixture and the smoke runner's Unix
socket relaunch regression alongside the UI contracts.
