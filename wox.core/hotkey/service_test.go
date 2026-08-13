package hotkey

import (
	"testing"
	utilhotkey "wox/util/hotkey"
)

func TestRegisteredHotkeyStateKeepsOnlySuccessfulEntries(t *testing.T) {
	entries := []Entry{
		{Source: SourceMain, ID: "main", CombineKey: "alt+space"},
		{Source: SourceSelection, ID: "selection", CombineKey: "win+alt+space"},
	}
	specs := []utilhotkey.Spec{
		{CombineKey: "alt+space", Callback: func() {}},
		{CombineKey: "win+alt+space", Callback: func() {}},
	}

	registeredSpecs, registeredEntries := registeredHotkeyState(specs, entries, []string{"win+alt+space"})
	if len(registeredSpecs) != 1 || registeredSpecs[0].CombineKey != "win+alt+space" {
		t.Fatalf("registered specs = %+v, want selection hotkey only", registeredSpecs)
	}
	if len(registeredEntries) != 1 || registeredEntries[0].Source != SourceSelection {
		t.Fatalf("registered entries = %+v, want selection entry only", registeredEntries)
	}
}
