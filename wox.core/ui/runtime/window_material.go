package woxui

import (
	"runtime"
	"sync/atomic"
)

// Window material is a process default owned by Open, not a per-window option.
//
// Every window uses the same native material: Desktop Acrylic on Windows 11,
// Accent Acrylic on Windows 10, NSVisualEffectMaterialPopover on macOS, and
// ext-background-effect-v1 blur on Linux compositors that advertise it.
// Linux sessions without that protocol paint an opaque theme wash instead.
//
// WindowRoleScreenshot is the only opt-out, because that surface must show
// the live desktop. Focus, Nonactivating, Resizable, Topmost, and
// Application vs Utility must not pick a different material.
//
// Light vs dark only tints that shared material. SetDefaultAppearance is
// what Open applies; existing windows still update through SetAppearance.

var defaultWindowAppearanceDark atomic.Bool

func init() {
	defaultWindowAppearanceDark.Store(true)
}

// SetDefaultAppearance is the light/dark inherited by windows created after this call.
func SetDefaultAppearance(isDark bool) {
	defaultWindowAppearanceDark.Store(isDark)
}

// DefaultAppearanceIsDark reports the light/dark Open will apply.
func DefaultAppearanceIsDark() bool {
	return defaultWindowAppearanceDark.Load()
}

func windowUsesDefaultMaterial(role WindowRole) bool {
	return role != WindowRoleScreenshot
}

// HasNativeWindowMaterial reports whether windows can show compositor backdrop
// through a translucent theme wash.
func HasNativeWindowMaterial() bool {
	return nativeWindowMaterialAvailable()
}

// NativeWindowCornerRadius returns the painted window-outline radius.
// Linux compositor blur is a rectangle, so a rounded wash would leak at the corners.
func NativeWindowCornerRadius(requested float32) float32 {
	if runtime.GOOS == "linux" && HasNativeWindowMaterial() {
		return 0
	}
	return requested
}
