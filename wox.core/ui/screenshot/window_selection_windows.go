//go:build windows

package screenshot

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
	woxui "wox/ui/runtime"
	"wox/util"
	"wox/util/window"
)

var screenshotDwmGetWindowAttribute = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmGetWindowAttribute")
var screenshotGetShellWindow = syscall.NewLazyDLL("user32.dll").NewProc("GetShellWindow")

// captureWindowsWindowCandidates snapshots visible DWM frames before the screenshot overlay is shown.
func captureWindowsWindowCandidates() []window.ManagedWindow {
	windows, err := window.ListManagedWindows()
	if err != nil {
		util.GetLogger().Debug(context.Background(), "screenshot window detection: "+err.Error())
		return nil
	}
	candidates := windows[:0]
	shell, _, _ := screenshotGetShellWindow.Call()
	for _, candidate := range windows {
		handle, err := strconv.ParseUint(candidate.Id, 10, 64)
		if err != nil || candidate.IsMinimized || uintptr(handle) == shell || win.HWND(handle) == win.GetDesktopWindow() {
			continue
		}
		// Cloaked windows belong to another virtual desktop even when IsWindowVisible is true.
		var cloaked uint32
		if screenshotDwmGetWindowAttribute.Find() == nil {
			screenshotDwmGetWindowAttribute.Call(uintptr(handle), 14, uintptr(unsafe.Pointer(&cloaked)), unsafe.Sizeof(cloaked))
		}
		if cloaked == 0 {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

// captureWindowsSelectedWindow matches the frozen physical frame to its HWND, then crops using actual captured pixels.
func captureWindowsSelectedWindow(windows []window.ManagedWindow, desktop image.Rectangle, selection Rect) (*image.RGBA, error) {
	selected := image.Rect(int(selection.X)+desktop.Min.X, int(selection.Y)+desktop.Min.Y,
		int(selection.X+selection.Width)+desktop.Min.X, int(selection.Y+selection.Height)+desktop.Min.Y)
	for _, candidate := range windows {
		bounds := candidate.Bounds
		frame := image.Rect(bounds.X, bounds.Y, bounds.X+bounds.Width, bounds.Y+bounds.Height)
		if frame.Intersect(desktop) != selected {
			continue
		}
		hwnd, err := strconv.ParseUint(candidate.Id, 10, 64)
		if err != nil {
			return nil, err
		}
		pixels, err := woxui.CaptureWindowsWindow(uintptr(hwnd), frame)
		if err != nil {
			return nil, err
		}
		cropped := image.NewRGBA(image.Rect(0, 0, selected.Dx(), selected.Dy()))
		draw.Draw(cropped, cropped.Bounds(), pixels, selected.Min.Sub(frame.Min), draw.Src)
		return cropped, nil
	}
	return nil, fmt.Errorf("selected window is no longer available")
}

// windowsScreenshotWindowCandidates keeps native physical pixels relative to the virtual desktop image.
// No monitor DPI conversion is needed: the Windows screenshot surface deliberately uses scale 1.
func windowsScreenshotWindowCandidates(windows []window.ManagedWindow, desktop image.Rectangle) []Rect {
	var candidates []Rect
	for _, candidate := range windows {
		bounds := candidate.Bounds
		visible := image.Rect(bounds.X, bounds.Y, bounds.X+bounds.Width, bounds.Y+bounds.Height).Intersect(desktop)
		if visible.Dx() >= 2 && visible.Dy() >= 2 {
			candidates = append(candidates, Rect{X: float32(visible.Min.X - desktop.Min.X), Y: float32(visible.Min.Y - desktop.Min.Y), Width: float32(visible.Dx()), Height: float32(visible.Dy())})
		}
	}
	return candidates
}
