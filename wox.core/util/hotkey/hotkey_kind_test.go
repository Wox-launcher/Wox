package hotkey

import (
	"runtime"
	"strings"
	"testing"
)

// TestWindowsKeyRegistersWithoutOpeningOtherModifierOnlyShortcuts keeps the
// launcher exception limited to one Windows key and leaves dictation intact.
func TestWindowsKeyRegistersWithoutOpeningOtherModifierOnlyShortcuts(t *testing.T) {
	for _, key := range []string{"left_win", "right_win", "left_alt", "left_ctrl", "left_win+left_shift"} {
		spec := mustParseHotkeySpec(t, key)
		kind, err := resolveHotkeyKind(spec, false, registerOptions{})
		allowed := runtime.GOOS == "windows" && (key == "left_win" || key == "right_win")
		if allowed && (err != nil || kind != hotkeyKindPressModifier) || !allowed && err == nil {
			t.Fatalf("ordinary registration of %s: kind=%s err=%v", key, kind, err)
		}
	}
}

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
		"ctrl+,",
		"ctrl+.",
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
