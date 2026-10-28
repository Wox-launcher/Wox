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

Linux synchronous UI calls and deferred posts use GLib's normal event priority,
ahead of GTK redraw sources, so continuous rendering cannot starve them. Posts
still attach a source rather than executing inline on the owning thread.

Linux desktop policies are selected in `window_linux_desktop.go`, using the actual
GTK display backend rather than assuming a Wayland session implies a Wayland
window. COSMIC, KDE, GNOME, Hyprland, wlroots and X11 policies live in their own
`window_linux_<desktop>.go` files. COSMIC keeps its shared utility session detector;
the other layer-shell policies preserve the legacy first-nonempty desktop-variable
order and aliases such as `plasmawayland`. GTK still probes protocol support and
excludes application windows before enabling layer-shell.

`native_linux.c` owns shared GTK lifecycle, rendering and layer-shell protocol
operations. It calls the narrow `native_linux_desktop.h` capability interface for
desktop window operations. `native_linux_x11.c` owns X11 activation, taskbar hints,
resize synchronization and compositor queries. Its focus history belongs to the
GTK object and is cleared on blur, hide and destruction. The adapter borrows a GTK
widget, never the renderer's `WoxLinuxWindow` state. X11 root-image capture remains
a generic capture primitive in the shared backend, separate from window policy.

Linux UI-call diagnostics track the current native stage, continuous synchronous
call backlog, and completed calls per watchdog sample. Stage changes alone do not
prove calls are being serviced: GTK rendering can keep idle-priority callbacks
waiting. A persistent backlog with no completed calls and fresh stages points to
dispatch starvation; an old stage helps locate a native stall. These are diagnostic
signals, not a replacement for a native stack when identifying the cause.

Run `python3 testdata/test_linux_startup.py` on Linux with GLib development files
to exercise native startup, deferred dispatch, backlog accounting, and both call
and post dispatch during continuous redraw without a display. The scheduled UI
workflow runs this fixture and the smoke runner's Unix socket relaunch regression
alongside the UI contracts.

Run `go test ./ui/runtime/internal/window` from `wox.core` for desktop policy tests.
Run `python3 testdata/test_linux_desktop.py` from this directory with GTK 3/X11
development files, `xvfb-run` and `xauth` installed for native adapter regression
tests. They exercise real EWMH messages, focus restoration after foreign windows
disappear, per-window state, remapping, native resize and object release at GTK
scales 1 and 2. They do not emulate a full Wayland compositor or window manager.

Run `python3 testdata/test_linux_pointer_passthrough.py` from this directory in a
live Hyprland session to verify input-region changes on the actual native backend.
It builds a toolbar and border window, verifies that releasing exclusive keyboard
focus lets the border receive input, then checks repeated restoration and removal
of pointer input without application redraws.
It briefly moves the pointer without clicking and restores its position afterward.
