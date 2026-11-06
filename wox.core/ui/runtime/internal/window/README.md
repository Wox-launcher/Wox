# Native window backend

Drawing-transform work is in progress. Internal display commands, the software
reference renderer and the Windows/Linux encoders support affine geometry and exact convex clipping; scene/DPI
conversion composes onto the matrix without also scaling local text/image geometry.
The reference path covers rotated clips, opacity groups, local-direction edge fades,
retained damage, and singular/nonfinite maps. Transformed scenes conservatively
disable caret backdrop patches. `DisplayList.PushTransform` / `PopTransform`
compose local logical maps inside their parents. Clip rectangles are captured
in target coordinates at push time and intersect as exact convex polygons;
later transforms cannot move an established clip. `ClipRect` returns a conservative
envelope in current drawing coordinates for text culling. Damage and material
metadata use target envelopes; scene/DPI conversion also maps material bounds.
The recording API, `widget.Transform`, and WoxView style mapping are implemented;
macOS native compilation and execution remain an outstanding verification gate.

`BeginSnapshotScope` anchors portable paint captures in logical coordinates and
separates live outer groups from frozen internal groups. `BeginSnapshot` /
`EndSnapshot` retain a balanced subtree's commands, clips, ancestor opacity/fades,
and immutable images. Recording culls are suspended during capture so a partial
frame cannot overwrite a complete snapshot with missing paint. Native damage
scissors remain active. Captures exclude native embedded surfaces and freeze
the visible caret phase; they retain no widget state or input callbacks.
`DrawSnapshot` rebases the commands into the current scope and drawing transform,
intersects frozen clips with the current parent clip, restores clip state, and
re-registers backdrop-material damage metadata. Backdrop materials retain their
recorded style and sample the replay backdrop through the ordinary renderer.
`DrawSnapshots` replays ordered fragments under their shared captured ancestor
groups once; distinct sibling groups retain separate identities even when their
opacity and bounds match. `BeginSnapshotRegion` retains only a clipped portion
of shared paint without changing live rendering. Its boundary uses the renderer's
normal clip antialiasing, so callers should omit it when retaining all that paint.
Snapshot storage and lifetime belong to the caller; `RetainedBytes` supplies a
conservative command/image estimate for caller budgets. Shared renderers keep
their existing image/resource cache ownership.

Windows image edges snap only when axis aligned, in physical target coordinates.
Rotated/skewed geometry stays unsnapped. This preserves equivalent image draws
when geometry is rebased into a snapshot or moved into a drawing transform.

Windows applies each command's logical matrix before the display scale. Convex
clip paths remain in target logical coordinates and have no small vertex limit.
Floating materials sample the transformed physical envelope and replace pixels
through the exact rounded mask. Sampling temporarily flushes and restores the
current clip so it includes earlier clipped content.

Groups containing backdrop materials use readable bitmap targets; ordinary
groups continue to use native D2D layers. Readable groups retain the background
and interpolate premultiplied before/after pixels, including alpha, through the
opacity/fade mask. This avoids both missing pending layer content and applying
the backdrop alpha twice. Their targets are reusable after EndDraw, with a
16 MiB retained pool released on resize, trim, cache clearing and destruction.
The pool is included in renderer resident-byte accounting.

`transform_native_test.go` and its Windows adapter replay the production Go encoder and frame lifecycle
on an offscreen WARP bitmap without a window. It covers rotated clips, group
alpha, edge fades, actual text/image drawing, material replacement and nested
backdrops, retained damage, display-scale changes, invalid-frame recovery,
singular matrices, clip replacement, and bounded resource reuse/clearing.

The macOS source now keeps each command's matrix in a separate top graphics state.
Clip and composite scopes stay in target logical coordinates, so restoring them
cannot restore a stale child's matrix. Singular maps skip drawing without
concatenating a singular CoreGraphics CTM. Exact clip paths survive overlay
handoff, and frame teardown unwinds clips, transforms and unfinished groups.
Floating-material sampling uses the transformed physical envelope. Groups that
contain materials draw into the readable bitmap context and interpolate captured
before/after premultiplied pixels over that envelope; ordinary groups retain
CoreGraphics transparency layers. Snapshot storage is released at group exit.

The macOS bitmap adapter reuses `encodeDarwinCommands` with the same shared
transform/backdrop pixel cases as Windows. This implementation still requires
compilation and execution on macOS; Windows/WSL test results do not validate its
Objective-C/CoreGraphics path. Scheduled native-backend tests now include macOS
and Windows, and PR compilation includes the backend test package.

Linux applies command matrices in its shape and texture vertex shaders. Target-space
convex clips and transformed composite bounds contribute inward half-planes in a GL
texture buffer, with scissor retaining the coarse damage bound. The GL buffer limit
is checked explicitly; the polygon-fill primitive's small vertex limit does not apply
to clips. Group entry retains its own matrix so child paints cannot alter its bounds
or fade direction. Material snapshots cover the transformed physical envelope and
sample in target axes through the exact local rounded mask. Clip resources and group
snapshots are included in resident-byte accounting and released with renderer caches.

The Linux adapter runs the shared transform/backdrop suite through `encodeLinuxCommands`
on an unmapped GTK/GL target. It releases the current context between calls so test
subtests can change OS threads. Only this fixture enables blur on CPU GL rasterizers;
production retains its software-renderer fallback policy. Run with
`GDK_BACKEND=x11 xvfb-run -a go test ./ui/runtime/internal/window` from `wox.core`.
This verifies native GL pixels without launching Wox or exercising desktop policies.

Display-list opacity isolates a complete painted group. Windows uses a D2D
layer, macOS uses a CoreGraphics transparency layer, and Linux interpolates
premultiplied before/after framebuffer snapshots using the shared composite
shader. Group boundaries restore enclosing clips; opacity is dimensionless and
does not scale with scene coordinates. Composite layers disable standalone caret
backdrop patches because the caret belongs to the completed group.

`opacity_test.go` checks reference pixels and damage-scoped replay. Native
`opacity_*_test.go` fixtures check opaque overlap, nested groups, glass alpha,
and following drawing at 1x/1.25x/1.5x/2x. Windows uses offscreen WARP and macOS
uses a bitmap context; neither opens a window. Linux uses an unmapped GTK/GL
fixture and skips when no display is available.

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

On macOS, embedded HTML hover uses the native callback's event, not the global
`NSApp.currentEvent`, which can differ for synthesized tracking callbacks.
WebKit suppresses buttonless DOM motion in non-key windows. The adapter sends
motion through its drag responder without sending a press or activating the
window; the DOM pressed-button mask still comes from the actual input state.
Leaving uses an outside point in WebView coordinates and the same responder,
so WebKit tracking-area ownership cannot keep CSS hover stuck. Plugin-window
native smoke coverage checks this nonactivating interaction contract.
