# Browser and external-link ownership

This package owns browser discovery, launch arguments, private-window policy,
external-link validation, default-application routing, and its native bridges.
Existing `OpenURL` and `OpenURLInPrivate` contracts remain unchanged.

`OpenExternalURL` accepts HTTP, HTTPS, and mail drafts, preserving the original
string through publication so long checkout URLs and fragments are not
re-encoded. Windows uses `shell.OpenWithOwner` for ordinary links so the effective
system association takes precedence over potentially stale browser registry values.
HTTP(S) links longer than 2048 bytes retain direct browser arguments when the default
browser is known, avoiding truncation of long checkout URLs; unknown browsers and
failed launches fall back to `shell.OpenWithOwner`. macOS uses Launch Services;
Linux uses GTK owner-aware URI dispatch, preserving desktop activation and portal
context rather than replacing it with a subprocess in the native UI path.

The caller dispatches to its GUI thread and supplies only a native owner handle:
HWND on Windows, GtkWindow on Linux, and zero on macOS. Window validity, focus,
and event-loop lifetime belong to the caller. No UI window type, native header,
or session state enters this package. Linux without CGO uses the existing shell
opener; GTK builds require `gtk+-3.0`.

Launch-policy tests inject desktop launch capabilities so checkout, mail, and
fallback regressions can be tested without opening applications.
