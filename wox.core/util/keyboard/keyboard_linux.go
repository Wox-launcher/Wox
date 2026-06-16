package keyboard

import "errors"

func simulateCopy() error {
	return errors.New("not implemented")
}

func simulatePaste() error {
	return errors.New("not implemented")
}

func simulateCapsLockTap() error {
	return errors.New("not implemented")
}

func setCapsLockState(enabled bool) error {
	return errors.New("not implemented")
}

func isCapsLockEnabled() bool {
	return false
}

func isKeyPressed(key Key) bool {
	return false
}
