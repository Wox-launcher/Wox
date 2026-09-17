package woxui

import (
	"strings"
	"testing"
)

func TestParseWebViewActionHotkey(t *testing.T) {
	got, ok := ParseHotkey("ctrl+k")
	if !ok || got.Key != "k" || got.Modifiers != KeyModifierControl {
		t.Fatalf("ctrl+k = %+v ok=%t", got, ok)
	}
	got, ok = ParseHotkey("command+j")
	if !ok || got.Key != "j" || got.Modifiers != KeyModifierMeta {
		t.Fatalf("command+j = %+v ok=%t", got, ok)
	}
	if _, ok = ParseHotkey(""); ok {
		t.Fatal("empty hotkey should not parse")
	}
}

func TestWebViewActionHotkeyJSUsesConfiguredKeyOnly(t *testing.T) {
	js := webViewActionHotkeyJS("ctrl+k")
	if !strings.Contains(js, `key:"k"`) || !strings.Contains(js, "ctrl:true") {
		t.Fatalf("js = %s", js)
	}
	if strings.Contains(js, `"j"`) {
		t.Fatalf("configured K must not also reserve J: %s", js)
	}
	if js := webViewActionHotkeyJS(""); js != "window.__woxActionHotkey=null" {
		t.Fatalf("empty js = %s", js)
	}
}

func TestWebViewActionHotkeyMatchesExactCombo(t *testing.T) {
	parsed, ok := ParseHotkey("ctrl+k")
	if !ok || !parsed.Matches("k", KeyModifierControl) {
		t.Fatal("ctrl+k should match")
	}
	if parsed.Matches("j", KeyModifierControl) || parsed.Matches("k", KeyModifierControl|KeyModifierShift) {
		t.Fatal("action hotkey must not match a different key or modifier set")
	}
}

func TestHotkeyRecordedAliasesAndInvalidModifiers(t *testing.T) {
	for _, tt := range []struct {
		text      string
		key       Key
		modifiers KeyModifiers
	}{
		{"win+k", "k", KeyModifierMeta},
		{"super+shift+k", "k", KeyModifierMeta | KeyModifierShift},
		{"ctrl+left", KeyArrowLeft, KeyModifierControl},
		{"ctrl+right", KeyArrowRight, KeyModifierControl},
		{"ctrl+up", KeyArrowUp, KeyModifierControl},
		{"ctrl+down", KeyArrowDown, KeyModifierControl},
		{"ctrl+pageup", KeyPageUp, KeyModifierControl},
		{"option+return", KeyEnter, KeyModifierAlt},
	} {
		parsed, ok := ParseHotkey(tt.text)
		if !ok || !parsed.Matches(tt.key, tt.modifiers) || parsed.Matches(tt.key, 0) {
			t.Fatalf("%s = %+v, valid=%t", tt.text, parsed, ok)
		}
	}
	for _, text := range []string{"unknown+k", "capslock+k", "ctrl+ctrl", "hold:ctrl+k", "ctrl+"} {
		if parsed, ok := ParseHotkey(text); ok {
			t.Fatalf("unsupported shortcut %q parsed as %+v", text, parsed)
		}
	}
}
