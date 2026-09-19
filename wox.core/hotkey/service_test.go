package hotkey

import (
	"context"
	"testing"
	"wox/setting"
	utilhotkey "wox/util/hotkey"
)

// TestQueryReleasePolicyIsEvaluatedAtTrigger keeps live routing changes out of registration snapshots.
func TestQueryReleasePolicyIsEvaluatedAtTrigger(t *testing.T) {
	allow := false
	service := NewService(Callbacks{
		QueryCanTriggerBeforeRelease: func(query setting.QueryHotkey) bool {
			return allow && query.Query == "screenshot new"
		},
	})
	service.collectWoxConfig(context.Background(), WoxConfig{
		MainHotkey: "capslock+w", SelectionHotkey: "capslock+s",
		QueryHotkeys: []setting.QueryHotkey{
			{Hotkey: "capslock+a", Query: "screenshot new", IsSilentExecution: true},
			{Hotkey: "capslock+b", Query: "other"},
		},
	})
	specs, err := buildHotkeySpecs(service.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 4 || specs[0].CanTriggerBeforeRelease != nil || specs[1].CanTriggerBeforeRelease != nil {
		t.Fatal("main and selection hotkeys must keep their release guard")
	}
	if specs[2].CanTriggerBeforeRelease() {
		t.Fatal("query policy was ignored")
	}
	allow = true
	if !specs[2].CanTriggerBeforeRelease() || specs[3].CanTriggerBeforeRelease() {
		t.Fatal("each query must use its current routing policy")
	}
}

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
