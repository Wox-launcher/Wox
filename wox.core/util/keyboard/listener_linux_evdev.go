//go:build linux

package keyboard

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"
	"wox/util"
)

// evdev event type for key events (EV_KEY from <linux/input-event-codes.h>).
const evKeyEventType = 0x01

// input_event on 64-bit Linux: struct timeval (8+8) + __u16 type + __u16 code + __s32 value = 24 bytes.
const inputEventSize = 24

// EVIOCGBIT returns the bitmap of supported event types for a given ev event type.
// Linux kernel: EVIOCGBIT(ev,len) = _IOC(_IOC_READ, 'E', 0x20 + ev, len)
// _IOC(dir,type,nr,size): dir<<30, type<<8, nr<<0, size<<16
func evIOCGBIT(ev, length int) uintptr {
	return uintptr((2 << 30) | (uint32('E') << 8) | uint32(0x20+ev) | (uint32(length) << 16))
}

// evdevCodeToKey maps Linux kernel key codes (from <linux/input-event-codes.h>)
// to Wox Key values. Modifier keys are essential for double-tap detection;
// common alphanumeric and control keys are included so the double-tap tracker
// can properly invalidate a pending sequence when any non-modifier key is pressed.
func evdevCodeToKey(code uint16) Key {
	switch code {
	// Modifiers
	case 29, 97: // KEY_LEFTCTRL, KEY_RIGHTCTRL
		return KeyCtrl
	case 42, 54: // KEY_LEFTSHIFT, KEY_RIGHTSHIFT
		return KeyShift
	case 56, 100: // KEY_LEFTALT, KEY_RIGHTALT
		return KeyAlt
	case 125, 126: // KEY_LEFTMETA, KEY_RIGHTMETA
		return KeySuper
	case 58: // KEY_CAPSLOCK
		return KeyCapsLock

	// Letters
	case 30:
		return KeyA
	case 48:
		return KeyB
	case 46:
		return KeyC
	case 32:
		return KeyD
	case 18:
		return KeyE
	case 33:
		return KeyF
	case 34:
		return KeyG
	case 35:
		return KeyH
	case 23:
		return KeyI
	case 36:
		return KeyJ
	case 37:
		return KeyK
	case 38:
		return KeyL
	case 50:
		return KeyM
	case 49:
		return KeyN
	case 24:
		return KeyO
	case 25:
		return KeyP
	case 16:
		return KeyQ
	case 19:
		return KeyR
	case 31:
		return KeyS
	case 20:
		return KeyT
	case 22:
		return KeyU
	case 47:
		return KeyV
	case 17:
		return KeyW
	case 45:
		return KeyX
	case 21:
		return KeyY
	case 44:
		return KeyZ

	// Numbers
	case 2:
		return Key1
	case 3:
		return Key2
	case 4:
		return Key3
	case 5:
		return Key4
	case 6:
		return Key5
	case 7:
		return Key6
	case 8:
		return Key7
	case 9:
		return Key8
	case 10:
		return Key9
	case 11:
		return Key0

	// Special keys
	case 57: // KEY_SPACE
		return KeySpace
	case 28: // KEY_ENTER
		return KeyReturn
	case 1: // KEY_ESC
		return KeyEscape
	case 15: // KEY_TAB
		return KeyTab
	case 111: // KEY_DELETE
		return KeyDelete

	// Arrow keys
	case 105:
		return KeyLeft
	case 106:
		return KeyRight
	case 103:
		return KeyUp
	case 108:
		return KeyDown

	// Function keys
	case 59:
		return KeyF1
	case 60:
		return KeyF2
	case 61:
		return KeyF3
	case 62:
		return KeyF4
	case 63:
		return KeyF5
	case 64:
		return KeyF6
	case 65:
		return KeyF7
	case 66:
		return KeyF8
	case 67:
		return KeyF9
	case 68:
		return KeyF10
	case 87:
		return KeyF11
	case 88:
		return KeyF12

	default:
		return KeyUnknown
	}
}

// isKeyboardDevice checks whether an evdev device is a real keyboard by
// verifying that it supports EV_KEY events AND has common letter keys (KEY_A)
// in its key bitmap. This filters out mice, touchpads, power buttons, and
// other devices that expose EV_KEY for button events but aren't keyboards.
func isKeyboardDevice(fd int) bool {
	// First check: does the device support EV_KEY at all?
	var evBits [8]byte
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		evIOCGBIT(0, len(evBits)),
		uintptr(unsafe.Pointer(&evBits[0])),
	)
	if errno != 0 || evBits[0]&(1<<1) == 0 {
		return false
	}

	// Second check: does the device have KEY_A (code 30) in its key bitmap?
	// Real keyboards always have letter keys; mice/touchpads/power buttons don't.
	var keyBits [128]byte // 1024 bits, enough for all key codes
	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		evIOCGBIT(1, len(keyBits)), // EV_KEY = 1
		uintptr(unsafe.Pointer(&keyBits[0])),
	)
	if errno != 0 {
		return false
	}
	// KEY_A = 30, bit 30 in the key bitmap → byte 3 (30/8=3), bit 6 (30%8=6)
	return keyBits[30/8]&(1<<(30%8)) != 0
}

