package hotkey

import "fmt"

type hotkeyKind string

const (
	hotkeyKindNormalCombo    hotkeyKind = "normalCombo"
	hotkeyKindDoubleModifier hotkeyKind = "doubleModifier"
	hotkeyKindCapsLockCombo  hotkeyKind = "capsLockCombo"
	hotkeyKindHoldModifier   hotkeyKind = "holdModifier"
	hotkeyKindPressModifier  hotkeyKind = "pressModifier"
	hotkeyKindUnknown        hotkeyKind = ""
)

type registerOptions struct {
	allowModifierPress bool
}

// resolveHotkeyKind turns a parsed hotkey shape into the runtime behavior
// allowed by the current registration intent.
func resolveHotkeyKind(spec hotkeySpec, hasRelease bool, options registerOptions) (hotkeyKind, error) {
	if hasRelease {
		if spec.isModifierChord() {
			return hotkeyKindHoldModifier, nil
		}
		return hotkeyKindUnknown, fmt.Errorf("release callbacks are only supported for modifier hold hotkeys")
	}

	if spec.isModifierChord() {
		if options.allowModifierPress {
			return hotkeyKindPressModifier, nil
		}
		return hotkeyKindUnknown, fmt.Errorf("modifier-only hotkeys require explicit modifier press support")
	}

	if spec.isDoubleModifier() {
		return hotkeyKindDoubleModifier, nil
	}
	if spec.isCapsLockKey() {
		return hotkeyKindCapsLockCombo, nil
	}
	return hotkeyKindNormalCombo, nil
}
