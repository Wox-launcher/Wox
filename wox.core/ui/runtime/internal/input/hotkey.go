package input

import (
	"wox/util/hotkey"
	"wox/util/keyboard"
)

// Hotkey is a normal key combination shared by launcher and embedded previews.
type Hotkey struct {
	Key       Key
	Modifiers KeyModifiers
}

// ParseHotkey adapts the shared shortcut parser to UI event names and modifier bits.
func ParseHotkey(value string) (Hotkey, bool) {
	combo, err := hotkey.ParseCombo(value)
	if err != nil {
		return Hotkey{}, false
	}
	key := Key(combo.Key)
	switch combo.Key {
	case "return":
		key = KeyEnter
	case "left":
		key = KeyArrowLeft
	case "right":
		key = KeyArrowRight
	case "up":
		key = KeyArrowUp
	case "down":
		key = KeyArrowDown
	case "pageup":
		key = KeyPageUp
	case "pagedown":
		key = KeyPageDown
	case "backquote":
		key = "`"
	}
	var modifiers KeyModifiers
	if combo.Modifiers&keyboard.ModifierCtrl != 0 {
		modifiers |= KeyModifierControl
	}
	if combo.Modifiers&keyboard.ModifierShift != 0 {
		modifiers |= KeyModifierShift
	}
	if combo.Modifiers&keyboard.ModifierAlt != 0 {
		modifiers |= KeyModifierAlt
	}
	if combo.Modifiers&keyboard.ModifierSuper != 0 {
		modifiers |= KeyModifierMeta
	}
	return Hotkey{Key: key, Modifiers: modifiers}, true
}

// Matches compares the key and the complete modifier set.
func (h Hotkey) Matches(key Key, modifiers KeyModifiers) bool {
	return h.Key != KeyUnknown && h.Key == key && h.Modifiers == modifiers
}
