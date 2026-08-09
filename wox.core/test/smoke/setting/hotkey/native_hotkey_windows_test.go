//go:build wox_ui_smoke && windows

package hotkey

import (
	"testing"

	"golang.org/x/sys/windows"
)

const keyEventFlagKeyUp = 0x0002

var keybdEvent = windows.NewLazySystemDLL("user32.dll").NewProc("keybd_event")

// sendNativeHotkey uses Windows keyboard injection because main-hotkey registration is a global OS boundary.
func sendNativeHotkey(t *testing.T, hotkey string) {
	t.Helper()
	var keys []uintptr
	switch hotkey {
	case "f12":
		keys = []uintptr{0x7B}
	case "ctrl+f12":
		keys = []uintptr{0x11, 0x7B}
	case "alt+space":
		keys = []uintptr{0x12, 0x20}
	default:
		t.Fatalf("unsupported native smoke hotkey %q", hotkey)
	}
	if err := keybdEvent.Find(); err != nil {
		t.Fatalf("resolve keybd_event: %v", err)
	}
	for _, key := range keys {
		keybdEvent.Call(key, 0, 0, 0)
	}
	for index := len(keys) - 1; index >= 0; index-- {
		keybdEvent.Call(keys[index], 0, keyEventFlagKeyUp, 0)
	}
}
