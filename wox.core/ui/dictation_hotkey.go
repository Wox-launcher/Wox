package ui

import (
	"fmt"
	"strings"
)

type dictationHotkeyTrigger string

const (
	dictationHotkeyTriggerPress dictationHotkeyTrigger = "press"
	dictationHotkeyTriggerHold  dictationHotkeyTrigger = "hold"
)

type dictationHotkeyBinding struct {
	trigger    dictationHotkeyTrigger
	combineKey string
}

// parseDictationHotkeyBinding keeps hold semantics in the saved binding string.
// Bare hotkey strings remain press-triggered so existing settings keep working.
func parseDictationHotkeyBinding(value string) (dictationHotkeyBinding, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return dictationHotkeyBinding{trigger: dictationHotkeyTriggerPress}, nil
	}

	const holdPrefix = "hold:"
	if strings.HasPrefix(trimmed, holdPrefix) {
		combineKey := strings.TrimSpace(strings.TrimPrefix(trimmed, holdPrefix))
		if combineKey == "" {
			return dictationHotkeyBinding{}, fmt.Errorf("hold dictation hotkey binding requires a hotkey")
		}
		return dictationHotkeyBinding{trigger: dictationHotkeyTriggerHold, combineKey: combineKey}, nil
	}

	if strings.Contains(trimmed, ":") {
		return dictationHotkeyBinding{}, fmt.Errorf("unsupported dictation hotkey binding: %s", trimmed)
	}
	return dictationHotkeyBinding{trigger: dictationHotkeyTriggerPress, combineKey: trimmed}, nil
}
