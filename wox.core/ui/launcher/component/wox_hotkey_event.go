package component

import (
	"runtime"
	"strings"

	ui "wox/ui/runtime"
	"wox/util/keyboard"
)

// RecordedHotkey converts a normal local key-down into the shared Wox shortcut grammar.
// Recorder controls reserve navigation/clear keys separately; this function imposes no such policy.
func RecordedHotkey(event ui.KeyEvent) string {
	if !event.Down || event.Repeat || event.Composing || event.Key == ui.KeyUnknown {
		return ""
	}
	key, err := keyboard.KeyName(string(event.Key))
	if err != nil {
		return ""
	}
	parts := []string{}
	if event.Modifiers&ui.KeyModifierControl != 0 {
		parts = append(parts, "ctrl")
	}
	if event.Modifiers&ui.KeyModifierShift != 0 {
		parts = append(parts, "shift")
	}
	if event.Modifiers&ui.KeyModifierAlt != 0 {
		parts = append(parts, "alt")
	}
	if event.Modifiers&ui.KeyModifierMeta != 0 {
		modifier := "super"
		if runtime.GOOS == "darwin" {
			modifier = "cmd"
		}
		parts = append(parts, modifier)
	}
	value := strings.Join(append(parts, key), "+")
	parsed, ok := ui.ParseHotkey(value)
	if !ok || !parsed.Matches(event.Key, event.Modifiers) {
		return ""
	}
	return value
}
