//go:build windows

package keyboard

import (
	"testing"
	"time"
)

func TestRefineWindowsModifierKeyUsesScanCodeAndExtendedFlag(t *testing.T) {
	if got := refineWindowsModifierKey(KeyShift, 0x10, 0x2A, 0); got != KeyLeftShift {
		t.Fatalf("left shift scan = %v, want left shift", got)
	}
	if got := refineWindowsModifierKey(KeyShift, 0x10, 0x36, 0); got != KeyRightShift {
		t.Fatalf("right shift scan = %v, want right shift", got)
	}
	if got := refineWindowsModifierKey(KeyCtrl, 0x11, 0x1D, 0); got != KeyLeftCtrl {
		t.Fatalf("left ctrl = %v, want left ctrl", got)
	}
	if got := refineWindowsModifierKey(KeyCtrl, 0x11, 0x1D, 0x01); got != KeyRightCtrl {
		t.Fatalf("extended ctrl = %v, want right ctrl", got)
	}
	if got := refineWindowsModifierKey(KeyLeftShift, 0xA0, 0x2A, 0); got != KeyLeftShift {
		t.Fatalf("specific left shift should stay %v", got)
	}
}

func TestWindowsModifierVirtualKeyMapping(t *testing.T) {
	for _, expected := range []struct {
		key Key
		vk  uint32
	}{
		{key: KeyCtrl, vk: 0x11},
		{key: KeyShift, vk: 0x10},
		{key: KeyAlt, vk: 0x12},
		{key: KeyLeftCtrl, vk: 0xA2},
		{key: KeyRightCtrl, vk: 0xA3},
		{key: KeyLeftShift, vk: 0xA0},
		{key: KeyRightShift, vk: 0xA1},
		{key: KeyLeftAlt, vk: 0xA4},
		{key: KeyRightAlt, vk: 0xA5},
		{key: KeyLeftSuper, vk: 0x5B},
		{key: KeyRightSuper, vk: 0x5C},
	} {
		actualVK, err := keyToWindowsVK(expected.key)
		if err != nil {
			t.Fatalf("virtual key for %s: %v", expected.key.Character(), err)
		}
		if actualVK != expected.vk {
			t.Fatalf("virtual key for %s = %#x, want %#x", expected.key.Character(), actualVK, expected.vk)
		}
	}
}

func TestWindowsPunctuationVirtualKeyMapping(t *testing.T) {
	for _, expected := range []struct {
		key Key
		vk  uint32
	}{
		{key: KeyMinus, vk: 0xBD},
		{key: KeyEqual, vk: 0xBB},
		{key: KeyLeftBracket, vk: 0xDB},
		{key: KeyRightBracket, vk: 0xDD},
		{key: KeyBackslash, vk: 0xDC},
		{key: KeySemicolon, vk: 0xBA},
		{key: KeyApostrophe, vk: 0xDE},
		{key: KeyComma, vk: 0xBC},
		{key: KeyPeriod, vk: 0xBE},
		{key: KeySlash, vk: 0xBF},
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
