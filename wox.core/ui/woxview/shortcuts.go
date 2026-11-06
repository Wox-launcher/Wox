package woxview

import (
	"fmt"
	"runtime"
	"strings"

	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// Shortcut declares a normal local key combination using the shared Wox hotkey grammar.
type Shortcut struct {
	Key     string   `json:"key"`
	OnPress *Handler `json:"onPress"`
	combo   ui.Hotkey
}

// parseShortcut adds the portable mod alias before invoking the existing UI hotkey parser.
func parseShortcut(value, platform string) (ui.Hotkey, bool) {
	parts := strings.Split(value, "+")
	for i, part := range parts {
		if strings.EqualFold(strings.TrimSpace(part), "mod") {
			parts[i] = "ctrl"
			if platform == "darwin" {
				parts[i] = "cmd"
			}
		}
	}
	return ui.ParseHotkey(strings.Join(parts, "+"))
}

// validateShortcuts rejects ambiguous aliases within one scope and precomputes matching off the UI thread.
func validateShortcuts(shortcuts []Shortcut) error {
	if len(shortcuts) > 128 {
		return fmt.Errorf("shortcuts exceeds 128 entries")
	}
	seen := map[ui.Hotkey]bool{}
	for i := range shortcuts {
		item := &shortcuts[i]
		combo, ok := parseShortcut(item.Key, runtime.GOOS)
		if !ok {
			return fmt.Errorf("shortcuts[%d]: invalid key %q", i, item.Key)
		}
		if seen[combo] {
			return fmt.Errorf("shortcuts[%d]: duplicate key %q", i, item.Key)
		}
		if item.OnPress == nil {
			return fmt.Errorf("shortcuts[%d].onPress is required", i)
		}
		seen[combo] = true
		item.combo = combo
	}
	return nil
}

// shortcuts keeps all matching local; only the chosen producer callback crosses the event boundary.
func (b *builder) shortcuts(id string, items []Shortcut, window bool, child widget.Widget) widget.Widget {
	taps := make([]func(), len(items))
	for i, item := range items {
		taps[i] = b.tap(item.OnPress)
	}
	return widget.ShortcutScope{Key: widget.Key(id + "/shortcuts"), Window: window, Child: child, OnKey: func(e ui.KeyEvent) bool {
		for i, item := range items {
			if item.combo.Matches(e.Key, e.Modifiers) {
				if taps[i] != nil {
					taps[i]()
				}
				return true
			}
		}
		return false
	}}
}
