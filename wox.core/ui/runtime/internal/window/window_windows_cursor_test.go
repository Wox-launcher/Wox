//go:build windows

package window

import (
	"testing"
	"unsafe"

	"github.com/lxn/win"
)

func TestWindowsGrabCursorIsACustomHand(t *testing.T) {
	cursor := windowsGrabCursor(0)
	if cursor == 0 || cursor == win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_HAND)) {
		t.Fatal("grab cursor fell back to the pointing hand")
	}
	var info win.ICONINFO
	if !win.GetIconInfo(win.HICON(cursor), &info) {
		t.Fatal("cannot inspect grab cursor")
	}
	defer win.DeleteObject(win.HGDIOBJ(info.HbmColor))
	defer win.DeleteObject(win.HGDIOBJ(info.HbmMask))
	if info.FIcon != 0 || info.XHotspot == 0 || info.YHotspot == 0 {
		t.Fatalf("invalid cursor metadata: %+v", info)
	}
	var bitmap win.BITMAP
	if win.GetObject(win.HGDIOBJ(info.HbmColor), unsafe.Sizeof(bitmap), unsafe.Pointer(&bitmap)) == 0 || bitmap.BmWidth != 20 || bitmap.BmHeight != 20 {
		t.Fatalf("grab cursor dimensions = %dx%d, want 20x20 at 100%% DPI", bitmap.BmWidth, bitmap.BmHeight)
	}
}
