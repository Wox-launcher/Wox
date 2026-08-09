//go:build windows

package keyboard

import "testing"

func TestWindowsFunctionKeyVirtualKeyMappingThroughF24(t *testing.T) {
	for _, expected := range []struct {
		key Key
		vk  uint32
	}{
		{key: KeyF1, vk: 0x70},
		{key: KeyF12, vk: 0x7B},
		{key: KeyF13, vk: 0x7C},
		{key: KeyF24, vk: 0x87},
	} {
		actualVK, err := keyToWindowsVK(expected.key)
		if err != nil {
			t.Fatalf("virtual key for %s: %v", expected.key.Character(), err)
		}
		if actualVK != expected.vk {
			t.Fatalf("virtual key for %s = %#x, want %#x", expected.key.Character(), actualVK, expected.vk)
		}
		if actualKey := windowsVKToKey(expected.vk); actualKey != expected.key {
			t.Fatalf("key for virtual key %#x = %v, want %v", expected.vk, actualKey, expected.key)
		}
	}
}

func TestWindowsRegistersF13GlobalHotkey(t *testing.T) {
	registration, err := RegisterGlobalHotkey(ModifierCtrl, KeyF13, func() {})
	if err != nil {
		t.Fatalf("register Ctrl+F13: %v", err)
	}
	if err := registration.Unregister(); err != nil {
		t.Fatalf("unregister Ctrl+F13: %v", err)
	}
}
