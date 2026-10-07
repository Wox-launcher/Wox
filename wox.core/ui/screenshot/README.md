# Screenshot ownership

This package owns screenshot capture workflows, the portable editor, object selection, export, and recording orchestration. Screenshot-specific native code belongs here too:

- `object_selection.go` owns portable selection paths, cancellation, foreground/refinement scheduling, and preview transitions.
- `object_selection_darwin.*`, `object_selection_windows.*`, and `object_selection_linux*` implement bounded native geometry queries; the editor receives rectangle values. Windows keeps a lazy cache for the current frozen HWND on a single MTA worker, capped at 4096 nodes. Expanded nodes retain only geometry; unloaded nodes and partially decoded sibling batches retain UIA references until expansion, window changes, or session cleanup. Cancellation preserves cache progress. AX/AT-SPI references stay inside the query lifetime.
- `selection_darwin.m` owns the macOS capture snapshot, selection session, inspector, selection layers, click/drag handling, and scrolling border. `selection_darwin.h` is its package-local C bridge; `platform_darwin.go` owns the Go handoff.
- `platform_*.go` maps capture and native coordinates into the editor. Linux desktop-specific capture behavior stays in the corresponding environment file.

Dependencies point from `screenshot` to `runtime`. Runtime supplies general window/event-loop, cursor, coordinate, and image-capture capabilities; it does not own selection policy or call screenshot implementations. `runtime/native_darwin_platform.h` exposes the shared overlay panel and platform primitives to native feature code without depending on screenshot headers or session types. Selector tests and native provider fixtures live in this package; shared overlay cursor tests stay in runtime.

## Coordinate contract

macOS native selection uses global top-left logical points and maps captured pixels using the actual image dimensions. Windows selection uses physical desktop pixels relative to the captured virtual desktop. Linux selection uses the capture's logical bounds, with native pixel geometry converted at the platform boundary. Selection chrome follows the active display's scale.

Display and window geometry is frozen before capture overlays appear. A failed control query falls back to its frozen window; empty desktop falls back to the queried display. Windows refreshes its UIA tree once on pointer dwell because Chromium/Electron may expose descendants after the initial request; subsequent foreground and refinement queries reuse the cache. Monitor gaps stay unselectable, and the display fallback does not qualify for isolated window-image capture.

Export encodes PNG once, publishes it with the platform compatibility formats, and invokes the in-process clipboard-ready callback immediately after successful publication. The system plugin uses this callback for its success notification, before history file writes, display validation, or editable-scene snapshots. The capture worker continues that work while retaining native capture resources; file-based API callers still receive a completed result only after the history image is written. Editable scene preparation snapshots pixels and annotations before capture resources close; high-precision window conversion, compression, hashing, and scene file writes run in the returned background save job. Window and cursor alpha retain their 16-bit scene precision for re-editing. A later history or scene save failure reports a persistence warning without treating the delivered clipboard image as failed.

`util/imageencode` owns the lossless PNG Sub filter and fast deflate stream. Windows CGO builds use native packed-row filtering without an additional full-image buffer; other builds keep the equivalent Go implementation. Export diagnostics separate PNG encoding, native clipboard preparation/commit, display validation, and scene pixel snapshots so debug-build costs are distinguishable from clipboard contention.

## Verification

From `wox.core`, run `go test -race -tags sqlite_fts5 ./ui/runtime ./ui/screenshot` and `go build -tags sqlite_fts5 .`. The macOS selector fixture uses a deterministic AX provider and AddressSanitizer without requesting accessibility permission. Windows native changes also require a CGO-enabled Windows build; Linux desktop integration must be checked in the affected desktop session.
