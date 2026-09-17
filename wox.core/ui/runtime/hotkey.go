package woxui

import "strings"

// Hotkey is a normal key combination shared by launcher and embedded previews.
type Hotkey struct {
	Key       Key
	Modifiers KeyModifiers
}

// ParseHotkey normalizes recorded key names and rejects unsupported modifier syntax.
func ParseHotkey(hotkey string) (Hotkey, bool) {
	hotkey = strings.TrimSpace(hotkey)
	if hotkey == "" {
		return Hotkey{}, false
	}
	parts := strings.Split(strings.ToLower(hotkey), "+")
	key := strings.TrimSpace(parts[len(parts)-1])
	switch key {
	case "", "ctrl", "control", "cmd", "command", "meta", "win", "super", "alt", "option", "shift", "capslock":
		return Hotkey{}, false
	case "return":
		key = string(KeyEnter)
	case "left":
		key = string(KeyArrowLeft)
	case "right":
		key = string(KeyArrowRight)
	case "up":
		key = string(KeyArrowUp)
	case "down":
		key = string(KeyArrowDown)
	case "pageup":
		key = string(KeyPageUp)
	case "pagedown":
		key = string(KeyPageDown)
	case "esc":
		key = string(KeyEscape)
	}
	var modifiers KeyModifiers
	for _, part := range parts[:len(parts)-1] {
		switch strings.TrimSpace(part) {
		case "ctrl", "control":
			modifiers |= KeyModifierControl
		case "cmd", "command", "meta", "win", "super":
			modifiers |= KeyModifierMeta
		case "alt", "option":
			modifiers |= KeyModifierAlt
		case "shift":
			modifiers |= KeyModifierShift
		default:
			return Hotkey{}, false
		}
	}
	return Hotkey{Key: Key(key), Modifiers: modifiers}, true
}

// Matches compares the key and the complete modifier set.
func (h Hotkey) Matches(key Key, modifiers KeyModifiers) bool {
	return h.Key != KeyUnknown && h.Key == key && h.Modifiers == modifiers
}
