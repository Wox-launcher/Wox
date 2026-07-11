package keyboard

func SimulateCopy() error {
	return simulateCopy()
}

func SimulatePaste() error {
	return simulatePaste()
}

// SimulateBackspace sends a Backspace key press+release through the platform
// input system. On Linux/Wayland this is used to undo the character that the
// system typed when a CapsLock combo key (e.g. CapsLock+A) was pressed — since
// evdev is read-only, the system sees the combo key and types it into the
// focused input field.
func SimulateBackspace() error {
	return simulateBackspace()
}

// SimulateCapsLockPress sends one Caps Lock down/up pair through the platform input system.
func SimulateCapsLockPress() error {
	return simulateCapsLockPress()
}

// SetCapsLockState explicitly sets the platform Caps Lock state when the OS supports it.
func SetCapsLockState(enabled bool) error {
	return setCapsLockState(enabled)
}

// IsCapsLockEnabled reports the current Caps Lock toggle state when the platform supports it.
func IsCapsLockEnabled() bool {
	return isCapsLockEnabled()
}

// IsKeyPressed reports whether the physical key is currently pressed when the platform supports it.
func IsKeyPressed(key Key) bool {
	return isKeyPressed(key)
}

// SimulateType types text into the currently focused window via the platform
// input system. On macOS and Windows this uses native Unicode keyboard events
// so the clipboard is not touched. On Linux it falls back to clipboard + paste
// because uinput does not support direct Unicode injection.
func SimulateType(text string) error {
	return simulateType(text)
}
