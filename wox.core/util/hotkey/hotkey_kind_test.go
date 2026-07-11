package hotkey

import (
	"strings"
	"testing"
)

func TestResolveHotkeyKindForModifierChordByRegistrationIntent(t *testing.T) {
	spec := mustParseHotkeySpec(t, "left_alt")

	kind, err := resolveHotkeyKind(spec, true, registerOptions{})
	if err != nil {
		t.Fatalf("expected hold modifier kind, got error: %v", err)
	}
	if kind != hotkeyKindHoldModifier {
		t.Fatalf("expected %s, got %s", hotkeyKindHoldModifier, kind)
	}

	kind, err = resolveHotkeyKind(spec, false, registerOptions{allowModifierPress: true})
	if err != nil {
		t.Fatalf("expected press modifier kind, got error: %v", err)
	}
	if kind != hotkeyKindPressModifier {
		t.Fatalf("expected %s, got %s", hotkeyKindPressModifier, kind)
	}

	if _, err = resolveHotkeyKind(spec, false, registerOptions{}); err == nil {
		t.Fatalf("expected ordinary press-only registration to reject modifier-only chord")
	}
}

func TestResolveHotkeyKindRejectsReleaseForNonModifierChord(t *testing.T) {
	cases := []string{
		"ctrl+space",
		"ctrl+ctrl",
		"capslock+e",
	}

	for _, combineKey := range cases {
		t.Run(combineKey, func(t *testing.T) {
			spec := mustParseHotkeySpec(t, combineKey)
			_, err := resolveHotkeyKind(spec, true, registerOptions{})
			if err == nil {
				t.Fatalf("expected release registration to reject %s", combineKey)
			}
			if !strings.Contains(err.Error(), "release") {
				t.Fatalf("expected release error, got: %v", err)
			}
		})
	}
}

func mustParseHotkeySpec(t *testing.T, combineKey string) hotkeySpec {
	t.Helper()

	spec, err := (&Hotkey{}).parseCombineKey(combineKey)
	if err != nil {
		t.Fatalf("parse %s: %v", combineKey, err)
	}
	return spec
}
