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

// holdModifierFamily returns the generic modifier that a left/right or generic
// modifier belongs to.
func holdModifierFamily(key keyboard.Key) (keyboard.Key, bool) {
	switch key {
	case keyboard.KeyCtrl, keyboard.KeyLeftCtrl, keyboard.KeyRightCtrl:
		return keyboard.KeyCtrl, true
	case keyboard.KeyShift, keyboard.KeyLeftShift, keyboard.KeyRightShift:
		return keyboard.KeyShift, true
	case keyboard.KeyAlt, keyboard.KeyLeftAlt, keyboard.KeyRightAlt:
		return keyboard.KeyAlt, true
	case keyboard.KeySuper, keyboard.KeyLeftSuper, keyboard.KeyRightSuper:
		return keyboard.KeySuper, true
	default:
		return 0, false
	}
}

// isGenericHoldModifier reports whether key is a side-agnostic modifier.
func isGenericHoldModifier(key keyboard.Key) bool {
	switch key {
	case keyboard.KeyCtrl, keyboard.KeyShift, keyboard.KeyAlt, keyboard.KeySuper:
		return true
	default:
		return false
	}
}

// holdModifierKeysRelated reports whether two keys should be treated as the
// same hold-modifier slot. Left and right stay distinct. A generic VK_SHIFT
// event may satisfy a left_shift or right_shift slot because Windows sometimes
// omits the side; left_shift never matches right_shift.
func holdModifierKeysRelated(a keyboard.Key, b keyboard.Key) bool {
	if a == b {
		return true
	}
	if isGenericHoldModifier(a) {
		family, ok := holdModifierFamily(b)
		return ok && family == a
	}
	if isGenericHoldModifier(b) {
		family, ok := holdModifierFamily(a)
		return ok && family == b
	}
	return false
}

func containsRelatedHoldModifierKey(keys []keyboard.Key, target keyboard.Key) bool {
	for _, key := range keys {
		if holdModifierKeysRelated(key, target) {
			return true
		}
	}
	return false
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
