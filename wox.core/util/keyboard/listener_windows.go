package keyboard

/*
#include <stdint.h>

int woxKeyboardEnsureThread(void);
int woxKeyboardRegisterHotkey(int id, unsigned int modifiers, unsigned int vkCode, unsigned long *errorCodeOut);
int woxKeyboardUnregisterHotkey(int id, unsigned long *errorCodeOut);
int woxKeyboardSetRawKeyboardHookEnabled(int enabled, unsigned long *errorCodeOut);
*/
import "C"

import (
	"fmt"
	"sync"
	"wox/util"
)

const (
	rawEventKeyDown = 0
	rawEventKeyUp   = 1
)

type hotkeyRegistration struct {
	id   int
	once sync.Once
}

type rawKeySubscription struct {
	id   int
	once sync.Once
}

var (
	managerMu        sync.Mutex
	nextHotkeyID     = 1
	nextListenerID   = 1
	hotkeyCallbacks  = map[int]func(){}
	rawKeyListeners  = map[int]RawKeyHandler{}
	rawHookIsEnabled bool
)

func RegisterGlobalHotkey(modifiers Modifier, key Key, callback func()) (HotkeyRegistration, error) {
	vkCode, err := keyToWindowsVK(key)
	if err != nil {
		return nil, err
	}

	managerMu.Lock()
	id := nextHotkeyID
	nextHotkeyID++
	managerMu.Unlock()

	if err := ensureNativeKeyboardThread(); err != nil {
		return nil, err
	}

	var errCode C.ulong
	ok := C.woxKeyboardRegisterHotkey(C.int(id), C.uint(modifiers), C.uint(vkCode), &errCode)
	if ok == 0 {
		return nil, fmt.Errorf("failed to register hotkey (err=%d)", uint32(errCode))
	}

	managerMu.Lock()
	hotkeyCallbacks[id] = callback
	managerMu.Unlock()

	return &hotkeyRegistration{id: id}, nil
}

func (r *hotkeyRegistration) Unregister() error {
	if r == nil {
		return nil
	}

	var unregisterErr error
	r.once.Do(func() {
		managerMu.Lock()
		delete(hotkeyCallbacks, r.id)
		managerMu.Unlock()

		var errCode C.ulong
		ok := C.woxKeyboardUnregisterHotkey(C.int(r.id), &errCode)
		if ok == 0 {
			unregisterErr = fmt.Errorf("failed to unregister hotkey (err=%d)", uint32(errCode))
		}
	})
	return unregisterErr
}

func AddRawKeyListener(handler RawKeyHandler) (RawKeySubscription, error) {
	if handler == nil {
		return nil, fmt.Errorf("raw key handler is required")
	}

	if err := ensureNativeKeyboardThread(); err != nil {
		return nil, err
	}

	managerMu.Lock()
	id := nextListenerID
	nextListenerID++
	rawKeyListeners[id] = handler
	managerMu.Unlock()

	// The native hook thread can be recreated independently of Go-side listener
	// state in dev rebuild flows. Enabling is idempotent on the native side, so
	// confirm it for every new listener instead of trusting the cached flag.
	if err := setRawHookEnabled(true); err != nil {
		managerMu.Lock()
		delete(rawKeyListeners, id)
		managerMu.Unlock()
		return nil, err
	}

	return &rawKeySubscription{id: id}, nil
}

func (s *rawKeySubscription) Close() error {
	if s == nil {
		return nil
	}

	var closeErr error
	s.once.Do(func() {
		managerMu.Lock()
		delete(rawKeyListeners, s.id)
		needDisable := rawHookIsEnabled && len(rawKeyListeners) == 0
		managerMu.Unlock()

		if needDisable {
			closeErr = setRawHookEnabled(false)
		}
	})
	return closeErr
}

func ensureNativeKeyboardThread() error {
	ok := C.woxKeyboardEnsureThread()
	if ok == 0 {
		return fmt.Errorf("failed to initialize native keyboard thread")
	}
	return nil
}

func setRawHookEnabled(enabled bool) error {
	var errCode C.ulong
	value := 0
	if enabled {
		value = 1
	}
	ok := C.woxKeyboardSetRawKeyboardHookEnabled(C.int(value), &errCode)
	if ok == 0 {
		return fmt.Errorf("failed to toggle raw keyboard hook (err=%d)", uint32(errCode))
	}

	managerMu.Lock()
	rawHookIsEnabled = enabled
	managerMu.Unlock()
	return nil
}

//export keyboardHotkeyTriggeredCGO
func keyboardHotkeyTriggeredCGO(id C.int) {
	managerMu.Lock()
	callback := hotkeyCallbacks[int(id)]
	managerMu.Unlock()
	if callback == nil {
		return
	}

	util.Go(util.NewTraceContext(), fmt.Sprintf("global hotkey %d callback", int(id)), func() {
		callback()
	})
}

//export keyboardHookEventCGO
func keyboardHookEventCGO(eventKind C.int, vkCode C.uint, modifiers C.uint) C.int {
	key := windowsVKToKey(uint32(vkCode))
	event := RawKeyEvent{
		Key:           key,
		Character:     key.Character(),
		Modifiers:     Modifier(modifiers),
		NativeKeyCode: uint32(vkCode),
	}
	if int(eventKind) == rawEventKeyUp {
		event.Type = EventTypeKeyUp
	} else {
		event.Type = EventTypeKeyDown
	}

	managerMu.Lock()
	listeners := make([]RawKeyHandler, 0, len(rawKeyListeners))
	for _, listener := range rawKeyListeners {
		listeners = append(listeners, listener)
	}
	managerMu.Unlock()

	consume := false
	for _, listener := range listeners {
		if listener != nil && listener(event) {
			consume = true
		}
	}

	if consume {
		return 1
	}
	return 0
}

