//go:build linux

package hotkey

import (
	"context"
	"fmt"
	"strings"
	"wox/util/keyboard"
)

func parseModifierToken(token string) (keyboard.Modifier, keyboard.Key, bool) {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "ctrl", "control":
		return keyboard.ModifierCtrl, keyboard.KeyCtrl, true
	case "shift":
		return keyboard.ModifierShift, keyboard.KeyShift, true
	case "alt":
		return keyboard.ModifierAlt, keyboard.KeyAlt, true
	case "cmd", "command", "super", "win", "window":
		return keyboard.ModifierSuper, keyboard.KeySuper, true
	case "left_ctrl", "left control", "left_control":
		return keyboard.ModifierCtrl, keyboard.KeyLeftCtrl, true
	case "right_ctrl", "right control", "right_control":
		return keyboard.ModifierCtrl, keyboard.KeyRightCtrl, true
	case "left_shift":
		return keyboard.ModifierShift, keyboard.KeyLeftShift, true
	case "right_shift":
		return keyboard.ModifierShift, keyboard.KeyRightShift, true
	case "left_alt":
		return keyboard.ModifierAlt, keyboard.KeyLeftAlt, true
	case "right_alt":
		return keyboard.ModifierAlt, keyboard.KeyRightAlt, true
	case "left_cmd", "left_command", "left_super", "left_win":
		return keyboard.ModifierSuper, keyboard.KeyLeftSuper, true
	case "right_cmd", "right_command", "right_super", "right_win":
		return keyboard.ModifierSuper, keyboard.KeyRightSuper, true
	default:
		return 0, keyboard.KeyUnknown, false
	}
}

func validateHotkeySpec(spec hotkeySpec) error {
	if !keyboard.IsWaylandSession() {
		return nil
	}

	if spec.isDoubleModifier() {
		if !keyboard.IsEvdevReadAvailable() {
			return fmt.Errorf("double modifier hotkeys require evdev access on Wayland; add user to 'input' group")
		}
	}

	if spec.isCapsLockKey() {
		if !keyboard.IsEvdevReadAvailable() {
			return fmt.Errorf("CapsLock combo hotkeys require evdev access on Wayland; add user to 'input' group")
		}
	}

	return nil
}

func init() {
	// On Wayland, the XDG GlobalShortcuts portal does not have a concept of
	// "hotkey conflicts" — the portal always accepts the registration request and
	// the desktop environment resolves conflicts internally. Running the standard
	// register-probe-unregister cycle (used on X11/macOS/Windows) is harmful
	// here because:
	//   1. If the portal is unavailable (old GNOME/KDE), every probe returns an
	//      error and the UI reports every hotkey as "not available".
	//   2. Even when the portal is available, creating a session only to destroy
	//      it immediately can trigger DE confirmation dialogs or cause spurious
	//      D-Bus errors.
	// Instead, on Wayland we only validate the spec itself and always return true
	// for well-formed hotkeys.
	platformHotkeyAvailableCheck = func(_ context.Context, hotkeyStr string) (bool, bool) {
		if !keyboard.IsWaylandSession() {
			// Not a Wayland session; fall through to the standard X11 check.
			return false, false
		}

		hk := &Hotkey{}
		spec, parseErr := hk.parseCombineKey(hotkeyStr)
		if parseErr != nil {
			return false, true
		}
		if validateErr := validateHotkeySpec(spec); validateErr != nil {
			return false, true
		}
		return true, true
	}
}
