# Tray menus

`MenuItem` carries commands and group boundaries, not cross-platform styling.
`SeparatorBefore` inserts a non-interactive native separator before an item,
except at the beginning of the menu. Commands retain their IDs and callbacks.

- Windows keeps Win32 menu tracking and owns only item measurement and painting
  in `menu_windows.c`. Each opening snapshots strings and command IDs, reads the
  system application appearance, and measures the system menu font at the anchor
  monitor's DPI. The transparent owner follows that monitor so native popup chrome
  uses the same DPI. Item dimensions are logical units converted once to pixels.
  High contrast and initialization failures use the original system menu.
  Strings are exposed through both `MIIM_STRING` and `MSAAMENUINFO`. Popup resources
  are released before dispatching a command, which may exit the app.
- macOS uses AppKit `NSMenu` and `NSMenuItem` separators. AppKit owns appearance,
  font metrics, menu spacing, accessibility, and point-to-pixel conversion.
- Linux exports StatusNotifierItem/DBusMenu semantics; the desktop tray host owns
  appearance and layout. Separators use `type=separator` and IDs separate from
  command IDs. Desktop-specific adaptations stay in the corresponding session
  implementation, such as `tray_linux_omarchy.go`.

These system surfaces follow platform typography and scaling, independently of
the launcher's theme and UI density. Main and query menus share the Windows
renderer. Omarchy exposes query icons as separate StatusNotifierItems; other
Linux sessions do not yet support query tray icons.

## Verification

Run `go test ./util/tray` from `wox.core` on the target platform. On Windows this
also builds the native fixture with the configured Go C compiler and checks
real menu snapshots, GDI drawing, text expansion, DPI scaling, mnemonics, and GDI
resource balance. On Linux the tests check exported layout and event dispatch.

For a Windows visual check, compile `testdata/menu_windows_test.c` with
`-DTRAY_MENU_PREVIEW -mwindows -lgdi32 -luser32` and launch the resulting executable.
Right-click opens the production menu tracker, F6/F7 selects the fixture's dark
or light palette, and Escape closes the fixture. This does not install a tray
icon or change system appearance. Actual high contrast, mixed-monitor placement,
screen readers, and macOS/Linux desktop rendering still require native checks.
