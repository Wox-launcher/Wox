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
	case "cmd", "command":
		return keyboard.ModifierSuper, keyboard.KeySuper, true
	default:
		return 0, keyboard.KeyUnknown, false
	}
}

func validateHotkeySpec(spec hotkeySpec) error {
	return nil
}
