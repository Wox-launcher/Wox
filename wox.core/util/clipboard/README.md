# Clipboard ownership

This package owns clipboard formats, image preparation, native publication and
provider state, change detection, self-write bookkeeping, and exit persistence.
Its native headers, implementations, tests, and standalone fixtures stay here.
It does not import UI packages or include their native headers.
Source files, tests, and native fixtures use the `clipboard_` prefix; the package
entry file remains `clipboard.go`.

`PrepareImage` and `PrepareImageFile` prepare a `PreparedImage` on the calling
worker. Windows converts packed RGBA/NRGBA directly into its compatibility DIB
and reuses supplied PNG bytes. macOS and GTK retain encoded PNG when available;
compatibility TIFF/pixbuf formats are produced only when requested. JPEG file
input continues through native pixels without an unnecessary PNG encode.

The window adapter dispatches `PreparedImage.Publish` and `PublishText` onto the
owning UI thread. The only native capability it passes is a Windows HWND or Linux
GdkDisplay; macOS uses the system pasteboard. Prepared formats and provider
lifecycle state belong to this package. `Close` is idempotent and serialized
against publication so interrupted window teardown cannot free in-flight data.
Borrowed input pixels and PNG bytes remain immutable until publication returns.

`Flush` accepts a generic UI-dispatch callback to materialize promised formats
before normal process exit. Windows eager formats need no dispatcher. The
clipboard package never calls a UI-specific native symbol or stores a UI window type.

High-level `Write`, `WriteImageBytes`, and `WriteAnimatedGIF` open a short
settle window so the watcher does not read a half-published clipboard. The copy
stays visible after that window, so clipboard history records text and images
Wox itself copies. Low-level UI publication is observable immediately. These
entry points have different caller policies, not separate ownership of clipboard
resources. Sequential paste restore still opts out in the clipboard plugin.
The Linux portal backend reads its own selection from retained publication bytes,
because GNOME rejects reading the owning session through `SelectionRead`.
Ownership changes disable these local reads, even when the offered formats match.
Linux desktop selection remains in `clipboard_linux.go` with each environment's
implementation in its own file. `clipboard_image_linux_gtk.*` is the UI-thread transport;
it does not select or alter GNOME, KDE, Hyprland, or X11 policies.

On Windows, independently build this package and run image/lifetime/watcher
tests with CGO enabled. Native image-format fixtures inspect prepared handles
without modifying the desktop clipboard. macOS PNG/TIFF and Linux GTK format
tests require their platform SDKs and opt in with `WOX_TEST_NATIVE_CLIPBOARD=1`;
GTK compilation requires `gtk+-3.0`. Existing live-clipboard tests must be run
separately from isolated format tests.
