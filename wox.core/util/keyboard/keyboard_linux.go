package keyboard

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

func simulateCopy() error {
	return errors.New("not implemented")
}

func simulatePaste() error {
	return errors.New("not implemented")
}

// simulateCapsLockTap and setCapsLockState are implemented in keyboard_linux_cgo.go
// when cgo is available (using X11 XTest extension via XWayland). When cgo is
// disabled, they fall back to ydotool. If neither is available, they return an error.
// These stubs are only used when cgo is disabled.
func simulateCapsLockTap() error {
	return simulateCapsLockTapImpl()
}

func setCapsLockState(enabled bool) error {
	return setCapsLockStateImpl(enabled)
}

func simulateCopy() error {
	return errors.New("not implemented")
}

func simulatePaste() error {
	return errors.New("not implemented")
}

// evdev LED ioctl: EVIOCGLED(len) = _IOC(_IOC_READ, 'E', 0x19, len)
func evIOCGLED(length int) uintptr {
	return uintptr((2 << 30) | (uint32('E') << 8) | uint32(0x19) | (uint32(length) << 16))
}

// evdev key state ioctl: EVIOCGKEY(len) = _IOC(_IOC_READ, 'E', 0x18, len)
func evIOCGKEY(length int) uintptr {
	return uintptr((2 << 30) | (uint32('E') << 8) | uint32(0x18) | (uint32(length) << 16))
}

// evdevKeyboardFd holds the fd of the first readable keyboard device, used for
// querying key and LED state. It's opened lazily on first use.
var (
	evdevKbdFdOnce sync.Once
	evdevKbdFd     int
	evdevKbdFdErr  error
)

// getEvdevKeyboardFd returns a file descriptor to the first readable keyboard
// device, opening it lazily. The fd is kept open for the process lifetime.
func getEvdevKeyboardFd() (int, error) {
	evdevKbdFdOnce.Do(func() {
		devices, err := discoverKeyboardDevices()
		if err != nil || len(devices) == 0 {
			evdevKbdFdErr = fmt.Errorf("no keyboard device available: %w", err)
			return
		}
		fd, err := syscall.Open(devices[0], syscall.O_RDONLY, 0)
		if err != nil {
			evdevKbdFdErr = err
			return
		}
		evdevKbdFd = fd
	})
	return evdevKbdFd, evdevKbdFdErr
}

// isCapsLockEnabled reads the CapsLock LED state from the evdev keyboard device.
// The LED state reflects the kernel's view of the CapsLock toggle, which is
// kept in sync with the compositor via libinput.
func isCapsLockEnabled() bool {
	fd, err := getEvdevKeyboardFd()
	if err != nil {
		return false
	}

	var ledBits [1]byte // 8 bits, LED_CAPSL = 0
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		evIOCGLED(len(ledBits)),
		uintptr(unsafe.Pointer(&ledBits[0])),
	)
	if errno != 0 {
		return false
	}
	// LED_CAPSL = 0 (bit 0 in the LED bitmap)
	return ledBits[0]&1 != 0
}

// isKeyPressed queries the evdev key state bitmap to check if a key is
// currently physically pressed. This is used by the CapsLock combo handler
// to wait for key release before triggering the callback.
func isKeyPressed(key Key) bool {
	if key == KeyCapsLock {
		// CapsLock is a lock key, not a press-and-hold key. The evdev key
		// state bitmap tracks the physical press state, not the toggle state.
		// For CapsLock, we check the LED state instead.
		return isCapsLockEnabled()
	}

	code, err := keyToEvdevKeyCode(key)
	if err != nil {
		return false
	}

	fd, err := getEvdevKeyboardFd()
	if err != nil {
		return false
	}

	var keyBits [128]byte // 1024 bits, enough for all key codes
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		evIOCGKEY(len(keyBits)),
		uintptr(unsafe.Pointer(&keyBits[0])),
	)
	if errno != 0 {
		return false
	}
	return keyBits[code/8]&(1<<(code%8)) != 0
}

// keyToEvdevKeyCode maps Wox Key values back to Linux kernel key codes.
func keyToEvdevKeyCode(key Key) (uint16, error) {
	switch key {
	case KeyA:
		return 30, nil
	case KeyB:
		return 48, nil
	case KeyC:
		return 46, nil
	case KeyD:
		return 32, nil
	case KeyE:
		return 18, nil
	case KeyF:
		return 33, nil
	case KeyG:
		return 34, nil
	case KeyH:
		return 35, nil
	case KeyI:
		return 23, nil
	case KeyJ:
		return 36, nil
	case KeyK:
		return 37, nil
	case KeyL:
		return 38, nil
	case KeyM:
		return 50, nil
	case KeyN:
		return 49, nil
	case KeyO:
		return 24, nil
	case KeyP:
		return 25, nil
	case KeyQ:
		return 16, nil
	case KeyR:
		return 19, nil
	case KeyS:
		return 31, nil
	case KeyT:
		return 20, nil
	case KeyU:
		return 22, nil
	case KeyV:
		return 47, nil
	case KeyW:
		return 17, nil
	case KeyX:
		return 45, nil
	case KeyY:
		return 21, nil
	case KeyZ:
		return 44, nil
	case Key0:
		return 11, nil
	case Key1:
		return 2, nil
	case Key2:
		return 3, nil
	case Key3:
		return 4, nil
	case Key4:
		return 5, nil
	case Key5:
		return 6, nil
	case Key6:
		return 7, nil
	case Key7:
		return 8, nil
	case Key8:
		return 9, nil
	case Key9:
		return 10, nil
	case KeySpace:
		return 57, nil
	case KeyReturn:
		return 28, nil
	case KeyEscape:
		return 1, nil
	case KeyTab:
		return 15, nil
	case KeyDelete:
		return 111, nil
	case KeyLeft:
		return 105, nil
	case KeyRight:
		return 106, nil
	case KeyUp:
		return 103, nil
	case KeyDown:
		return 108, nil
	case KeyCtrl:
		return 29, nil
	case KeyShift:
		return 42, nil
	case KeyAlt:
		return 56, nil
	case KeySuper:
		return 125, nil
	case KeyCapsLock:
		return 58, nil
	default:
		return 0, fmt.Errorf("no evdev key code for key: %d", key)
	}
}

// simulateCapsLockTap injects a CapsLock key press+release via ydotool, which
// works on both X11 and Wayland. ydotool requires the ydotoold daemon to be
// running and the user to have access to /dev/uinput.
func simulateCapsLockTap() error {
	// ydotool key codes use Linux evdev key codes. KEY_CAPSLOCK = 58.
	// Format: "58:1 58:0" means key 58 press (1) then release (0).
	cmd := exec.Command("ydotool", "key", "58:1", "58:0")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ydotool caps lock tap failed (is ydotoold running?): %w", err)
	}
	return nil
}

// setCapsLockState sets the CapsLock toggle state by injecting a CapsLock tap
// only if the current state doesn't match the target. This mirrors the Windows
// approach where we only toggle when needed.
func setCapsLockState(enabled bool) error {
	current := isCapsLockEnabled()
	if current == enabled {
		return nil
	}
	return simulateCapsLockTap()
}
