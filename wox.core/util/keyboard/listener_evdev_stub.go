//go:build !linux

package keyboard

// IsEvdevReadAvailable is always false on non-Linux platforms where
// the evdev interface does not exist.
func IsEvdevReadAvailable() bool { return false }

// IsUinputWriteAvailable is always false on non-Linux platforms where
// the uinput interface does not exist.
func IsUinputWriteAvailable() bool { return false }
