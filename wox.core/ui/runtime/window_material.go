package woxui

import "sync/atomic"

// Window material is a process default owned by Open, not a per-window option.
//
// Every window uses the same native material: Desktop Acrylic on Windows 11,
// Accent Acrylic on Windows 10, and NSVisualEffectMaterialPopover on macOS.
// Linux has no system backdrop and paints the theme wash instead.
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
