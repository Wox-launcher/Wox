package hotkey

import (
	"errors"
	"wox/util/keyboard"
)

// RegistrationErrorKey keeps backend failures distinct from confirmed hotkey conflicts.
// A failed registration alone does not prove another application owns the key.
func RegistrationErrorKey(err error) string {
	if errors.Is(err, keyboard.ErrHotkeyConflict) {
		return "i18n:ui_hotkey_conflict_system"
	}
	if errors.Is(err, keyboard.ErrGlobalHotkeysUnavailable) {
		return "i18n:ui_hotkey_registration_unsupported"
	}
	return "i18n:ui_hotkey_registration_failed"
}
