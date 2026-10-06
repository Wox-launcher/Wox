package woxui

import "runtime"

// IsWindowShortcut identifies reserved settings and close commands with exact platform modifiers.
func IsWindowShortcut(key Key, modifiers KeyModifiers) bool {
	return isWindowShortcut(key, modifiers, runtime.GOOS)
}

func isWindowShortcut(key Key, modifiers KeyModifiers, platform string) bool {
	primary := KeyModifierControl
	if platform == "darwin" {
		primary = KeyModifierMeta
	}
	return modifiers == primary && (key == Key(",") || key == Key("w"))
}
