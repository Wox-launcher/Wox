package hotkey

import (
	"fmt"
	"strings"
)

// TriggerMode describes whether a saved hotkey binding fires on press or while held.
type TriggerMode string

const (
	TriggerPress TriggerMode = "press"
	TriggerHold  TriggerMode = "hold"
)

const holdBindingPrefix = "hold:"

// Binding is the parsed form of a persisted hotkey binding string.
type Binding struct {
	Trigger    TriggerMode
	CombineKey string
}

// ParseBinding keeps trigger semantics in the saved hotkey string.
// Bare hotkey strings remain press-triggered so existing settings keep working.
func ParseBinding(value string) (Binding, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return Binding{Trigger: TriggerPress}, nil
	}

	if strings.HasPrefix(trimmed, holdBindingPrefix) {
		combineKey := strings.TrimSpace(strings.TrimPrefix(trimmed, holdBindingPrefix))
		if combineKey == "" {
			return Binding{}, fmt.Errorf("hold hotkey binding requires a hotkey")
		}
		return Binding{Trigger: TriggerHold, CombineKey: combineKey}, nil
	}

	if strings.Contains(trimmed, ":") {
		return Binding{}, fmt.Errorf("unsupported hotkey binding: %s", trimmed)
	}
	return Binding{Trigger: TriggerPress, CombineKey: trimmed}, nil
}
