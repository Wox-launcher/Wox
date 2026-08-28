//go:build linux

package osvariant

import "wox/util"

// GetCurrentPlatformVariant returns the current Linux theme variant, such as hyprland.
func GetCurrentPlatformVariant() string {
	return linuxPlatformVariantForSession(util.IsHyprlandSession())
}
