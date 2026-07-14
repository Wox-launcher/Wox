//go:build darwin

package keyboard

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework Carbon -framework ApplicationServices -framework IOKit
#include <stdlib.h>

int woxDarwinEnsureKeyboardReady(char **errorOut);
int woxDarwinRegisterHotkey(int id, unsigned int modifiers, unsigned int keyCode, char **errorOut);
int woxDarwinUnregisterHotkey(int id, char **errorOut);
int woxDarwinSetRawKeyboardHookEnabled(int enabled, char **errorOut);
int woxDarwinHasRawKeyboardAccess(void);
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
	"wox/util"
	"wox/util/mainthread"
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
	managerMu            sync.Mutex
	nextHotkeyID         = 1
	nextListenerID       = 1
	hotkeyCallbacks      = map[int]func(){}
	rawKeyListeners      = map[int]RawKeyHandler{}
	rawHookIsEnabled     bool
	rawHookIsDeferred    bool
	rawHookAccessKnown   bool
	rawHookAccessGranted bool
)

func init() {
	reconcileRawKeyListenerAccessWithPermissionStatusPlatform = reconcileRawKeyListenerAccessWithPermissionStatusDarwin
}

func RegisterGlobalHotkey(modifiers Modifier, key Key, callback func()) (HotkeyRegistration, error) {
	keyCode, err := keyToDarwinKeyCode(key)
	if err != nil {
		return nil, err
	}

	managerMu.Lock()
	id := nextHotkeyID
	nextHotkeyID++
	managerMu.Unlock()

	if err := ensureNativeKeyboardReady(); err != nil {
		return nil, err
	}

	if err := runDarwinKeyboardCall(func(errorOut **C.char) C.int {
		return C.woxDarwinRegisterHotkey(C.int(id), C.uint(modifiers), C.uint(keyCode), errorOut)
	}); err != nil {
		return nil, err
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

		unregisterErr = runDarwinKeyboardCall(func(errorOut **C.char) C.int {
			return C.woxDarwinUnregisterHotkey(C.int(r.id), errorOut)
		})
	})
	return unregisterErr
}

func AddRawKeyListener(handler RawKeyHandler) (RawKeySubscription, error) {
	if handler == nil {
		return nil, fmt.Errorf("raw key handler is required")
	}

	if err := ensureNativeKeyboardReady(); err != nil {
		return nil, err
	}

	managerMu.Lock()
	id := nextListenerID
	nextListenerID++
	rawKeyListeners[id] = handler
	managerMu.Unlock()

	// Keep the subscription pending when macOS permissions are missing. The
	// listener becomes active after the explicit permission flow reconciles the
	// backend, without forcing every subsystem to register again.
	if err := reconcileRawKeyListenerAccessDarwin(); err != nil {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("raw keyboard listener remains pending: %s", err.Error()))
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
		managerMu.Unlock()

		closeErr = reconcileRawKeyListenerAccessDarwin()
	})
	return closeErr
}

func ensureNativeKeyboardReady() error {
	return runDarwinKeyboardCall(func(errorOut **C.char) C.int {
		return C.woxDarwinEnsureKeyboardReady(errorOut)
	})
}

func setRawHookEnabled(enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}

	if err := runDarwinKeyboardCall(func(errorOut **C.char) C.int {
		return C.woxDarwinSetRawKeyboardHookEnabled(C.int(value), errorOut)
	}); err != nil {
		return err
	}

	managerMu.Lock()
	rawHookIsEnabled = enabled
	managerMu.Unlock()
	return nil
}

// reconcileRawKeyListenerAccessDarwin starts privacy-sensitive keyboard hooks only after Accessibility is granted.
func reconcileRawKeyListenerAccessDarwin() error {
	managerMu.Lock()
	accessKnown := rawHookAccessKnown
	accessGranted := rawHookAccessGranted
	managerMu.Unlock()
	// macOS can cache the pre-grant denial in the long-lived core process, so a
	// later recorder must reuse the most recent isolated permission probe.
	if !accessKnown {
		accessGranted = C.woxDarwinHasRawKeyboardAccess() != 0
	}
	return reconcileRawKeyListenerAccessWithPermissionStatusDarwin(accessGranted)
}

