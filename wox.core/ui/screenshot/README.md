# Screenshot ownership

This package owns screenshot capture workflows, the portable editor, object selection, export, and recording orchestration. Screenshot-specific native code belongs here too:

- `object_selection.go` owns portable selection paths, cancellation, foreground/refinement scheduling, and preview transitions.
- `object_selection_darwin.*`, `object_selection_windows.*`, and `object_selection_linux*` implement bounded native geometry queries. AX/UIA/AT-SPI references stay inside the query lifetime; the editor receives rectangle values.
- `selection_darwin.m` owns the macOS capture snapshot, selection session, inspector, selection layers, click/drag handling, and scrolling border. `selection_darwin.h` is its package-local C bridge; `platform_darwin.go` owns the Go handoff.
- `platform_*.go` maps capture and native coordinates into the editor. Linux desktop-specific capture behavior stays in the corresponding environment file.

Dependencies point from `screenshot` to `runtime`. Runtime supplies general window/event-loop, cursor, coordinate, and image-capture capabilities; it does not own selection policy or call screenshot implementations. `runtime/native_darwin_platform.h` exposes the shared overlay panel and platform primitives to native feature code without depending on screenshot headers or session types. Selector tests and native provider fixtures live in this package; shared overlay cursor tests stay in runtime.

## Coordinate contract

macOS native selection uses global top-left logical points and maps captured pixels using the actual image dimensions. Windows selection uses physical desktop pixels relative to the captured virtual desktop. Linux selection uses the capture's logical bounds, with native pixel geometry converted at the platform boundary. Selection chrome follows the active display's scale.

Display and window geometry is frozen before capture overlays appear. A failed control query falls back to its frozen window; empty desktop falls back to the queried display. Monitor gaps stay unselectable, and the display fallback does not qualify for isolated window-image capture.

## Verification

From `wox.core`, run `go test -race -tags sqlite_fts5 ./ui/runtime ./ui/screenshot` and `go build -tags sqlite_fts5 .`. The macOS selector fixture uses a deterministic AX provider and AddressSanitizer without requesting accessibility permission. Windows native changes also require a CGO-enabled Windows build; Linux desktop integration must be checked in the affected desktop session.
