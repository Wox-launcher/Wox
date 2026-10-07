# Browser and external-link ownership

This package owns browser discovery, launch arguments, private-window policy,
external-link validation, default-application routing, and its native bridges.
Existing `OpenURL` and `OpenURLInPrivate` contracts remain unchanged.

`OpenExternalURL` accepts HTTP, HTTPS, and mail drafts, preserving the original
string through publication so long checkout URLs and fragments are not
re-encoded. Windows prefers direct browser arguments when the default browser
is known and uses `shell.OpenWithOwner` for fallback. macOS uses Launch Services;
Linux uses GTK owner-aware URI dispatch, preserving desktop activation and portal
context rather than replacing it with a subprocess in the native UI path.

The caller dispatches to its GUI thread and supplies only a native owner handle:
HWND on Windows, GtkWindow on Linux, and zero on macOS. Window validity, focus,
and event-loop lifetime belong to the caller. No UI window type, native header,
or session state enters this package. Linux without CGO uses the existing shell
opener; GTK builds require `gtk+-3.0`.

Launch-policy tests inject desktop launch capabilities so checkout, mail, and
fallback regressions can be tested without opening applications.
