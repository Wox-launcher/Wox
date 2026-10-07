package hotkey

import (
	"fmt"
	"strings"
	"wox/util/keyboard"
)

// Combo describes a normal shortcut independently of native registration or UI event types.
type Combo struct {
	Key       string
	Modifiers keyboard.Modifier
}

// parsedTokens keeps syntax separate from native backend support and special trigger policies.
type parsedTokens struct {
	Combo
	capsLock     bool
	modifierKeys []keyboard.Key
}

// ParseCombo accepts ordinary shortcuts while leaving CapsLock, hold, and double-modifier triggers to registration.
func ParseCombo(value string) (Combo, error) {
	parsed, err := parseTokens(value)
	if err != nil {
		return Combo{}, err
	}
	if parsed.Key == "" || parsed.Key == "capslock" || parsed.capsLock {
		return Combo{}, fmt.Errorf("not a normal hotkey: %s", value)
	}
	for _, modifier := range parsed.modifierKeys {
		// UI events contain aggregate modifier bits and cannot enforce a left/right physical chord.
		if isSpecificModifierKey(modifier) {
			return Combo{}, fmt.Errorf("side-specific modifier requires native registration: %s", value)
		}
	}
	return parsed.Combo, nil
}

// parseTokens is the shared syntax parser for local shortcuts and global hotkey registration.
func parseTokens(value string) (parsedTokens, error) {
	tokens := strings.Split(value, "+")
	var parsed parsedTokens
	for _, token := range tokens {
		if isCapsLockToken(strings.ToLower(strings.TrimSpace(token))) && len(tokens) > 1 {
			parsed.capsLock = true
			continue
		}
		modifier, key, ok := parseModifierToken(token)
		if ok {
			parsed.Modifiers |= modifier
			parsed.modifierKeys = append(parsed.modifierKeys, key)
			continue
		}
		name, err := keyboard.KeyName(token)
		if err != nil {
			return parsedTokens{}, err
		}
		if parsed.Key != "" {
			return parsedTokens{}, fmt.Errorf("multiple keys in hotkey: %s", value)
		}
		parsed.Key = name
	}
	return parsed, nil
}

// parseModifierToken accepts persisted aliases consistently across registration and local UI matching.
func parseModifierToken(token string) (keyboard.Modifier, keyboard.Key, bool) {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "ctrl", "control":
		return keyboard.ModifierCtrl, keyboard.KeyCtrl, true
	case "shift":
		return keyboard.ModifierShift, keyboard.KeyShift, true
	case "alt", "option":
		return keyboard.ModifierAlt, keyboard.KeyAlt, true
	case "cmd", "command", "meta", "win", "window", "super":
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
