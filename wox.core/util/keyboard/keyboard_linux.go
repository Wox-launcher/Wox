package keyboard

import (
	"encoding/binary"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Linux evdev key codes used for copy/paste injection via uinput.
// Ctrl is held while C (copy) or V (paste) is tapped, matching the
// Windows/Darwin implementations that use Cmd+C/Cmd+V.
const (
	evKeyLeftCtrlCode = 29
	evKeyCodeC         = 46
	evKeyCodeV         = 47
	evKeyBackspaceCode = 14
	evKeyDownValue     = 1
	evKeyUpValue       = 0
	evSynTypeCode      = 0x00
	evSynReportCode    = 0x00
)

// simulateCopy injects Ctrl+C via the uinput virtual keyboard to trigger
// a system-level copy operation. This requires uinput write access.
func simulateCopy() error {
	waitModifiersRelease()
	return simulateCtrlKeyCombo(evKeyCodeC, "copy")
}

// simulatePaste injects Ctrl+V via the uinput virtual keyboard to trigger
// a system-level paste operation. This requires uinput write access.
func simulatePaste() error {
	waitModifiersRelease()
	return simulateCtrlKeyCombo(evKeyCodeV, "paste")
}

// simulateBackspace injects a Backspace key press+release via the uinput
// virtual keyboard. On Linux/Wayland, when a CapsLock combo (e.g. CapsLock+A)
// is triggered, the system also sees the combo key and types it into the
// focused input field. This function sends a Backspace to delete that
// stray character.
func simulateBackspace() error {
	fd, err := ensureUinputDevice()
	if err != nil {
		return err
	}

	if err := writeUinputEvent(fd, evKeyEventType, evKeyBackspaceCode, int32(evKeyDownValue)); err != nil {
		return fmt.Errorf("backspace press failed: %w", err)
	}
	if err := writeUinputEvent(fd, evSynTypeCode, evSynReportCode, 0); err != nil {
		return fmt.Errorf("syn after press failed: %w", err)
	}
	if err := writeUinputEvent(fd, evKeyEventType, evKeyBackspaceCode, int32(evKeyUpValue)); err != nil {
		return fmt.Errorf("backspace release failed: %w", err)
	}
	if err := writeUinputEvent(fd, evSynTypeCode, evSynReportCode, 0); err != nil {
		return fmt.Errorf("syn after release failed: %w", err)
	}
	return nil
}

// waitModifiersRelease waits for all physical modifier keys to be released
// before injecting Ctrl+C/Ctrl+V. If the trigger hotkey includes Alt/Shift/
// Win, the injected events could be interpreted as a different shortcut
// (e.g. Alt+Ctrl+C) if those modifiers are still held by the user.
func waitModifiersRelease() {
	for i := 0; i < 20; i++ {
		if isKeyPressed(KeyCtrl) || isKeyPressed(KeyAlt) || isKeyPressed(KeyShift) || isKeyPressed(KeySuper) {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		break
	}
}

// simulateCtrlKeyCombo injects a Ctrl+key press+release sequence via uinput.
// The sequence is: Ctrl down → key down → syn → key up → syn → Ctrl up → syn.
func simulateCtrlKeyCombo(keyCode int, label string) error {
	fd, err := ensureUinputDevice()
	if err != nil {
		return err
	}

	// Ctrl down
	if err := writeUinputEvent(fd, evKeyEventType, uint16(evKeyLeftCtrlCode), int32(evKeyDownValue)); err != nil {
		return fmt.Errorf("%s: ctrl press failed: %w", label, err)
	}
	// key down
	if err := writeUinputEvent(fd, evKeyEventType, uint16(keyCode), int32(evKeyDownValue)); err != nil {
		return fmt.Errorf("%s: key press failed: %w", label, err)
	}
	// sync (flush the press events)
	if err := writeUinputEvent(fd, evSynTypeCode, evSynReportCode, 0); err != nil {
		return fmt.Errorf("%s: syn after press failed: %w", label, err)
	}
	// key up
	if err := writeUinputEvent(fd, evKeyEventType, uint16(keyCode), int32(evKeyUpValue)); err != nil {
		return fmt.Errorf("%s: key release failed: %w", label, err)
	}
	// Ctrl up
	if err := writeUinputEvent(fd, evKeyEventType, uint16(evKeyLeftCtrlCode), int32(evKeyUpValue)); err != nil {
		return fmt.Errorf("%s: ctrl release failed: %w", label, err)
	}
	// sync (flush the release events)
	if err := writeUinputEvent(fd, evSynTypeCode, evSynReportCode, 0); err != nil {
		return fmt.Errorf("%s: syn after release failed: %w", label, err)
	}
	return nil
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

// uinput device management for CapsLock state restoration.
//
// On Linux with evdev (read-only), we cannot prevent the system from seeing
// CapsLock key events. When CapsLock is used as a combo prefix (e.g.
// CapsLock+A), the system toggles the caps lock state. To undo this toggle,
// we create a temporary uinput virtual keyboard and inject a CapsLock
// key press+release through it. This requires write access to /dev/uinput,
// which is granted by membership in the 'uinput' group.
//
// We require uinput instead of just the 'input' group because:
// 1. The 'input' group only grants read access to /dev/input/event* devices,
//    which is sufficient for passive key event listening (double-tap detection).
// 2. The 'uinput' group grants write access to /dev/uinput, which allows
//    creating virtual input devices and injecting key events. This is needed
//    to restore the CapsLock state after a combo is triggered.
// 3. Without uinput, CapsLock+A would toggle caps lock every time the combo
//    is used, which is not the expected behavior (matching macOS/Windows
//    where the event is consumed before the system sees it).

// uinput ioctl constants from <linux/uinput.h>
const (
	uiDevCreate  = 0x5501 // USB UI_DEV_CREATE
	uiDevDestroy = 0x5502 // USB UI_DEV_DESTROY
	uiSetEvbit   = 0x40045564 // _IOW('U', 100, int)
	uiSetKeybit  = 0x40045565 // _IOW('U', 101, int)
)

// uinput_set_evbit and uinput_set_keybit are _IOW('U', nr, int) = (1<<30) | ('U'<<8) | nr | (4<<16)
// but the kernel header values are 0x40045564 and 0x40045565 respectively.
// Let me use the raw values directly.

// uinputDevCreate is a lazily-created virtual keyboard used for CapsLock
// injection. It's created on first use and kept open for the process lifetime.
var (
	uinputOnce     sync.Once
	uinputFd       int
	uinputInitErr  error
	uinputClosed   bool
)

// ensureUinputDevice creates a uinput virtual keyboard device that supports
// EV_KEY events with all the keys Wox needs to inject: CapsLock (for state
// restoration), Ctrl+C (for copy), and Ctrl+V (for paste). The device is
// created once and reused for all subsequent injections.
func ensureUinputDevice() (int, error) {
	uinputOnce.Do(func() {
		fd, err := syscall.Open("/dev/uinput", syscall.O_WRONLY, 0)
		if err != nil {
			uinputInitErr = fmt.Errorf("cannot open /dev/uinput (add user to 'uinput' group): %w", err)
			return
		}

		// Enable EV_KEY event type
		// UI_SET_EVBIT = _IOW('U', 100, int)
		evbit := uintptr(0x40045564)
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), evbit, uintptr(0x01)) // EV_KEY = 1
		if errno != 0 {
			syscall.Close(fd)
			uinputInitErr = fmt.Errorf("UI_SET_EVBIT failed: errno=%d", errno)
			return
		}

		// Enable all keys we need to inject: CapsLock, LeftCtrl, C, V
		// UI_SET_KEYBIT = _IOW('U', 101, int)
		keybit := uintptr(0x40045565)
		keysToEnable := []int{
			58,                  // KEY_CAPSLOCK
			evKeyLeftCtrlCode,   // KEY_LEFTCTRL
			evKeyCodeC,          // KEY_C
			evKeyCodeV,          // KEY_V
			evKeyBackspaceCode,  // KEY_BACKSPACE
		}
		for _, key := range keysToEnable {
			_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), keybit, uintptr(key))
			if errno != 0 {
				syscall.Close(fd)
				uinputInitErr = fmt.Errorf("UI_SET_KEYBIT(%d) failed: errno=%d", key, errno)
				return
			}
		}

		// Write the uinput_user_dev struct to configure the device name and
		// other properties before creating it. The kernel header defines:
		// struct uinput_user_dev {
		//   char name[80];                 // 80
		//   struct input_id id;            // 8 (4x __u16)
		//   int ff_effects_max;            // 4
		//   int absmax[ABS_CNT];           // 64 * 4 = 256
		//   int absmin[ABS_CNT];           // 256
		//   int absfuzz[ABS_CNT];          // 256
		//   int absflat[ABS_CNT];          // 256
		// }
		// Total: 80 + 8 + 4 + 256*4 = 1116 bytes (verified via C sizeof).
		var userDev [1116]byte
		name := []byte("Wox Input Injector\x00")
		copy(userDev[:80], name)

		_, err = syscall.Write(fd, userDev[:])
		if err != nil {
			syscall.Close(fd)
			uinputInitErr = fmt.Errorf("write uinput_user_dev failed: %w", err)
			return
		}

		// Create the device: UI_DEV_CREATE = 0x5501 (no argument)
		_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(uiDevCreate), 0)
		if errno != 0 {
			syscall.Close(fd)
			uinputInitErr = fmt.Errorf("UI_DEV_CREATE failed: errno=%d", errno)
			return
		}

		uinputFd = fd
	})
	return uinputFd, uinputInitErr
}

