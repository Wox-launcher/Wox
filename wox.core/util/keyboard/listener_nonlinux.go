//go:build !linux

package keyboard

// Stubs for Linux-only keyboard functionality (evdev, uinput, GNOME portal,
// Hyprland IPC). The real implementations live in listener_linux_*.go. These
// no-op stubs let the rest of the package compile unchanged on macOS/Windows.

// IsEvdevReadAvailable is always false on non-Linux platforms where
// the evdev interface does not exist.
func IsEvdevReadAvailable() bool { return false }

// IsUinputWriteAvailable is always false on non-Linux platforms where
// the uinput interface does not exist.
func IsUinputWriteAvailable() bool { return false }

// CheckUinputAccess is always NotInGroup on non-Linux platforms.
func CheckUinputAccess() UinputAccessStatus { return UinputAccessNotInGroup }

// InvokeGnomeHotkeyCallback is a no-op on non-Linux platforms.
// The real implementation is in listener_linux_gnome.go.
func InvokeGnomeHotkeyCallback(id string) {}

func registerGlobalHotkeysLinuxHyprland(specs []GlobalHotkeySpec) (HotkeyRegistration, bool, error) {
	return nil, false, nil
}

func InvokeHyprlandHotkeyCallback(key string) {}

func RegisterHyprlandHotkeyCallback(key string, callback func()) {}
