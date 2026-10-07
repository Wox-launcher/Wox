package hotkey

import (
	"testing"
	"wox/util/keyboard"
)

// TestParseComboSharesRegistrationAliases covers the previously divergent core/UI spellings.
func TestParseComboSharesRegistrationAliases(t *testing.T) {
	for _, value := range []string{"ctrl+del", "control+delete", " CTRL + DEL "} {
		combo, err := ParseCombo(value)
		if err != nil || combo.Key != "delete" || combo.Modifiers != keyboard.ModifierCtrl {
			t.Fatalf("local combo %q = %+v, %v", value, combo, err)
		}
		spec, err := (&Hotkey{}).parseCombineKey(value)
		if err != nil || spec.key != keyboard.KeyDelete || spec.modifiers != combo.Modifiers {
			t.Fatalf("native combo %q = %+v, %v", value, spec, err)
		}
	}
	for _, value := range []string{"meta+return", "command+enter", "win+enter"} {
		combo, err := ParseCombo(value)
		if err != nil || combo.Key != "return" || combo.Modifiers != keyboard.ModifierSuper {
			t.Fatalf("meta combo %q = %+v, %v", value, combo, err)
		}
		spec, err := (&Hotkey{}).parseCombineKey(value)
		if err != nil || spec.key != keyboard.KeyReturn || spec.modifiers != combo.Modifiers {
			t.Fatalf("native combo %q = %+v, %v", value, spec, err)
		}
	}
}

// TestParseComboKeepsLocalNavigationAndSpecialTriggersSeparate preserves both callers' capability boundaries.
func TestParseComboKeepsLocalNavigationAndSpecialTriggersSeparate(t *testing.T) {
	for _, value := range []string{"ctrl+page-up", "ctrl+pagedown", "ctrl+home", "ctrl+end", "ctrl+backspace", "ctrl+é"} {
		if _, err := ParseCombo(value); err != nil {
			t.Fatalf("local navigation %q: %v", value, err)
		}
		if _, err := (&Hotkey{}).parseCombineKey(value); err == nil {
			t.Fatalf("local-only navigation %q bypassed native support checking", value)
		}
	}
	for _, value := range []string{"capslock+k", "ctrl+ctrl", "left_ctrl+k", "left_ctrl", "hold:ctrl+k", "ctrl+unknown", "ctrl+", "a+b"} {
		if _, err := ParseCombo(value); err == nil {
			t.Fatalf("unsupported local combo %q was accepted", value)
		}
	}
	for _, value := range []string{"capslock+k", "ctrl+ctrl", "left_ctrl+k", "left_ctrl"} {
		if _, err := (&Hotkey{}).parseCombineKey(value); err != nil {
			t.Fatalf("existing native trigger %q: %v", value, err)
		}
	}
}
