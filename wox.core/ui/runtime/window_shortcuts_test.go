package woxui

import "testing"

// TestWindowShortcutsMatchExactPlatformModifiers covers all desktop mappings without native dependencies.
func TestWindowShortcutsMatchExactPlatformModifiers(t *testing.T) {
	for _, platform := range []string{"windows", "linux", "darwin"} {
		primary := KeyModifierControl
		if platform == "darwin" {
			primary = KeyModifierMeta
		}
		for modifiers := KeyModifiers(0); modifiers < 16; modifiers++ {
			for _, key := range []Key{",", "w", "q", "a", KeyUnknown} {
				want := modifiers == primary && (key == "," || key == "w")
				if got := isWindowShortcut(key, modifiers, platform); got != want {
					t.Fatalf("%s key=%q modifiers=%d: got %t, want %t", platform, key, modifiers, got, want)
				}
			}
		}
	}
}
