//go:build linux && cgo

package keyboard

import "testing"

func TestLinuxInsertMappings(t *testing.T) {
	code, err := keyToLinuxKeyCode(KeyInsert)
	if err != nil || code != 0xFF63 || linuxKeyCodeToKey(code) != KeyInsert {
		t.Fatalf("X11 Insert mapping: code=%d err=%v reverse=%v", code, err, linuxKeyCodeToKey(code))
	}
	evdev, err := keyToEvdevKeyCode(KeyInsert)
	if err != nil || evdev != 110 || evdevCodeToKey(evdev) != KeyInsert {
		t.Fatalf("evdev Insert mapping: code=%d err=%v reverse=%v", evdev, err, evdevCodeToKey(evdev))
	}
	name, err := keyToWaylandTriggerName(KeyInsert)
	if err != nil || name != "Insert" || hyprlandKeyName(KeyInsert) != "INSERT" {
		t.Fatalf("Wayland Insert mapping: name=%q err=%v Hyprland=%q", name, err, hyprlandKeyName(KeyInsert))
	}
}
