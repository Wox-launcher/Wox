package keyboard

import (
	"fmt"
	"strings"
)

type Modifier uint32

const (
	// ModifierCtrl is Control on all supported platforms.
	ModifierCtrl Modifier = 1 << iota
	// ModifierShift is Shift on all supported platforms.
	ModifierShift
	// ModifierAlt is Alt on Windows and Linux, Option on macOS.
	ModifierAlt
	// ModifierSuper is the platform primary meta modifier:
	// Command on macOS, Win/Super on Windows and Linux.
	ModifierSuper
)

type Key uint16

const (
	KeyUnknown Key = iota
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
	KeySpace
	KeyReturn
	KeyEscape
	KeyTab
	KeyDelete
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyCapsLock
	// KeyBackquote represents the backquote/tilde key (` ~).
	KeyBackquote
	// KeyCtrl is Control on all supported platforms.
	KeyCtrl
	// KeyShift is Shift on all supported platforms.
	KeyShift
	// KeyAlt is Alt on Windows and Linux, Option on macOS.
	KeyAlt
	// KeySuper represents the platform primary meta key:
	// Command on macOS, Win/Super on Windows and Linux.
	KeySuper
)

func ParseKey(token string) (Key, error) {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "a":
		return KeyA, nil
	case "b":
		return KeyB, nil
	case "c":
		return KeyC, nil
	case "d":
		return KeyD, nil
	case "e":
		return KeyE, nil
	case "f":
		return KeyF, nil
	case "g":
		return KeyG, nil
	case "h":
		return KeyH, nil
	case "i":
		return KeyI, nil
	case "j":
		return KeyJ, nil
	case "k":
		return KeyK, nil
	case "l":
		return KeyL, nil
	case "m":
		return KeyM, nil
	case "n":
		return KeyN, nil
	case "o":
		return KeyO, nil
	case "p":
		return KeyP, nil
	case "q":
		return KeyQ, nil
	case "r":
		return KeyR, nil
	case "s":
		return KeyS, nil
	case "t":
		return KeyT, nil
	case "u":
		return KeyU, nil
	case "v":
		return KeyV, nil
	case "w":
		return KeyW, nil
	case "x":
		return KeyX, nil
	case "y":
		return KeyY, nil
	case "z":
		return KeyZ, nil
	case "0":
		return Key0, nil
	case "1":
		return Key1, nil
	case "2":
		return Key2, nil
	case "3":
		return Key3, nil
	case "4":
		return Key4, nil
	case "5":
		return Key5, nil
	case "6":
		return Key6, nil
	case "7":
		return Key7, nil
	case "8":
		return Key8, nil
	case "9":
		return Key9, nil
	case "space":
		return KeySpace, nil
	case "return", "enter":
		return KeyReturn, nil
	case "escape", "esc":
		return KeyEscape, nil
	case "tab":
		return KeyTab, nil
	case "delete", "del":
		return KeyDelete, nil
	case "left":
		return KeyLeft, nil
	case "right":
		return KeyRight, nil
	case "up":
		return KeyUp, nil
	case "down":
		return KeyDown, nil
	case "f1":
		return KeyF1, nil
	case "f2":
		return KeyF2, nil
	case "f3":
		return KeyF3, nil
	case "f4":
		return KeyF4, nil
	case "f5":
		return KeyF5, nil
	case "f6":
		return KeyF6, nil
	case "f7":
		return KeyF7, nil
	case "f8":
		return KeyF8, nil
	case "f9":
		return KeyF9, nil
	case "f10":
		return KeyF10, nil
	case "f11":
		return KeyF11, nil
	case "f12":
		return KeyF12, nil
	case "capslock", "caps_lock", "caps lock":
		return KeyCapsLock, nil
	case "backquote", "tilde", "~", "`":
		return KeyBackquote, nil
	default:
		return KeyUnknown, fmt.Errorf("invalid key: %s", token)
	}
}

func (k Key) Character() string {
	switch k {
	case KeyA:
		return "a"
	case KeyB:
		return "b"
	case KeyC:
		return "c"
	case KeyD:
		return "d"
	case KeyE:
		return "e"
	case KeyF:
		return "f"
	case KeyG:
		return "g"
	case KeyH:
		return "h"
	case KeyI:
		return "i"
	case KeyJ:
		return "j"
	case KeyK:
		return "k"
	case KeyL:
		return "l"
	case KeyM:
		return "m"
	case KeyN:
		return "n"
	case KeyO:
		return "o"
	case KeyP:
		return "p"
	case KeyQ:
		return "q"
	case KeyR:
		return "r"
	case KeyS:
		return "s"
	case KeyT:
		return "t"
	case KeyU:
		return "u"
	case KeyV:
		return "v"
	case KeyW:
		return "w"
	case KeyX:
		return "x"
	case KeyY:
		return "y"
	case KeyZ:
		return "z"
	case Key0:
		return "0"
	case Key1:
		return "1"
	case Key2:
		return "2"
	case Key3:
		return "3"
	case Key4:
		return "4"
	case Key5:
		return "5"
	case Key6:
		return "6"
	case Key7:
		return "7"
	case Key8:
		return "8"
	case Key9:
		return "9"
	case KeyBackquote:
		return "~"
	default:
		return ""
	}
}

type EventType int

const (
	EventTypeKeyDown EventType = iota
	EventTypeKeyUp
)

type RawKeyEvent struct {
	Type                         EventType
	Key                          Key
	Character                    string
	Modifiers                    Modifier
	NativeKeyCode                uint32
	NativeEventType              int
	NativeFlags                  uint64
	NativeCapsLockStateAvailable bool
	NativeCapsLockPressed        bool
}

type RawKeyHandler func(event RawKeyEvent) bool

type RawKeySubscription interface {
	Close() error
}

type HotkeyRegistration interface {
	Unregister() error
}

type GlobalHotkeySpec struct {
	Modifiers Modifier
	Key       Key
	Callback  func()
}

var registerGlobalHotkeysPlatform func(specs []GlobalHotkeySpec) (registration HotkeyRegistration, handled bool, err error)
var isWaylandGlobalShortcutsPortalAvailablePlatform func() bool

type globalHotkeyGroupRegistration struct {
	registrations []HotkeyRegistration
}

// IsWaylandGlobalShortcutsPortalAvailable reports whether the Wayland
// GlobalShortcuts portal is available as the active global-hotkey backend.
func IsWaylandGlobalShortcutsPortalAvailable() bool {
	if isWaylandGlobalShortcutsPortalAvailablePlatform == nil {
		return false
	}
	return isWaylandGlobalShortcutsPortalAvailablePlatform()
}

func RegisterGlobalHotkeys(specs []GlobalHotkeySpec) (HotkeyRegistration, error) {
	if registerGlobalHotkeysPlatform != nil {
		if registration, handled, err := registerGlobalHotkeysPlatform(specs); handled {
			return registration, err
		}
	}

	group := &globalHotkeyGroupRegistration{}
	for _, spec := range specs {
		registration, err := RegisterGlobalHotkey(spec.Modifiers, spec.Key, spec.Callback)
		if err != nil {
			_ = group.Unregister()
			return nil, err
		}
		group.registrations = append(group.registrations, registration)
	}
	return group, nil
}

func (g *globalHotkeyGroupRegistration) Unregister() error {
	if g == nil {
		return nil
	}

	var firstErr error
	for _, registration := range g.registrations {
		if registration == nil {
			continue
		}
		if err := registration.Unregister(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	g.registrations = nil
	return firstErr
}
