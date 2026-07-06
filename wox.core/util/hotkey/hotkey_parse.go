package hotkey

import (
	"fmt"
	"strings"
	"wox/util/keyboard"

	"github.com/samber/lo"
)

type hotkeySpec struct {
	capsLock          bool
	modifiers         keyboard.Modifier
	key               keyboard.Key
	doubleModifierKey keyboard.Key
	// holdModifierKey is set when the hotkey is a single left/right modifier
	// key (e.g. "left_cmd"). In this mode no system hotkey is registered;
	// a raw key listener handles both press and release.
	holdModifierKey keyboard.Key
}

func (s hotkeySpec) isCapsLockKey() bool {
	return s.capsLock
}

func (s hotkeySpec) isDoubleModifier() bool {
	return s.doubleModifierKey != keyboard.KeyUnknown
}

func (s hotkeySpec) isHoldModifier() bool {
	return s.holdModifierKey != keyboard.KeyUnknown
}

func (h *Hotkey) parseCombineKey(combineKey string) (hotkeySpec, error) {
	tokens := lo.Map(strings.Split(combineKey, "+"), func(item string, index int) string {
		return strings.TrimSpace(item)
	})

	var spec hotkeySpec
	var modifierKeys []keyboard.Key

	for _, token := range tokens {
		normalizedToken := strings.ToLower(strings.TrimSpace(token))
		if isCapsLockToken(normalizedToken) && len(tokens) > 1 {
			spec.capsLock = true
			continue
		}

		modifier, modifierKey, ok := parseModifierToken(token)
		if ok {
			spec.modifiers |= modifier
			modifierKeys = append(modifierKeys, modifierKey)
			continue
		}

		key, err := keyboard.ParseKey(token)
		if err != nil {
			return hotkeySpec{}, err
		}
		if spec.key != keyboard.KeyUnknown {
			return hotkeySpec{}, fmt.Errorf("multiple keys in hotkey: %s", combineKey)
		}
		spec.key = key
	}

	if spec.key == keyboard.KeyUnknown {
		if spec.capsLock {
			return hotkeySpec{}, fmt.Errorf("missing key in caps lock hotkey: %s", combineKey)
		}
		if len(modifierKeys) == 2 && modifierKeys[0] == modifierKeys[1] {
			spec.doubleModifierKey = modifierKeys[0]
			return spec, nil
		}
		// A single left/right specific modifier key (e.g. "left_cmd") is
		// treated as a hold-modifier hotkey. No system hotkey is registered;
		// a raw key listener handles press and release.
		if len(modifierKeys) == 1 && isSpecificModifierKey(modifierKeys[0]) {
			spec.holdModifierKey = modifierKeys[0]
			return spec, nil
		}
		return hotkeySpec{}, fmt.Errorf("missing key in hotkey: %s", combineKey)
	}

	if spec.capsLock && (spec.modifiers != 0 || len(modifierKeys) > 0) {
		return hotkeySpec{}, fmt.Errorf("caps lock hotkey does not support extra modifiers: %s", combineKey)
	}

	return spec, nil
}

func isCapsLockToken(token string) bool {
	return token == "capslock" || token == "caps_lock" || token == "caps lock"
}

// isSpecificModifierKey reports whether the key is a left/right specific
// modifier key (e.g. KeyLeftCtrl, KeyRightSuper) rather than a generic
// modifier key (e.g. KeyCtrl, KeySuper).
func isSpecificModifierKey(key keyboard.Key) bool {
	switch key {
	case keyboard.KeyLeftCtrl, keyboard.KeyRightCtrl,
		keyboard.KeyLeftShift, keyboard.KeyRightShift,
		keyboard.KeyLeftAlt, keyboard.KeyRightAlt,
		keyboard.KeyLeftSuper, keyboard.KeyRightSuper:
		return true
	default:
		return false
	}
}

func IsCapsLockHotkeyString(combineKey string) bool {
	spec, err := (&Hotkey{}).parseCombineKey(combineKey)
	return err == nil && spec.isCapsLockKey()
}

// IsDoubleModifierHotkeyString reports whether combineKey is a double-modifier
// hotkey (e.g. "ctrl+ctrl", "shift+shift"). Used by callers (such as the Linux
// doctor checks) that need to detect special hotkeys without registering them.
func IsDoubleModifierHotkeyString(combineKey string) bool {
	spec, err := (&Hotkey{}).parseCombineKey(combineKey)
	return err == nil && spec.isDoubleModifier()
}
