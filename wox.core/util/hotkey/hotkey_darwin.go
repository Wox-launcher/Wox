//go:build darwin

package hotkey

import (
	"strings"
	"wox/util/keyboard"
)

func parseModifierToken(token string) (keyboard.Modifier, keyboard.Key, bool) {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "ctrl", "control":
		return keyboard.ModifierCtrl, keyboard.KeyCtrl, true
	case "shift":
		return keyboard.ModifierShift, keyboard.KeyShift, true
	case "option", "alt":
		return keyboard.ModifierAlt, keyboard.KeyAlt, true
	case "cmd", "command", "win", "window", "super":
		return keyboard.ModifierSuper, keyboard.KeySuper, true
	case "left_ctrl", "left control", "left_control":
		return keyboard.ModifierCtrl, keyboard.KeyLeftCtrl, true
	case "right_ctrl", "right control", "right_control":
		return keyboard.ModifierCtrl, keyboard.KeyRightCtrl, true
	case "left_shift":
		return keyboard.ModifierShift, keyboard.KeyLeftShift, true
	case "right_shift":
		return keyboard.ModifierShift, keyboard.KeyRightShift, true
	case "left_alt", "left_option":
		return keyboard.ModifierAlt, keyboard.KeyLeftAlt, true
	case "right_alt", "right_option":
		return keyboard.ModifierAlt, keyboard.KeyRightAlt, true
	case "left_cmd", "left_command", "left_win", "left_super":
		return keyboard.ModifierSuper, keyboard.KeyLeftSuper, true
	case "right_cmd", "right_command", "right_win", "right_super":
		return keyboard.ModifierSuper, keyboard.KeyRightSuper, true
	default:
		return 0, keyboard.KeyUnknown, false
	}
}

func validateHotkeySpec(spec hotkeySpec) error {
	return nil
}
