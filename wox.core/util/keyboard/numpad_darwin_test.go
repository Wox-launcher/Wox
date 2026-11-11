package keyboard

import "testing"

// TestDarwinNumpadKeyMapping covers Carbon registration and event tap key codes.
func TestDarwinNumpadKeyMapping(t *testing.T) {
	for digit, expected := range []uint32{82, 83, 84, 85, 86, 87, 88, 89, 91, 92} {
		key := KeyNumpad0 + Key(digit)
		code, err := keyToDarwinKeyCode(key)
		if err != nil || code != expected || darwinKeyCodeToKey(code) != key {
			t.Fatalf("%s maps to macOS key %d (%v), reverse %v", key.Character(), code, err, darwinKeyCodeToKey(code))
		}
	}
}
