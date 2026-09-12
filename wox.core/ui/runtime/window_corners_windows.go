//go:build windows

package woxui

import (
	"context"
	"fmt"
	"math"
	"syscall"
	"unsafe"
	"wox/util"

	"github.com/lxn/win"
)

var cornerSetWindowRgn = syscall.NewLazyDLL("user32.dll").NewProc("SetWindowRgn")
var cornerCreateRoundRectRgn = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateRoundRectRgn")

func (w *platformWindow) setWindowChrome(custom bool, radius float32) error {
	return w.call(windowCommand{kind: windowCommandSetWindowChrome, customChrome: custom, cornerRadius: radius}).err
}

func (w *platformWindow) setWindowChromeNative(custom bool, radius float32) error {
	w.customWindowChrome = custom
	if !custom {
		if err := w.setCornerRadiusNative(-1); err != nil {
			return err
		}
		w.applyBackdrop()
		return nil
	}
	if radius >= 0 {
		return w.setCornerRadiusNative(radius)
	}
	if w.customCornerRadius != nil {
		if err := w.setCornerRadiusNative(-1); err != nil {
			return err
		}
		w.customWindowChrome = true
	}
	w.applyBackdrop()
	return nil
}

// setCornerRadiusNative restores the system region when leaving a custom theme.
func (w *platformWindow) setCornerRadiusNative(radius float32) error {
	if radius < 0 {
		if w.customCornerRadius == nil {
			return nil
		}
		previous := w.customCornerRadius
		w.customCornerRadius = nil
		result, _, err := cornerSetWindowRgn.Call(uintptr(w.hwnd), 0, 1)
		if result == 0 {
			w.customCornerRadius = previous
			return fmt.Errorf("reset window region: %w", err)
		}
		w.cornerRegion = [3]int32{}
		w.setCornerAttributes(false)
		w.applyBackdrop()
		if w.renderer != nil {
			return w.renderer.setCornerRadius(nil)
		}
		return nil
	}
	w.customCornerRadius = &radius
	w.applyBackdrop()
	return w.applyCornerRadius()
}

// applyBackdrop is shared by creation, theme changes, and re-show so native material follows the current shape.
// Custom corners use transparent composition: neither system nor Accent blur reliably follows an arbitrary region.
func (w *platformWindow) applyBackdrop() {
	if !windowsWindowUsesSystemBackdrop(w.options) {
		return
	}
	if w.customCornerRadius == nil && !w.customWindowChrome {
		tryApplyWindowsAccent(w.hwnd, 0, 0, 0)
		applyWindowsBackdrop(w.hwnd, w.darkAppearance)
		return
	}
	w.setCornerAttributes(true)
	backdrop := int32(dwmSystemBackdropNone) // DWMSBT_NONE: do not leave Desktop Acrylic behind the rounded visual.
	if dwmSetWindowAttribute.Find() == nil {
		result, _, _ := dwmSetWindowAttribute.Call(uintptr(w.hwnd), dwmwaSystemBackdrop, uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
		if int32(result) < 0 {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("disable system backdrop for custom corners: HRESULT 0x%08X", uint32(result)))
		}
	}
	// Disable both blur paths, including an Accent policy left over from an earlier theme.
	if !tryApplyWindowsAccent(w.hwnd, 0, 0, 0) {
		util.GetLogger().Warn(context.Background(), "disable Accent blur for custom window corners failed")
	}
	// An extended glass frame can still paint a rectangular tint behind the transparent swap chain.
	margins := windowsMargins{}
	if dwmExtendFrameIntoClientArea.Find() == nil {
		result, _, _ := dwmExtendFrameIntoClientArea.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&margins)))
		if int32(result) < 0 {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("clear glass frame for custom corners: HRESULT 0x%08X", uint32(result)))
		}
	}
	radius := float32(-1)
	if w.customCornerRadius != nil {
		radius = *w.customCornerRadius
	}
	util.GetLogger().Debug(context.Background(), fmt.Sprintf("window custom chrome: transparent composition, native blur disabled, radius=%g logical, scale=%g", radius, w.scale))
}

// setCornerAttributes prevents DWM's own edge and rounding from overlapping authored chrome.
func (w *platformWindow) setCornerAttributes(custom bool) {
	corner, border := int32(dwmWindowCornerRound), uint32(0xFFFFFFFF)
	if custom {
		corner, border = 1, 0xFFFFFFFE
	}
	if dwmSetWindowAttribute.Find() == nil {
		dwmSetWindowAttribute.Call(uintptr(w.hwnd), dwmwaWindowCorner, uintptr(unsafe.Pointer(&corner)), unsafe.Sizeof(corner))
		dwmSetWindowAttribute.Call(uintptr(w.hwnd), 34, uintptr(unsafe.Pointer(&border)), unsafe.Sizeof(border))
	}
}

// windowCornerDiameter converts logical radius at the HWND boundary and clamps to physical client dimensions.
func windowCornerDiameter(radius, scale float32, width, height int32) int32 {
	return int32(math.Round(float64(min(max(float32(0), radius)*scale*2, float32(min(width, height))))))
}

// applyCornerRadius runs on the native thread after size/DPI changes, keeping the material and content inside one region.
func (w *platformWindow) applyCornerRadius() error {
	if w.hwnd == 0 {
		return nil
	}
	if w.customCornerRadius == nil {
		return nil
	}
	var bounds win.RECT
	if !win.GetClientRect(w.hwnd, &bounds) {
		return fmt.Errorf("read window corner bounds")
	}
	width, height := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
	if width <= 0 || height <= 0 {
		return nil
	}
	diameter := windowCornerDiameter(*w.customCornerRadius, w.scale, width, height)
	if w.renderer != nil {
		physicalRadius := max(float32(0), *w.customCornerRadius) * w.scale
		if err := w.renderer.setCornerRadius(&physicalRadius); err != nil {
			return err
		}
	}
	w.setCornerAttributes(true)
	shape := [3]int32{width, height, diameter}
	if w.cornerRegion == shape {
		return nil
	}
	region, _, err := cornerCreateRoundRectRgn.Call(0, 0, uintptr(width+1), uintptr(height+1), uintptr(diameter), uintptr(diameter))
	if region == 0 {
		return fmt.Errorf("create window corner region: %w", err)
	}
	// SetWindowRgn takes ownership only on success. DWM rounding must not clip a custom shape a second time.
	// Cache before the call: SetWindowRgn synchronously sends window-position messages.
	previous := w.cornerRegion
	w.cornerRegion = shape
	result, _, err := cornerSetWindowRgn.Call(uintptr(w.hwnd), region, 1)
	if result == 0 {
		w.cornerRegion = previous
		win.DeleteObject(win.HGDIOBJ(region))
		return fmt.Errorf("set window corner region: %w", err)
	}
	return nil
}
