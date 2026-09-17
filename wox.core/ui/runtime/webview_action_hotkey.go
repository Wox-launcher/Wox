package woxui

import (
	"encoding/json"
	"fmt"
)

// webViewActionHotkeyJS assigns window.__woxActionHotkey for Darwin/Linux page scripts.
func webViewActionHotkeyJS(hotkey string) string {
	parsed, ok := ParseHotkey(hotkey)
	if !ok {
		return "window.__woxActionHotkey=null"
	}
	encoded, err := json.Marshal(string(parsed.Key))
	if err != nil {
		return "window.__woxActionHotkey=null"
	}
	return fmt.Sprintf(
		"window.__woxActionHotkey={key:%s,ctrl:%t,meta:%t,alt:%t,shift:%t}",
		encoded,
		parsed.Modifiers&KeyModifierControl != 0,
		parsed.Modifiers&KeyModifierMeta != 0,
		parsed.Modifiers&KeyModifierAlt != 0,
		parsed.Modifiers&KeyModifierShift != 0,
	)
}
