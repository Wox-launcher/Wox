package hotkey

import (
	"fmt"
	"strings"
	"wox/util/keyboard"

	"github.com/samber/lo"
)

type hotkeySpec struct {
	hyper             bool
	capsLock          bool
	modifiers         keyboard.Modifier
	key               keyboard.Key
	doubleModifierKey keyboard.Key
}

func (s hotkeySpec) isHyperKey() bool {
	return s.hyper
}

func (s hotkeySpec) isCapsLockKey() bool {
	return s.capsLock
}

func (s hotkeySpec) isDoubleModifier() bool {
	return s.doubleModifierKey != keyboard.KeyUnknown
}

func (h *Hotkey) parseCombineKey(combineKey string) (hotkeySpec, error) {
	tokens := lo.Map(strings.Split(combineKey, "+"), func(item string, index int) string {
		return strings.TrimSpace(item)
	})

	var spec hotkeySpec
	var modifierKeys []keyboard.Key

	for _, token := range tokens {
		normalizedToken := strings.ToLower(strings.TrimSpace(token))
		if normalizedToken == "hyper" {
			spec.hyper = true
			continue
		}
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
		if spec.hyper {
			return hotkeySpec{}, fmt.Errorf("missing key in hyper hotkey: %s", combineKey)
		}
		if spec.capsLock {
			return hotkeySpec{}, fmt.Errorf("missing key in caps lock hotkey: %s", combineKey)
		}
		if len(modifierKeys) == 2 && modifierKeys[0] == modifierKeys[1] {
			spec.doubleModifierKey = modifierKeys[0]
			return spec, nil
		}
		return hotkeySpec{}, fmt.Errorf("missing key in hotkey: %s", combineKey)
	}

	if spec.hyper && (spec.modifiers != 0 || len(modifierKeys) > 0) {
		return hotkeySpec{}, fmt.Errorf("hyper hotkey does not support extra modifiers: %s", combineKey)
	}
	if spec.capsLock && (spec.modifiers != 0 || len(modifierKeys) > 0) {
		return hotkeySpec{}, fmt.Errorf("caps lock hotkey does not support extra modifiers: %s", combineKey)
	}
	if spec.hyper && spec.capsLock {
		return hotkeySpec{}, fmt.Errorf("hyper hotkey cannot also be caps lock hotkey: %s", combineKey)
	}

	return spec, nil
}

func isCapsLockToken(token string) bool {
	return token == "capslock" || token == "caps_lock" || token == "caps lock"
}

func IsHyperHotkeyString(combineKey string) bool {
	spec, err := (&Hotkey{}).parseCombineKey(combineKey)
	return err == nil && spec.isHyperKey()
}

func IsCapsLockHotkeyString(combineKey string) bool {
	spec, err := (&Hotkey{}).parseCombineKey(combineKey)
	return err == nil && spec.isCapsLockKey()
}
