//go:build windows

package flow

import (
	"fmt"

	"golang.org/x/sys/windows/registry"

	"wox/util/osvariant"
)

// flowBackdropSupported matches Flow skipping the drop shadow once Windows 11
// backdrop (acrylic or mica) is available for a blur theme.
func flowBackdropSupported() bool {
	return osvariant.GetCurrentPlatformVariant() == "win11"
}

// flowSystemDark reads AppsUseLightTheme. Flow treats a missing value as light.
func flowSystemDark() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return false
	}
	return value == 0
}

// flowSystemAccent reads the Windows accent. DWM stores it as AABBGGRR.
func flowSystemAccent() (string, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\DWM`, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue("AccentColor")
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("#%02x%02x%02xff", byte(value), byte(value>>8), byte(value>>16)), true
}

// flowSystemAccentLight1 reads SystemAccentColorLight1. Flow's dark Win11 bullet
// uses that shade, and Windows stores the ramp in AccentPalette from lightest to darkest.
func flowSystemAccentLight1() (string, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()
	value, _, err := key.GetBinaryValue("AccentPalette")
	if err != nil || len(value) < 12 {
		return "", false
	}
	return fmt.Sprintf("#%02x%02x%02xff", value[8], value[9], value[10]), true
}
