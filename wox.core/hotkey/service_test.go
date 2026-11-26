package hotkey

import (
	"context"
	"errors"
	"testing"
	"wox/setting"
	utilhotkey "wox/util/hotkey"
)

// TestPluginHotkeysCannotReplaceOtherOwners covers native special bindings that could otherwise overwrite callbacks.
func TestPluginHotkeysCannotReplaceOtherOwners(t *testing.T) {
	for _, source := range []EntrySource{SourcePlugin, SourceSelection, SourceQuery} {
		entries := []Entry{{Source: source, ID: "existing", CombineKey: "right_alt"}, {Source: SourcePlugin, ID: "incoming", CombineKey: "hold:right_alt"}}
		if err := validateHotkeyConflicts(entries); err == nil {
			t.Fatalf("accepted plugin conflict with %s", source)
		}
	}
}

// TestQueryReleasePolicyIsEvaluatedAtTrigger keeps live routing changes out of registration snapshots.
func TestQueryReleasePolicyIsEvaluatedAtTrigger(t *testing.T) {
	// Selection hotkeys are unavailable on some Wayland compositors.
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("WAYLAND_SOCKET", "")
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

func TestRegisteredHotkeyStateMatchesHoldBindingAndSkippedEmptyEntry(t *testing.T) {
	entries := []Entry{
		{Source: SourceMain, ID: "empty"},
		{Source: SourcePlugin, ID: "hold", CombineKey: "hold:right_shift", OnPress: func() {}, OnRelease: func() {}},
		{Source: SourcePlugin, ID: "other", CombineKey: "ctrl+k", OnPress: func() {}},
	}
	specs, err := buildHotkeySpecs(entries)
	if err != nil {
		t.Fatal(err)
	}
	registeredSpecs, registered := registeredHotkeyState(specs, entries, []string{"right_shift"})
	if len(registered) != 1 || registered[0].ID != "hold" || len(registeredSpecs) != 1 || registeredSpecs[0].CombineKey != "right_shift" {
		t.Fatalf("registered entries = %+v, specs = %+v", registered, registeredSpecs)
	}
}

func TestResultHotkeyConflictsIncludePluginAndModifierOrder(t *testing.T) {
	entries := []Entry{{Source: SourcePlugin, ID: "plugin", CombineKey: "Ctrl+Alt+K"}, {Source: SourceResult, ID: "result", CombineKey: "alt+ctrl+k"}}
	if err := validateHotkeyConflicts(entries); err == nil {
		t.Fatal("plugin conflict was accepted")
	}
	entries[0].CombineKey = "hold:left_alt"
	entries[1].CombineKey = "left_alt"
	if err := validateHotkeyConflicts(entries); err == nil {
		t.Fatal("hold/press conflict was accepted")
	}
}

func TestResultBindingPartialRegistrationIsFailure(t *testing.T) {
	entries := []Entry{{Source: SourceMain, ID: "main", CombineKey: "alt+space"}, {Source: SourceResult, ID: "result", CombineKey: "ctrl+k"}}
	if err := missingResultHotkey(entries, entries[:1]); err == nil {
		t.Fatal("partial native registration was accepted")
	}
	if err := missingResultHotkey(entries, entries); err != nil {
		t.Fatal(err)
	}
}

func TestResultBindingPersistenceFailureRollsBackCollector(t *testing.T) {
	service := NewService(Callbacks{})
	service.collectResultBindings([]setting.ResultBinding{{Hash: "old", Hotkey: "ctrl+k"}})
	saveErr := errors.New("disk full")
	err := service.ApplyResultBindings(context.Background(), nil, func() error { return saveErr })
	if !errors.Is(err, saveErr) {
		t.Fatalf("save error lost: %v", err)
	}
	entries := service.Snapshot()
	if len(entries) != 1 || entries[0].ID != "old" {
		t.Fatalf("previous configuration not restored: %+v", entries)
	}
}

func TestSyncedConfigSavesUnavailableChordAndDropsOldRegistration(t *testing.T) {
	ctx := context.Background()
	service := NewService(Callbacks{})
	service.collectWoxConfig(ctx, WoxConfig{MainHotkey: "ctrl+j"})
	service.registered = service.Snapshot()
	incoming := WoxConfig{MainHotkey: "ctrl+invalid-key"}
	var saved bool
	err := service.ApplySyncedConfig(ctx, incoming, func() error {
		saved = true
		return nil
	})
	if !saved || !errors.Is(err, ErrHotkeyRegistrationConflict) {
		t.Fatalf("expected successful persistence and a registration warning, saved=%t err=%v", saved, err)
	}
	if got := service.Snapshot(); len(got) != 1 || got[0].CombineKey != incoming.MainHotkey || service.IsRegistered(SourceMain, "main") {
		t.Fatalf("expected shared chord without a fallback binding, got %+v", got)
	}
}

func TestSyncedHotkeySpecsKeepsOtherShortcutsWhenOneChordConflicts(t *testing.T) {
	press := func() {}
	entries, specs := syncedHotkeySpecs([]Entry{
		{Source: SourcePlugin, ID: "plugin", CombineKey: "ctrl+k", OnPress: press},
		{Source: SourceMain, ID: "main", CombineKey: "ctrl+k", OnPress: press},
		{Source: SourceQuery, ID: "query", CombineKey: "ctrl+j", OnPress: press},
	})
	if len(entries) != 2 || len(specs) != 2 || entries[0].Source != SourceMain || entries[1].Source != SourceQuery {
		t.Fatalf("shared chord priority must preserve the unrelated shortcut, got %+v", entries)
	}
}

func TestSyncedResultConflictKeepsOtherHotkeys(t *testing.T) {
	entries := []Entry{
		{Source: SourceResult, ID: "conflict", CombineKey: "alt+ctrl+k", OnPress: func() {}},
		{Source: SourceMain, ID: "main", CombineKey: "ctrl+alt+k", OnPress: func() {}},
		{Source: SourceResult, ID: "malformed", CombineKey: "invalid-key", OnPress: func() {}},
		{Source: SourceResult, ID: "valid", CombineKey: "ctrl+j", OnPress: func() {}},
	}
	got := usableResultHotkeys(context.Background(), entries)
	if len(got) != 2 || got[0].Source != SourceMain || got[1].ID != "valid" {
		t.Fatalf("unrelated hotkeys were lost: %+v", got)
	}
}

// TestStandaloneHoldSpecAndConflict covers declarative plugin admission.
func TestStandaloneHoldSpecAndConflict(t *testing.T) {
	entries := []Entry{{Source: SourcePlugin, ID: "hold", CombineKey: "hold:delete", OnPress: func() {}, OnRelease: func() {}}}
	specs, err := buildHotkeySpecs(entries)
	if err != nil || len(specs) != 1 || specs[0].CombineKey != "delete" || specs[0].OnRelease == nil {
		t.Fatalf("standalone hold specs = %+v, err = %v", specs, err)
	}
	entries = append(entries, Entry{Source: SourcePlugin, ID: "tap", CombineKey: "delete", OnPress: func() {}})
	if err := validateHotkeyConflicts(entries); err == nil {
		t.Fatal("same key was accepted for press and hold")
	}
}