// reconcileRawKeyListenerAccessWithPermissionStatusDarwin uses the fresh probe result instead of the process-cached preflight APIs.
func reconcileRawKeyListenerAccessWithPermissionStatusDarwin(accessibilityGranted bool) error {
	managerMu.Lock()
	rawHookAccessKnown = true
	rawHookAccessGranted = accessibilityGranted
	hasListeners := len(rawKeyListeners) > 0
	isEnabled := rawHookIsEnabled
	isDeferred := rawHookIsDeferred
	managerMu.Unlock()

	if !hasListeners {
		if isEnabled {
			if err := setRawHookEnabled(false); err != nil {
				return err
			}
		}
		managerMu.Lock()
		rawHookIsDeferred = false
		managerMu.Unlock()
		return nil
	}

	if !accessibilityGranted {
		if isEnabled {
			if err := setRawHookEnabled(false); err != nil {
				return err
			}
		}
		if !isDeferred {
			util.GetLogger().Info(util.NewTraceContext(), "raw keyboard listener deferred until Accessibility permission is granted")
		}
		managerMu.Lock()
		rawHookIsDeferred = true
		managerMu.Unlock()
		return nil
	}

	if !isEnabled {
		if err := setRawHookEnabled(true); err != nil {
			return err
		}
		util.GetLogger().Info(util.NewTraceContext(), "raw keyboard listener enabled after permission reconciliation")
	}
	managerMu.Lock()
	rawHookIsDeferred = false
	managerMu.Unlock()
	return nil
}

