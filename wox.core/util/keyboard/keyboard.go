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
