package hotkey

import (
	"errors"
	"fmt"
	"testing"
	"wox/util/keyboard"
)

func TestRegistrationErrorKey(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{"confirmed desktop conflict", fmt.Errorf("COSMIC: %w", keyboard.ErrHotkeyConflict), "i18n:ui_hotkey_conflict_system"},
		{"missing portal", keyboard.ErrGlobalHotkeysUnavailable, "i18n:ui_hotkey_registration_unsupported"},
		{"wrapped backend error", fmt.Errorf("register main: %w", keyboard.ErrGlobalHotkeysUnavailable), "i18n:ui_hotkey_registration_unsupported"},
		{"permission failure", errors.New("permission denied"), "i18n:ui_hotkey_registration_failed"},
		{"unregistered without error", nil, "i18n:ui_hotkey_registration_failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := RegistrationErrorKey(test.err); got != test.want {
				t.Fatalf("RegistrationErrorKey(%v) = %q, want %q", test.err, got, test.want)
			}
		})
	}
}