func runDarwinKeyboardCall(call func(errorOut **C.char) C.int) error {
	var callErr error
	mainthread.Call(func() {
		var errorOut *C.char
		result := call(&errorOut)
		if errorOut != nil {
			defer C.free(unsafe.Pointer(errorOut))
		}
		if result == 0 {
			if errorOut != nil {
				callErr = fmt.Errorf("%s", C.GoString(errorOut))
			} else {
				callErr = fmt.Errorf("native keyboard call failed")
			}
		}
	})
	return callErr
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
func keyboardHookEventCGO(eventKind C.int, keyCode C.uint, modifiers C.uint, character C.uint, nativeEventType C.int, nativeFlags C.ulonglong, nativeCapsLockStateAvailable C.int, nativeCapsLockPressed C.int) C.int {
	key := darwinKeyCodeToKey(uint32(keyCode))
	characterValue := key.Character()
	if character != 0 {
		// Prefer the layout-aware character from Cocoa over the US-layout fallback
		// derived from the hardware key code.
		characterValue = string(rune(character))
	}
	event := RawKeyEvent{
		Key:                          key,
		Character:                    characterValue,
		Modifiers:                    Modifier(modifiers),
		NativeKeyCode:                uint32(keyCode),
		NativeEventType:              int(nativeEventType),
		NativeFlags:                  uint64(nativeFlags),
		NativeCapsLockStateAvailable: nativeCapsLockStateAvailable != 0,
		NativeCapsLockPressed:        nativeCapsLockPressed != 0,
	}
	if int(eventKind) == 1 {
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

func keyToDarwinKeyCode(key Key) (uint32, error) {
	switch key {
	case KeyA:
		return 0, nil
	case KeyB:
		return 11, nil
	case KeyC:
		return 8, nil
	case KeyD:
		return 2, nil
	case KeyE:
		return 14, nil
	case KeyF:
		return 3, nil
	case KeyG:
		return 5, nil
	case KeyH:
		return 4, nil
	case KeyI:
		return 34, nil
	case KeyJ:
		return 38, nil
	case KeyK:
		return 40, nil
	case KeyL:
		return 37, nil
	case KeyM:
		return 46, nil
	case KeyN:
		return 45, nil
	case KeyO:
		return 31, nil
	case KeyP:
		return 35, nil
	case KeyQ:
		return 12, nil
	case KeyR:
		return 15, nil
	case KeyS:
		return 1, nil
	case KeyT:
		return 17, nil
	case KeyU:
		return 32, nil
	case KeyV:
		return 9, nil
	case KeyW:
		return 13, nil
	case KeyX:
		return 7, nil
	case KeyY:
		return 16, nil
	case KeyZ:
		return 6, nil
	case Key0:
		return 29, nil
	case Key1:
		return 18, nil
	case Key2:
		return 19, nil
	case Key3:
		return 20, nil
	case Key4:
		return 21, nil
	case Key5:
		return 23, nil
	case Key6:
		return 22, nil
	case Key7:
		return 26, nil
	case Key8:
		return 28, nil
	case Key9:
		return 25, nil
	case KeySpace:
		return 49, nil
	case KeyReturn:
		return 36, nil
	case KeyEscape:
		return 53, nil
	case KeyTab:
		return 48, nil
	case KeyDelete:
		return 51, nil
	case KeyLeft:
		return 123, nil
	case KeyRight:
		return 124, nil
	case KeyDown:
		return 125, nil
	case KeyUp:
		return 126, nil
	case KeyF1:
		return 122, nil
	case KeyF2:
		return 120, nil
	case KeyF3:
		return 99, nil
	case KeyF4:
		return 118, nil
	case KeyF5:
		return 96, nil
	case KeyF6:
		return 97, nil
	case KeyF7:
		return 98, nil
	case KeyF8:
		return 100, nil
	case KeyF9:
		return 101, nil
	case KeyF10:
		return 109, nil
	case KeyF11:
		return 103, nil
	case KeyF12:
		return 111, nil
	case KeyCapsLock:
		return 57, nil
	case KeyBackquote:
		return 50, nil
	default:
		return 0, fmt.Errorf("unsupported macOS hotkey key: %d", key)
	}
}

func darwinKeyCodeToKey(keyCode uint32) Key {
	switch keyCode {
	case 0:
		return KeyA
	case 11:
		return KeyB
	case 8:
		return KeyC
	case 2:
		return KeyD
	case 14:
		return KeyE
	case 3:
		return KeyF
	case 5:
		return KeyG
	case 4:
		return KeyH
	case 34:
		return KeyI
	case 38:
		return KeyJ
	case 40:
		return KeyK
	case 37:
		return KeyL
	case 46:
		return KeyM
	case 45:
		return KeyN
	case 31:
		return KeyO
	case 35:
		return KeyP
	case 12:
		return KeyQ
	case 15:
		return KeyR
	case 1:
		return KeyS
	case 17:
		return KeyT
	case 32:
		return KeyU
	case 9:
		return KeyV
	case 13:
		return KeyW
	case 7:
		return KeyX
	case 16:
		return KeyY
	case 6:
		return KeyZ
	case 29:
		return Key0
	case 18:
		return Key1
	case 19:
		return Key2
	case 20:
		return Key3
	case 21:
		return Key4
	case 23:
		return Key5
	case 22:
		return Key6
	case 26:
		return Key7
	case 28:
		return Key8
	case 25:
		return Key9
	case 49:
		return KeySpace
	case 36:
		return KeyReturn
	case 53:
		return KeyEscape
	case 48:
		return KeyTab
	case 51:
		return KeyDelete
	case 123:
		return KeyLeft
	case 124:
		return KeyRight
	case 125:
		return KeyDown
	case 126:
		return KeyUp
	case 122:
		return KeyF1
	case 120:
		return KeyF2
	case 99:
		return KeyF3
	case 118:
		return KeyF4
	case 96:
		return KeyF5
	case 97:
		return KeyF6
	case 98:
		return KeyF7
	case 100:
		return KeyF8
	case 101:
		return KeyF9
	case 109:
		return KeyF10
	case 103:
		return KeyF11
	case 111:
		return KeyF12
	case 57:
		return KeyCapsLock
	case 55:
		return KeyLeftSuper
	case 54:
		return KeyRightSuper
	case 56:
		return KeyLeftShift
	case 60:
		return KeyRightShift
	case 58:
		return KeyLeftAlt
	case 61:
		return KeyRightAlt
	case 59:
		return KeyLeftCtrl
	case 62:
		return KeyRightCtrl
	case 50:
		return KeyBackquote
	default:
		return KeyUnknown
	}
}
