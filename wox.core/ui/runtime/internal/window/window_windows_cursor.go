//go:build windows

package window

import (
	"wox/util"
	woxsvg "wox/util/svg"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

var windowsGrabCursors = util.NewHashMap[int, win.HCURSOR]()
var destroyWindowsCursor = windows.NewLazySystemDLL("user32.dll").NewProc("DestroyCursor")

// windowsGrabCursor rasterizes the hand at the active window's DPI and reuses it across windows.
func windowsGrabCursor(hwnd win.HWND) win.HCURSOR {
	dpi := int(win.GetDpiForWindow(hwnd))
	if dpi <= 0 {
		dpi = 96
	}
	size := min(256, max(16, (20*dpi+48)/96))
	if cursor, ok := windowsGrabCursors.Load(size); ok {
		return cursor
	}
	const hand = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path fill="#ffffff" stroke="#000000" stroke-width="1.6" stroke-linejoin="round" d="M10 15V6a2 2 0 0 1 4 0v6-9a2 2 0 0 1 4 0v9-7a2 2 0 0 1 4 0v9-5a2 2 0 0 1 4 0v10c0 4-2 6-4 8v2H12v-2l-5-6-3-5a2 2 0 0 1 3-2z"/></svg>`
	raster, err := woxsvg.Render(hand, size, size)
	if err == nil {
		var image *Image
		image, err = NewImage(raster)
		if err == nil {
			var handle win.HICON
			handle, err = windowsIconOrCursorFromImage(image, size, false)
			if err == nil {
				cursor := win.HCURSOR(handle)
				actual, loaded := windowsGrabCursors.LoadOrStore(size, cursor)
				if loaded {
					destroyWindowsCursor.Call(uintptr(cursor))
				}
				return actual
			}
		}
	}
	return win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_HAND))
}