func keyToWindowsVK(key Key) (uint32, error) {
	switch key {
	case KeyA:
		return 'A', nil
	case KeyB:
		return 'B', nil
	case KeyC:
		return 'C', nil
	case KeyD:
		return 'D', nil
	case KeyE:
		return 'E', nil
	case KeyF:
		return 'F', nil
	case KeyG:
		return 'G', nil
	case KeyH:
		return 'H', nil
	case KeyI:
		return 'I', nil
	case KeyJ:
		return 'J', nil
	case KeyK:
		return 'K', nil
	case KeyL:
		return 'L', nil
	case KeyM:
		return 'M', nil
	case KeyN:
		return 'N', nil
	case KeyO:
		return 'O', nil
	case KeyP:
		return 'P', nil
	case KeyQ:
		return 'Q', nil
	case KeyR:
		return 'R', nil
	case KeyS:
		return 'S', nil
	case KeyT:
		return 'T', nil
	case KeyU:
		return 'U', nil
	case KeyV:
		return 'V', nil
	case KeyW:
		return 'W', nil
	case KeyX:
		return 'X', nil
	case KeyY:
		return 'Y', nil
	case KeyZ:
		return 'Z', nil
	case Key0:
		return '0', nil
	case Key1:
		return '1', nil
	case Key2:
		return '2', nil
	case Key3:
		return '3', nil
	case Key4:
		return '4', nil
	case Key5:
		return '5', nil
	case Key6:
		return '6', nil
	case Key7:
		return '7', nil
	case Key8:
		return '8', nil
	case Key9:
		return '9', nil
	case KeySpace:
		return 0x20, nil
	case KeyReturn:
		return 0x0D, nil
	case KeyEscape:
		return 0x1B, nil
	case KeyTab:
		return 0x09, nil
	case KeyDelete:
		return 0x2E, nil
	case KeyLeft:
		return 0x25, nil
	case KeyRight:
		return 0x27, nil
	case KeyUp:
		return 0x26, nil
	case KeyDown:
		return 0x28, nil
	case KeyF1:
		return 0x70, nil
	case KeyF2:
		return 0x71, nil
	case KeyF3:
		return 0x72, nil
	case KeyF4:
		return 0x73, nil
	case KeyF5:
		return 0x74, nil
	case KeyF6:
		return 0x75, nil
	case KeyF7:
		return 0x76, nil
	case KeyF8:
		return 0x77, nil
	case KeyF9:
		return 0x78, nil
	case KeyF10:
		return 0x79, nil
	case KeyF11:
		return 0x7A, nil
	case KeyF12:
		return 0x7B, nil
	default:
		return 0, fmt.Errorf("unsupported Windows hotkey key: %d", key)
	}
}

func windowsVKToKey(vkCode uint32) Key {
	switch vkCode {
	case 'A':
		return KeyA
	case 'B':
		return KeyB
	case 'C':
		return KeyC
	case 'D':
		return KeyD
	case 'E':
		return KeyE
	case 'F':
		return KeyF
	case 'G':
		return KeyG
	case 'H':
		return KeyH
	case 'I':
		return KeyI
	case 'J':
		return KeyJ
	case 'K':
		return KeyK
	case 'L':
		return KeyL
	case 'M':
		return KeyM
	case 'N':
		return KeyN
	case 'O':
		return KeyO
	case 'P':
		return KeyP
	case 'Q':
		return KeyQ
	case 'R':
		return KeyR
	case 'S':
		return KeyS
	case 'T':
		return KeyT
	case 'U':
		return KeyU
	case 'V':
		return KeyV
	case 'W':
		return KeyW
	case 'X':
		return KeyX
	case 'Y':
		return KeyY
	case 'Z':
		return KeyZ
	case '0':
		return Key0
	case '1':
		return Key1
	case '2':
		return Key2
	case '3':
		return Key3
	case '4':
		return Key4
	case '5':
		return Key5
	case '6':
		return Key6
	case '7':
		return Key7
	case '8':
		return Key8
	case '9':
		return Key9
	case 0x20:
		return KeySpace
	case 0x0D:
		return KeyReturn
	case 0x1B:
		return KeyEscape
	case 0x09:
		return KeyTab
	case 0x2E:
		return KeyDelete
	case 0x25:
		return KeyLeft
	case 0x27:
		return KeyRight
	case 0x26:
		return KeyUp
	case 0x28:
		return KeyDown
	case 0x70:
		return KeyF1
	case 0x71:
		return KeyF2
	case 0x72:
		return KeyF3
	case 0x73:
		return KeyF4
	case 0x74:
		return KeyF5
	case 0x75:
		return KeyF6
	case 0x76:
		return KeyF7
	case 0x77:
		return KeyF8
	case 0x78:
		return KeyF9
	case 0x79:
		return KeyF10
	case 0x7A:
		return KeyF11
	case 0x7B:
		return KeyF12
	case 0xA2, 0xA3, 0x11:
		return KeyCtrl
	case 0xA0, 0xA1, 0x10:
		return KeyShift
	case 0xA4, 0xA5, 0x12:
		return KeyAlt
	case 0x5B, 0x5C:
		return KeySuper
	default:
		return KeyUnknown
	}
}
