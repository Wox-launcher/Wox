package hotkey

import (
	"fmt"
	"strings"
	"wox/util"
	"wox/util/keyboard"
)

// MapHotkeyBetweenPlatforms translates the primary modifier while preserving binding intent.
func MapHotkeyBetweenPlatforms(value, from, to string) (string, error) {
	if !util.IsSupportedPlatform(from) || !util.IsSupportedPlatform(to) {
		return "", fmt.Errorf("invalid shortcut platform")
	}
	binding, err := ParseBinding(value)
	if err != nil || binding.CombineKey == "" {
		return "", err
	}
	if from != to && (from == util.PlatformMacOS || to == util.PlatformMacOS) {
		tokens := strings.Split(binding.CombineKey, "+")
		for i, token := range tokens {
			_, key, ok := parseModifierToken(token)
			if !ok {
				continue
			}
			mapped := crossPlatformModifier(key, from == util.PlatformMacOS)
			if mapped != "" {
				tokens[i] = mapped
			}
		}
		binding.CombineKey = strings.Join(tokens, "+")
	}
	if _, err := (&Hotkey{}).parseCombineKey(binding.CombineKey); err != nil {
		return "", err
	}
	if binding.Trigger == TriggerHold {
		return holdBindingPrefix + binding.CombineKey, nil
	}
	return binding.CombineKey, nil
}

// crossPlatformModifier swaps Ctrl and Super roles to preserve chords that use both modifiers.
func crossPlatformModifier(key keyboard.Key, fromMac bool) string {
	switch key {
	case keyboard.KeySuper:
		return "ctrl"
	case keyboard.KeyLeftSuper:
		return "left_ctrl"
	case keyboard.KeyRightSuper:
		return "right_ctrl"
	case keyboard.KeyCtrl:
		if !fromMac {
			return "cmd"
		}
		return "super"
	case keyboard.KeyLeftCtrl:
		if !fromMac {
			return "left_cmd"
		}
		return "left_super"
	case keyboard.KeyRightCtrl:
		if !fromMac {
			return "right_cmd"
		}
		return "right_super"
	case keyboard.KeyAlt, keyboard.KeyLeftAlt, keyboard.KeyRightAlt:
		modifier := "option"
		if fromMac {
			modifier = "alt"
		}
		switch key {
		case keyboard.KeyLeftAlt:
			return "left_" + modifier
		case keyboard.KeyRightAlt:
			return "right_" + modifier
		default:
			return modifier
		}
	}
	return ""
}
