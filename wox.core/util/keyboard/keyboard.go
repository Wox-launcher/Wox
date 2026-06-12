package keyboard

func SimulateCopy() error {
	return simulateCopy()
}

func SimulatePaste() error {
	return simulatePaste()
}

// SimulateCapsLockTap sends one Caps Lock down/up pair through the platform input system.
func SimulateCapsLockTap() error {
	return simulateCapsLockTap()
}

// SetCapsLockState explicitly sets the platform Caps Lock state when the OS supports it.
func SetCapsLockState(enabled bool) error {
	return setCapsLockState(enabled)
}

// IsKeyPressed reports whether the physical key is currently pressed when the platform supports it.
func IsKeyPressed(key Key) bool {
	return isKeyPressed(key)
}
