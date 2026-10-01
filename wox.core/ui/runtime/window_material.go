package woxui

import (
	"runtime"
	"sync/atomic"
)

// Window material is a process default owned by Open, not a per-window option.
//
// Every window uses the same native material: Desktop Acrylic on Windows 11,
// Accent Acrylic on Windows 10, Liquid Glass (NSGlassEffectView) on macOS 26+,
// NSVisualEffectMaterialPopover on older macOS, and ext-background-effect-v1
// blur on Linux compositors that advertise it.
// Linux sessions without that protocol paint an opaque theme wash instead.
//
// Authored AppBorderColor, AppBorderWidth, or AppBorderRadius disable native
// blur on Windows/macOS and use transparent composition so the Go UI can paint
// the outline. Linux retains blur inside a matching rounded protocol region.
// WindowRoleScreenshot always opts out, because that surface must show
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
// Linux default material uses a square surface. Authored chrome bypasses this
// helper and supplies its own radius to both the present clip and the blur region.
func NativeWindowCornerRadius(requested float32) float32 {
	if runtime.GOOS == "linux" && HasNativeWindowMaterial() {
		return 0
	}
	return requested
}

// ThemeCapabilities exposes available materials for nested theme variants.
// A theme can tune an available capability but cannot enable an unsupported protocol.
func ThemeCapabilities() []string {
	if HasNativeWindowMaterial() {
		return []string{"backgroundBlur"}
	}
	return nil
}
