//go:build windows

package keyboard

import (
	"testing"
	"time"
)

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

func TestWindowsShellReservedComboOnlyCoversWinSpaceFamily(t *testing.T) {
	for _, tc := range []struct {
		name      string
		modifiers Modifier
		key       Key
		reserved  bool
	}{
		{name: "win+space", modifiers: ModifierSuper, key: KeySpace, reserved: true},
		{name: "shift+win+space", modifiers: ModifierSuper | ModifierShift, key: KeySpace, reserved: true},
		{name: "ctrl+win+space", modifiers: ModifierSuper | ModifierCtrl, key: KeySpace, reserved: true},
		{name: "win+alt+space", modifiers: ModifierSuper | ModifierAlt, key: KeySpace, reserved: true},
		{name: "alt+space", modifiers: ModifierAlt, key: KeySpace, reserved: false},
		{name: "win+e", modifiers: ModifierSuper, key: KeyE, reserved: false},
		{name: "win+l", modifiers: ModifierSuper, key: KeyL, reserved: false},
	} {
		if actual := isWindowsShellReservedCombo(tc.modifiers, tc.key); actual != tc.reserved {
			t.Fatalf("%s reserved = %t, want %t", tc.name, actual, tc.reserved)
		}
	}
}

func TestWindowsHookHotkeyFiresOncePerPressAndMasksWin(t *testing.T) {
	fired := make(chan struct{}, 4)
	maskSent := 0
	hotkey := &hookHotkey{
		modifiers:   ModifierSuper,
		key:         KeySpace,
		callback:    func() { fired <- struct{}{} },
		sendWinMask: func() { maskSent++ },
	}

	if consumed := hotkey.handle(RawKeyEvent{Type: EventTypeKeyDown, Key: KeyA, Modifiers: ModifierSuper}); consumed {
		t.Fatalf("unrelated key must not be consumed")
	}
	if consumed := hotkey.handle(RawKeyEvent{Type: EventTypeKeyDown, Key: KeySpace, Modifiers: ModifierCtrl}); consumed {
		t.Fatalf("space with a different modifier set must not be consumed")
	}
	if consumed := hotkey.handle(RawKeyEvent{Type: EventTypeKeyUp, Key: KeySpace}); consumed {
		t.Fatalf("release without a consumed press must pass through")
	}

	if consumed := hotkey.handle(RawKeyEvent{Type: EventTypeKeyDown, Key: KeySpace, Modifiers: ModifierSuper}); !consumed {
		t.Fatalf("win+space press must be consumed")
	}
	if consumed := hotkey.handle(RawKeyEvent{Type: EventTypeKeyDown, Key: KeySpace, Modifiers: ModifierSuper}); !consumed {
		t.Fatalf("auto-repeat press must be consumed")
	}
	if consumed := hotkey.handle(RawKeyEvent{Type: EventTypeKeyUp, Key: KeySpace}); !consumed {
		t.Fatalf("release of a consumed press must be consumed")
	}

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatalf("callback did not fire")
	}
	select {
	case <-fired:
		t.Fatalf("auto-repeat must not fire the callback again")
	case <-time.After(100 * time.Millisecond):
	}
	if maskSent != 1 {
		t.Fatalf("win mask key sent %d times, want 1", maskSent)
	}
}

func TestWindowsRegistersWinSpaceGlobalHotkey(t *testing.T) {
	registration, err := RegisterGlobalHotkey(ModifierSuper, KeySpace, func() {})
	if err != nil {
		t.Fatalf("register Win+Space: %v", err)
	}
	if err := registration.Unregister(); err != nil {
		t.Fatalf("unregister Win+Space: %v", err)
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
