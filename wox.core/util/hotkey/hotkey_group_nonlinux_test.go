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

// TestReconcileIndependentLeavesUnchangedHotkeysRegistered verifies replacement and rollback without disturbing other owners.
func TestReconcileIndependentLeavesUnchangedHotkeysRegistered(t *testing.T) {
	original := registerGlobalHotkey
	t.Cleanup(func() { registerGlobalHotkey = original })
	registrations := map[keyboard.Key][]*testHotkeyRegistration{}
	fail := false
	registerGlobalHotkey = func(_ keyboard.Modifier, key keyboard.Key, _ func()) (keyboard.HotkeyRegistration, error) {
		if fail && key == keyboard.KeyF11 {
			return nil, errors.New("unavailable")
		}
		r := &testHotkeyRegistration{}
		registrations[key] = append(registrations[key], r)
		return r, nil
	}
	previous := []Spec{{CombineKey: "ctrl+f12", Callback: func() {}}, {CombineKey: "ctrl+f10", Callback: func() {}}}
	group, err := RegisterGroup(t.Context(), previous)
	if err != nil {
		t.Fatal(err)
	}
	defer group.Unregister(t.Context())
	keepKey, _ := BindingKey("ctrl+f12")
	next := []Spec{{CombineKey: "ctrl+f12", Callback: func() {}}, {CombineKey: "ctrl+f11", Callback: func() {}}}
	fail = true
	if err := group.ReconcileIndependent(t.Context(), previous, next, map[string]bool{keepKey: true}); err == nil {
		t.Fatal("unavailable hotkey accepted")
	}
	if registrations[keyboard.KeyF12][0].unregistered || len(registrations[keyboard.KeyF12]) != 1 || len(registrations[keyboard.KeyF10]) != 2 {
		t.Fatal("rollback disturbed unchanged registration or failed to restore old registration")
	}
	fail = false
	if err := group.ReconcileIndependent(t.Context(), previous, next, map[string]bool{keepKey: true}); err != nil {
		t.Fatal(err)
	}
	if registrations[keyboard.KeyF12][0].unregistered || len(registrations[keyboard.KeyF12]) != 1 || len(registrations[keyboard.KeyF11]) != 1 {
		t.Fatal("unchanged hotkey was re-registered")
	}
}
