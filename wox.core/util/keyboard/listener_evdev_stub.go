//go:build !linux

package keyboard

// IsEvdevRawListenerAvailable is always false on non-Linux platforms where
// the evdev interface does not exist.
func IsEvdevRawListenerAvailable() bool { return false }
