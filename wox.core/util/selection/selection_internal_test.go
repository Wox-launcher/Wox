package selection

import (
	"context"
	"testing"
)

func TestGetSelectedPrefersInternalProvider(t *testing.T) {
	SetInternalSelectedTextProvider(func() (string, bool) {
		return "note selection", true
	})
	t.Cleanup(func() { SetInternalSelectedTextProvider(nil) })

	got, err := GetSelected(context.Background())
	if err != nil {
		t.Fatalf("GetSelected: %v", err)
	}
	if got.Type != SelectionTypeText || got.Text != "note selection" {
		t.Fatalf("selection = %+v, want note selection", got)
	}
}

func TestGetSelectedTreatsEmptyInternalSelectionAsNoSelection(t *testing.T) {
	SetInternalSelectedTextProvider(func() (string, bool) {
		return "", true
	})
	t.Cleanup(func() { SetInternalSelectedTextProvider(nil) })

	_, err := GetSelected(context.Background())
	if err != noSelection {
		t.Fatalf("empty internal selection err = %v, want noSelection", err)
	}
}

func TestLookupInternalSelectedTextIgnoresUnhandledProvider(t *testing.T) {
	SetInternalSelectedTextProvider(func() (string, bool) {
		return "ignored", false
	})
	t.Cleanup(func() { SetInternalSelectedTextProvider(nil) })

	text, handled := lookupInternalSelectedText()
	if handled {
		t.Fatalf("unhandled provider claimed selection %q", text)
	}
}
