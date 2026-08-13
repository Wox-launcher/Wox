//go:build !linux

package hotkey

import (
	"context"
	"errors"
	"testing"
	"wox/util/keyboard"
)

type testHotkeyRegistration struct {
	unregistered bool
}

func (r *testHotkeyRegistration) Unregister() error {
	r.unregistered = true
	return nil
}

func TestRegisterGroupKeepsSuccessfulNormalHotkeysOnNonLinux(t *testing.T) {
	originalRegisterGlobalHotkey := registerGlobalHotkey
	t.Cleanup(func() {
		registerGlobalHotkey = originalRegisterGlobalHotkey
	})

	successfulRegistration := &testHotkeyRegistration{}
	registerGlobalHotkey = func(modifiers keyboard.Modifier, key keyboard.Key, callback func()) (keyboard.HotkeyRegistration, error) {
		if key == keyboard.KeySpace {
			return nil, errors.New("hotkey already registered")
		}
		return successfulRegistration, nil
	}

	group, err := RegisterGroup(context.Background(), []Spec{
		{CombineKey: "alt+space", Callback: func() {}},
		{CombineKey: "ctrl+f12", Callback: func() {}},
	})
	if err != nil {
		t.Fatalf("register group: %v", err)
	}

	registeredKeys := group.RegisteredCombineKeys()
	if len(registeredKeys) != 1 || registeredKeys[0] != "ctrl+f12" {
		t.Fatalf("registered keys = %v, want [ctrl+f12]", registeredKeys)
	}
	if successfulRegistration.unregistered {
		t.Fatal("successful hotkey was unregistered after another hotkey failed")
	}

	group.Unregister(context.Background())
	if !successfulRegistration.unregistered {
		t.Fatal("successful hotkey was not unregistered with its group")
	}
}
