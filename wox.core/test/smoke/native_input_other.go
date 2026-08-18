//go:build wox_ui_smoke && !windows && !darwin

package smoke

import "fmt"

// SendNativeKeyChord reports that deterministic native input is unavailable on this platform.
func SendNativeKeyChord(keys ...string) error {
	return fmt.Errorf("native smoke chord is unsupported on this platform: %v", keys)
}
