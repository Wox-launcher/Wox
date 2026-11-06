package launcher

import (
	"strings"

	component "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

// hotkeyMatches compares a configured normal hotkey with one physical key-down event.
func hotkeyMatches(hotkey string, event woxui.KeyEvent) bool {
	if !event.Down || event.Composing || event.Key == woxui.KeyUnknown || strings.TrimSpace(hotkey) == "" {
		return false
	}
	parsed, ok := woxui.ParseHotkey(hotkey)
	return ok && parsed.Matches(event.Key, event.Modifiers)
}

func formatHotkeyLabels(hotkey string) []string { return component.FormatHotkeyLabels(hotkey) }
