//go:build linux && cgo

package keyboard

/*
#cgo LDFLAGS: -lX11 -lpthread
#include <stdlib.h>

int woxLinuxEnsureKeyboardReady(char **errorOut);
int woxLinuxRegisterHotkey(int id, unsigned int modifiers, unsigned int keyCode, char **errorOut);
int woxLinuxUnregisterHotkey(int id, char **errorOut);
int woxLinuxSetRawKeyboardHookEnabled(int enabled, char **errorOut);
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
	"wox/util"
)

type x11HotkeyRegistration struct {
	id   int
	once sync.Once
}

type x11RawKeySubscription struct {
	id   int
	once sync.Once
}

var (
	x11ManagerMu        sync.Mutex
	x11NextHotkeyID     = 1
	x11NextListenerID   = 1
	x11HotkeyCallbacks  = map[int]func(){}
	x11RawKeyListeners  = map[int]RawKeyHandler{}
	x11RawHookIsEnabled bool
)

func registerGlobalHotkeyLinuxX11(modifiers Modifier, key Key, callback func()) (HotkeyRegistration, error) {
	keyCode, err := keyToLinuxKeyCode(key)
	if err != nil {
		return nil, err
	}

	x11ManagerMu.Lock()
	id := x11NextHotkeyID
	x11NextHotkeyID++
	x11ManagerMu.Unlock()

	if err := runLinuxKeyboardCall(func(errorOut **C.char) C.int {
		return C.woxLinuxEnsureKeyboardReady(errorOut)
	}); err != nil {
		return nil, err
	}

	if err := runLinuxKeyboardCall(func(errorOut **C.char) C.int {
		return C.woxLinuxRegisterHotkey(C.int(id), C.uint(modifiers), C.uint(keyCode), errorOut)
	}); err != nil {
		return nil, err
	}

	x11ManagerMu.Lock()
	x11HotkeyCallbacks[id] = callback
	x11ManagerMu.Unlock()

	return &x11HotkeyRegistration{id: id}, nil
}

func (r *x11HotkeyRegistration) Unregister() error {
	if r == nil {
		return nil
	}

	var unregisterErr error
	r.once.Do(func() {
		x11ManagerMu.Lock()
		delete(x11HotkeyCallbacks, r.id)
		x11ManagerMu.Unlock()

		unregisterErr = runLinuxKeyboardCall(func(errorOut **C.char) C.int {
			return C.woxLinuxUnregisterHotkey(C.int(r.id), errorOut)
		})
	})
	return unregisterErr
}

func addRawKeyListenerLinuxX11(handler RawKeyHandler) (RawKeySubscription, error) {
	if handler == nil {
		return nil, fmt.Errorf("raw key handler is required")
	}

	if err := runLinuxKeyboardCall(func(errorOut **C.char) C.int {
		return C.woxLinuxEnsureKeyboardReady(errorOut)
	}); err != nil {
		return nil, err
	}

	x11ManagerMu.Lock()
	id := x11NextListenerID
	x11NextListenerID++
	x11RawKeyListeners[id] = handler
	needEnable := !x11RawHookIsEnabled
	x11ManagerMu.Unlock()

	if needEnable {
		if err := setLinuxX11RawHookEnabled(true); err != nil {
			x11ManagerMu.Lock()
			delete(x11RawKeyListeners, id)
			x11ManagerMu.Unlock()
			return nil, err
		}
	}

	return &x11RawKeySubscription{id: id}, nil
}

func (s *x11RawKeySubscription) Close() error {
	if s == nil {
		return nil
	}

	var closeErr error
	s.once.Do(func() {
		x11ManagerMu.Lock()
		delete(x11RawKeyListeners, s.id)
		needDisable := x11RawHookIsEnabled && len(x11RawKeyListeners) == 0
		x11ManagerMu.Unlock()

		if needDisable {
			closeErr = setLinuxX11RawHookEnabled(false)
		}
	})
	return closeErr
}

func setLinuxX11RawHookEnabled(enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}

	if err := runLinuxKeyboardCall(func(errorOut **C.char) C.int {
		return C.woxLinuxSetRawKeyboardHookEnabled(C.int(value), errorOut)
	}); err != nil {
		return err
	}

	x11ManagerMu.Lock()
	x11RawHookIsEnabled = enabled
	x11ManagerMu.Unlock()
	return nil
}

func runLinuxKeyboardCall(call func(errorOut **C.char) C.int) error {
	var errorOut *C.char
	result := call(&errorOut)
	if errorOut != nil {
		defer C.free(unsafe.Pointer(errorOut))
	}
	if result == 0 {
		if errorOut != nil {
			return fmt.Errorf(C.GoString(errorOut))
		}
		return fmt.Errorf("native linux keyboard call failed")
	}
	return nil
}

//export keyboardHotkeyTriggeredCGO
func keyboardHotkeyTriggeredCGO(id C.int) {
	x11ManagerMu.Lock()
	callback := x11HotkeyCallbacks[int(id)]
	x11ManagerMu.Unlock()
	if callback == nil {
		return
	}

	util.Go(util.NewTraceContext(), fmt.Sprintf("global hotkey %d callback", int(id)), func() {
		callback()
	})
}

//export keyboardHookEventCGO
func keyboardHookEventCGO(eventKind C.int, keyCode C.uint, modifiers C.uint) C.int {
	key := linuxKeyCodeToKey(uint32(keyCode))
	event := RawKeyEvent{
		Key:           key,
		Character:     key.Character(),
		Modifiers:     Modifier(modifiers),
		NativeKeyCode: uint32(keyCode),
	}
	if int(eventKind) == 1 {
		event.Type = EventTypeKeyUp
	} else {
		event.Type = EventTypeKeyDown
	}

	x11ManagerMu.Lock()
	listeners := make([]RawKeyHandler, 0, len(x11RawKeyListeners))
	for _, listener := range x11RawKeyListeners {
		listeners = append(listeners, listener)
	}
	x11ManagerMu.Unlock()

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

func keyToLinuxKeyCode(key Key) (uint32, error) {
	switch key {
	case KeyA:
		return 0x0061, nil
	case KeyB:
		return 0x0062, nil
	case KeyC:
		return 0x0063, nil
	case KeyD:
		return 0x0064, nil
	case KeyE:
		return 0x0065, nil
	case KeyF:
		return 0x0066, nil
	case KeyG:
		return 0x0067, nil
	case KeyH:
		return 0x0068, nil
	case KeyI:
		return 0x0069, nil
	case KeyJ:
		return 0x006A, nil
	case KeyK:
		return 0x006B, nil
	case KeyL:
		return 0x006C, nil
	case KeyM:
		return 0x006D, nil
	case KeyN:
		return 0x006E, nil
	case KeyO:
		return 0x006F, nil
	case KeyP:
		return 0x0070, nil
	case KeyQ:
		return 0x0071, nil
	case KeyR:
		return 0x0072, nil
	case KeyS:
		return 0x0073, nil
	case KeyT:
		return 0x0074, nil
	case KeyU:
		return 0x0075, nil
	case KeyV:
		return 0x0076, nil
	case KeyW:
		return 0x0077, nil
	case KeyX:
		return 0x0078, nil
	case KeyY:
		return 0x0079, nil
	case KeyZ:
		return 0x007A, nil
	case Key0:
		return 0x0030, nil
	case Key1:
		return 0x0031, nil
	case Key2:
		return 0x0032, nil
	case Key3:
		return 0x0033, nil
	case Key4:
		return 0x0034, nil
	case Key5:
		return 0x0035, nil
	case Key6:
		return 0x0036, nil
	case Key7:
		return 0x0037, nil
	case Key8:
		return 0x0038, nil
	case Key9:
		return 0x0039, nil
	case KeySpace:
		return 0x0020, nil
	case KeyReturn:
		return 0xFF0D, nil
	case KeyEscape:
		return 0xFF1B, nil
	case KeyTab:
		return 0xFF09, nil
	case KeyDelete:
		return 0xFFFF, nil
	case KeyLeft:
		return 0xFF51, nil
	case KeyRight:
		return 0xFF53, nil
	case KeyUp:
		return 0xFF52, nil
	case KeyDown:
		return 0xFF54, nil
	case KeyF1:
		return 0xFFBE, nil
	case KeyF2:
		return 0xFFBF, nil
	case KeyF3:
		return 0xFFC0, nil
	case KeyF4:
		return 0xFFC1, nil
	case KeyF5:
		return 0xFFC2, nil
	case KeyF6:
		return 0xFFC3, nil
	case KeyF7:
		return 0xFFC4, nil
	case KeyF8:
		return 0xFFC5, nil
	case KeyF9:
		return 0xFFC6, nil
	case KeyF10:
		return 0xFFC7, nil
	case KeyF11:
		return 0xFFC8, nil
	case KeyF12:
		return 0xFFC9, nil
	case KeyCapsLock:
		return 0xFFE5, nil
	default:
		return 0, fmt.Errorf("unsupported Linux hotkey key: %d", key)
	}
}

func linuxKeyCodeToKey(code uint32) Key {
	switch code {
	case 0x0061:
		return KeyA
	case 0x0062:
		return KeyB
	case 0x0063:
		return KeyC
	case 0x0064:
		return KeyD
	case 0x0065:
		return KeyE
	case 0x0066:
		return KeyF
	case 0x0067:
		return KeyG
	case 0x0068:
		return KeyH
	case 0x0069:
		return KeyI
	case 0x006A:
		return KeyJ
	case 0x006B:
		return KeyK
	case 0x006C:
		return KeyL
	case 0x006D:
		return KeyM
	case 0x006E:
		return KeyN
	case 0x006F:
		return KeyO
	case 0x0070:
		return KeyP
	case 0x0071:
		return KeyQ
	case 0x0072:
		return KeyR
	case 0x0073:
		return KeyS
	case 0x0074:
		return KeyT
	case 0x0075:
		return KeyU
	case 0x0076:
		return KeyV
	case 0x0077:
		return KeyW
	case 0x0078:
		return KeyX
	case 0x0079:
		return KeyY
	case 0x007A:
		return KeyZ
	case 0x0030:
		return Key0
	case 0x0031:
		return Key1
	case 0x0032:
		return Key2
	case 0x0033:
		return Key3
	case 0x0034:
		return Key4
	case 0x0035:
		return Key5
	case 0x0036:
		return Key6
	case 0x0037:
		return Key7
	case 0x0038:
		return Key8
	case 0x0039:
		return Key9
	case 0x0020:
		return KeySpace
	case 0xFF0D:
		return KeyReturn
	case 0xFF1B:
		return KeyEscape
	case 0xFF09:
		return KeyTab
	case 0xFFFF:
		return KeyDelete
	case 0xFF51:
		return KeyLeft
	case 0xFF53:
		return KeyRight
	case 0xFF52:
		return KeyUp
	case 0xFF54:
		return KeyDown
	case 0xFFBE:
		return KeyF1
	case 0xFFBF:
		return KeyF2
	case 0xFFC0:
		return KeyF3
	case 0xFFC1:
		return KeyF4
	case 0xFFC2:
		return KeyF5
	case 0xFFC3:
		return KeyF6
	case 0xFFC4:
		return KeyF7
	case 0xFFC5:
		return KeyF8
	case 0xFFC6:
		return KeyF9
	case 0xFFC7:
		return KeyF10
	case 0xFFC8:
		return KeyF11
	case 0xFFC9:
		return KeyF12
	case 0xFFE5:
		return KeyCapsLock
	case 0xFFE3, 0xFFE4:
		return KeyCtrl
	case 0xFFE1, 0xFFE2:
		return KeyShift
	case 0xFFE9, 0xFFEA:
		return KeyAlt
	case 0xFFEB, 0xFFEC:
		return KeySuper
	default:
		return KeyUnknown
	}
}