// discoverKeyboardDevices returns paths of readable evdev keyboard devices.
// Devices that cannot be opened (EACCES) are skipped; if no keyboard device
// is readable the returned error explains the input group requirement.
func discoverKeyboardDevices() ([]string, error) {
	matches, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}

	var keyboards []string
	var firstErr error
	for _, path := range matches {
		fd, err := syscall.Open(path, syscall.O_RDONLY, 0)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		isKbd := isKeyboardDevice(fd)
		syscall.Close(fd)
		if isKbd {
			keyboards = append(keyboards, path)
		}
	}

	if len(keyboards) == 0 && firstErr != nil {
		return nil, fmt.Errorf("no readable keyboard devices (add user to 'input' group): %w", firstErr)
	}
	return keyboards, nil
}

// evdevRawSubscription manages the goroutines and file descriptors for an
// evdev raw key listener session. Close shuts down all goroutines and releases
// all file descriptors.
type evdevRawSubscription struct {
	once   sync.Once
	stopCh chan struct{}
	fds    []int
}

func (s *evdevRawSubscription) Close() error {
	s.once.Do(func() {
		close(s.stopCh)
		for _, fd := range s.fds {
			syscall.Close(fd)
		}
	})
	return nil
}

// addRawKeyListenerLinuxEvdev opens all readable keyboard evdev devices and
// forwards key events to the handler. This is a passive read-only listener:
// it does not grab, remap, or inject any keyboard input. It only works for
// modifier-key double-tap detection and similar patterns that consume
// RawKeyEvent sequences.
func addRawKeyListenerLinuxEvdev(handler RawKeyHandler) (RawKeySubscription, error) {
	if handler == nil {
		return nil, fmt.Errorf("raw key handler is required")
	}

	devices, err := discoverKeyboardDevices()
	if err != nil {
		return nil, err
	}

	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"[hotkey] evdev: discovered %d keyboard devices: %v", len(devices), devices))

	sub := &evdevRawSubscription{
		stopCh: make(chan struct{}),
	}

	for _, path := range devices {
		fd, err := syscall.Open(path, syscall.O_RDONLY, 0)
		if err != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
				"[hotkey] evdev: cannot open %s: %v", path, err))
			continue
		}
		sub.fds = append(sub.fds, fd)
	}

	if len(sub.fds) == 0 {
		return nil, fmt.Errorf("evdev: no keyboard devices could be opened (add user to 'input' group)")
	}

	for _, fd := range sub.fds {
		go evdevReadLoop(fd, sub.stopCh, handler)
	}

	return sub, nil
}

// evdevReadLoop reads input_event records from an evdev device fd and forwards
// parsed key events to the handler. It exits when stopCh is closed or the fd
// is closed/returns an error.
func evdevReadLoop(fd int, stopCh <-chan struct{}, handler RawKeyHandler) {
	var buf [inputEventSize]byte
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		n, err := syscall.Read(fd, buf[:])
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return // fd closed or error; goroutine exits
		}
		if n < inputEventSize {
			continue
		}

		evType := binary.LittleEndian.Uint16(buf[16:18])
		if evType != evKeyEventType {
			continue
		}

		code := binary.LittleEndian.Uint16(buf[18:20])
		value := int32(binary.LittleEndian.Uint32(buf[20:24]))

		// Ignore key-repeat events (value == 2); only press (1) and release (0) matter.
		if value == 2 {
			continue
		}

		key := evdevCodeToKey(code)
		if key == KeyUnknown {
			continue
		}

		event := RawKeyEvent{
			Key:           key,
			Character:     key.Character(),
			NativeKeyCode: uint32(code),
		}
		if value == 1 {
			event.Type = EventTypeKeyDown
		} else {
			event.Type = EventTypeKeyUp
		}

		util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf(
			"[hotkey] evdev event: key=%s type=%s code=%d", key.Character(), event.Type, code))

		handler(event)
	}
}

// evdevAvailability caches the result of the first IsEvdevRawListenerAvailable
// probe so we don't open/close /dev/input/event0 on every call.
var (
	evdevAvailableOnce sync.Once
	evdevAvailable     bool
)

// IsEvdevRawListenerAvailable reports whether the current user has read access
// to evdev keyboard devices. On Linux this requires membership in the 'input'
// group (or equivalent udev rules granting read access to /dev/input/event*).
// The probe is cached for the process lifetime.
func IsEvdevRawListenerAvailable() bool {
	evdevAvailableOnce.Do(func() {
		devices, err := discoverKeyboardDevices()
		if err != nil || len(devices) == 0 {
			evdevAvailable = false
			return
		}
		// Verify we can actually open one of the discovered keyboards.
		fd, err := syscall.Open(devices[0], syscall.O_RDONLY, 0)
		if err != nil {
			evdevAvailable = false
			return
		}
		syscall.Close(fd)
		evdevAvailable = true
	})
	return evdevAvailable
}
