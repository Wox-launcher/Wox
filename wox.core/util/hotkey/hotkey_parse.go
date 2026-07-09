package hotkey

import (
	"fmt"
	"sort"
	"strings"
	"wox/util/keyboard"

	"github.com/samber/lo"
)

type hotkeySpec struct {
	capsLock          bool
	modifiers         keyboard.Modifier
	key               keyboard.Key
	doubleModifierKey keyboard.Key
	// modifierChordKeys is set when the hotkey string is a pure left/right
	// modifier chord. The registration intent decides whether it becomes a
	// holdModifier or pressModifier runtime hotkey.
	modifierChordKeys []keyboard.Key
}

func (s hotkeySpec) isCapsLockKey() bool {
	return s.capsLock
}

func (s hotkeySpec) isDoubleModifier() bool {
	return s.doubleModifierKey != keyboard.KeyUnknown
}

func (s hotkeySpec) isModifierChord() bool {
	return len(s.modifierChordKeys) > 0
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
		// One or two left/right specific modifier keys (e.g. "left_cmd" or
		// "left_shift+left_cmd") are parsed as a pure modifier chord. The
		// registration intent decides whether this chord is hold or press.
		if isHoldModifierKeyChord(modifierKeys) {
			spec.modifierChordKeys = canonicalHoldModifierKeys(modifierKeys)
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

func isHoldModifierKeyChord(keys []keyboard.Key) bool {
	if len(keys) == 0 || len(keys) > 2 {
		return false
	}

	seen := map[keyboard.Key]bool{}
	for _, key := range keys {
		if !isSpecificModifierKey(key) || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

func canonicalHoldModifierKeys(keys []keyboard.Key) []keyboard.Key {
	canonical := make([]keyboard.Key, 0, len(keys))
	seen := map[keyboard.Key]bool{}
	for _, key := range keys {
		if key == keyboard.KeyUnknown || seen[key] {
			continue
		}
		canonical = append(canonical, key)
		seen[key] = true
	}
	sort.Slice(canonical, func(i, j int) bool {
		return canonical[i] < canonical[j]
	})
	return canonical
}

func holdModifierComboString(keys []keyboard.Key) string {
	parts := []string{}
	for _, key := range canonicalHoldModifierKeys(keys) {
		if character := key.Character(); character != "" {
			parts = append(parts, character)
		}
	}
	return strings.Join(parts, "+")
}

func containsHoldModifierKey(keys []keyboard.Key, target keyboard.Key) bool {
	for _, key := range keys {
		if key == target {
			return true
		}
	}
	return false
}

func orderedHoldModifierRecorderKeys() []keyboard.Key {
	return []keyboard.Key{
		keyboard.KeyLeftCtrl,
		keyboard.KeyRightCtrl,
		keyboard.KeyLeftShift,
		keyboard.KeyRightShift,
		keyboard.KeyLeftAlt,
		keyboard.KeyRightAlt,
		keyboard.KeyLeftSuper,
		keyboard.KeyRightSuper,
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

// IsModifierChordHotkeyString reports whether combineKey is a pure modifier
// chord (e.g. "left_alt", "left_ctrl+left_alt") that resolves to hold/press
// behavior instead of a normal modifier+key combo. Callers use this to decide
// whether a dictation hotkey belongs in the Wayland portal group (normal combos
// only) or must stay on the evdev special-registration path (modifier chords).
func IsModifierChordHotkeyString(combineKey string) bool {
	spec, err := (&Hotkey{}).parseCombineKey(combineKey)
	return err == nil && spec.isModifierChord()
}

// IsSpecialHotkeyString reports whether combineKey is a special hotkey
// (double-modifier, CapsLock combo, or modifier chord). Special hotkeys cannot
// be bound through the Wayland GlobalShortcuts portal group and must be
// registered individually via the evdev/raw-key path.
func IsSpecialHotkeyString(combineKey string) bool {
	spec, err := (&Hotkey{}).parseCombineKey(combineKey)
	if err != nil {
		return false
	}
	return spec.isDoubleModifier() || spec.isCapsLockKey() || spec.isModifierChord()
}
