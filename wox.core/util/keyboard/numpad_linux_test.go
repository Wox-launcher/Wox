//go:build linux && cgo

package keyboard

import (
	"fmt"
	"testing"
)

// TestLinuxNumpadKeyMappings covers the native and desktop-specific shortcut encodings.
func TestLinuxNumpadKeyMappings(t *testing.T) {
	for digit, evdevCode := range []uint16{82, 79, 80, 81, 75, 76, 77, 71, 72, 73} {
		key := KeyNumpad0 + Key(digit)
		code, err := keyToLinuxKeyCode(key)
		if err != nil || code != 0xffb0+uint32(digit) || linuxKeyCodeToKey(code) != key {
			t.Fatalf("%s maps to X11 symbol %#x (%v), reverse %v", key.Character(), code, err, linuxKeyCodeToKey(code))
		}
		evdev, err := keyToEvdevKeyCode(key)
		if err != nil || evdev != evdevCode || evdevCodeToKey(evdev) != key {
			t.Fatalf("%s maps to evdev code %d (%v), reverse %v", key.Character(), evdev, err, evdevCodeToKey(evdev))
		}
		name := fmt.Sprintf("KP_%d", digit)
		portal, err := formatWaylandPreferredTrigger(ModifierCtrl, key)
		if err != nil || portal != "CTRL+NUM+"+name {
			t.Fatalf("%s maps to portal trigger %q (%v)", key.Character(), portal, err)
		}
		gnome, err := modifiersKeyToGnomeBinding(ModifierCtrl, key)
		if err != nil || gnome != "<Primary>"+name || hyprlandKeyName(key) != name {
			t.Fatalf("%s maps to GNOME trigger %q (%v) and Hyprland name %q", key.Character(), gnome, err, hyprlandKeyName(key))
		}
	}
}
