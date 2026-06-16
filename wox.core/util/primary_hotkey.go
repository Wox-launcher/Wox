package util

import "strings"

// PrimaryModifier returns the platform's primary modifier for Wox-defined
// in-app shortcuts. Global activation hotkeys keep their explicit defaults.
func PrimaryModifier() string {
	if IsMacOS() {
		return "cmd"
	}
	return "ctrl"
}

// PrimaryHotkey builds a real hotkey string from the primary modifier and a
// suffix such as "f", "shift+f", or "enter".
func PrimaryHotkey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return PrimaryModifier()
	}
	return PrimaryModifier() + "+" + key
}
