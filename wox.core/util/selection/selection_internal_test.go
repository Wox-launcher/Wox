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

func TestClipboardCopyObservedPrefersSequenceNumber(t *testing.T) {
	if !clipboardCopyObserved(10, 11, 100, 0) {
		t.Fatal("sequence change must count as a copy")
	}
	if clipboardCopyObserved(10, 10, 100, 200) {
		t.Fatal("stale watcher timestamp must not count while sequence is unchanged")
	}
	if !clipboardCopyObserved(0, 0, 100, 100) {
		t.Fatal("platforms without a sequence counter fall back to the watcher timestamp")
	}
	if clipboardCopyObserved(0, 0, 100, 99) {
		t.Fatal("watcher timestamp from before simulate must not count as a copy")
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
