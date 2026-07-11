package hotkey

import "wox/util/keyboard"

// modifierKeyMatchesRawEvent reports whether a registered modifier key should
// handle a raw key event. Raw listeners often emit left/right-specific
// modifiers, while settings such as "ctrl+ctrl" register generic modifiers.
func modifierKeyMatchesRawEvent(registeredKey keyboard.Key, eventKey keyboard.Key) bool {
	if registeredKey == eventKey {
		return true
	}

	switch registeredKey {
	case keyboard.KeyCtrl:
		return eventKey == keyboard.KeyLeftCtrl || eventKey == keyboard.KeyRightCtrl
	case keyboard.KeyShift:
		return eventKey == keyboard.KeyLeftShift || eventKey == keyboard.KeyRightShift
	case keyboard.KeyAlt:
		return eventKey == keyboard.KeyLeftAlt || eventKey == keyboard.KeyRightAlt
	case keyboard.KeySuper:
		return eventKey == keyboard.KeyLeftSuper || eventKey == keyboard.KeyRightSuper
	default:
		return false
	}
}

// modifierKeyLogLabel returns a readable label for generic and specific modifiers.
func modifierKeyLogLabel(key keyboard.Key) string {
	if label := key.Character(); label != "" {
		return label
	}

	switch key {
	case keyboard.KeyCtrl:
		return "ctrl"
	case keyboard.KeyShift:
		return "shift"
	case keyboard.KeyAlt:
		return "alt"
	case keyboard.KeySuper:
		return "cmd"
	default:
		return ""
	}
}