// writeUinputEvent writes a single input_event to the uinput device.
// struct input_event { struct timeval time; __u16 type; __u16 code; __s32 value; }
// On 64-bit Linux: 16 + 2 + 2 + 4 = 24 bytes.
func writeUinputEvent(fd int, evType, code uint16, value int32) error {
	var ev [24]byte
	// timeval is zeroed (kernel fills it in)
	// type at offset 16
	binary.LittleEndian.PutUint16(ev[16:18], evType)
	// code at offset 18
	binary.LittleEndian.PutUint16(ev[18:20], code)
	// value at offset 20
	binary.LittleEndian.PutUint32(ev[20:24], uint32(value))

	_, err := syscall.Write(fd, ev[:])
	return err
}

// simulateCapsLockTap injects a CapsLock key press+release via the uinput
// virtual keyboard device. This toggles the caps lock state without relying
// on any external tools.
func simulateCapsLockTap() error {
	fd, err := ensureUinputDevice()
	if err != nil {
		return err
	}

	// KEY_CAPSLOCK = 58, EV_KEY = 0x01
	// value: 1 = press, 0 = release
	if err := writeUinputEvent(fd, evKeyEventType, 58, int32(evKeyDownValue)); err != nil {
		return fmt.Errorf("caps lock press injection failed: %w", err)
	}
	// EV_SYN = 0x00, SYN_REPORT = 0
	if err := writeUinputEvent(fd, evSynTypeCode, evSynReportCode, 0); err != nil {
		return fmt.Errorf("syn after press failed: %w", err)
	}
	if err := writeUinputEvent(fd, evKeyEventType, 58, int32(evKeyUpValue)); err != nil {
		return fmt.Errorf("caps lock release injection failed: %w", err)
	}
	if err := writeUinputEvent(fd, evSynTypeCode, evSynReportCode, 0); err != nil {
		return fmt.Errorf("syn after release failed: %w", err)
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