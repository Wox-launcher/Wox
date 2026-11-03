package ui

import (
	"testing"
	corehotkey "wox/hotkey"
	"wox/setting"
)

// TestQueryHotkeyReleasePolicy keeps selection reads guarded without delaying unrelated queries.
func TestQueryHotkeyReleasePolicy(t *testing.T) {
	for _, query := range []string{
		"screenshot new ", "apps ", "search {ordinary}",
		"search {wox:clipboard_text}", "search {wox:active_browser_url}",
		"search {wox:file_explorer_path}", "search {wox:selected_text}", "files {wox:selected_file}",
	} {
		for _, silent := range []bool{false, true} {
			want := query != "search {wox:selected_text}" && query != "files {wox:selected_file}"
			got := queryCanTriggerBeforeRelease(setting.QueryHotkey{Query: query, IsSilentExecution: silent})
			if got != want {
				t.Errorf("query %q silent=%t: before release=%t, want %t", query, silent, got, want)
			}
		}
	}
}

// TestHotkeyCompareUsesPhysicalWinChord covers aliases and dictation's hold
// binding without treating the left and right Windows keys as the same key.
func TestHotkeyCompareUsesPhysicalWinChord(t *testing.T) {
	left := hotkeyCompareKeys("left_win")
	for _, alias := range []string{"left_cmd", "left_super", "hold:left_win"} {
		if !hotkeyCompareKeysIntersect(left, hotkeyCompareKeys(alias)) {
			t.Fatalf("%s must conflict with left Win", alias)
		}
	}
	if hotkeyCompareKeysIntersect(left, hotkeyCompareKeys("right_win")) {
		t.Fatal("left and right Win must remain distinct")
	}
}

// TestHotkeyLegacyAliases retains old conflict comparisons alongside physical chords.
func TestHotkeyLegacyAliases(t *testing.T) {
	for _, pair := range [][2]string{{"win+e", "windows+e"}, {"win+e", "meta+e"}, {"alt+space", "option+space"}} {
		if !hotkeyCompareKeysIntersect(hotkeyCompareKeys(pair[0]), hotkeyCompareKeys(pair[1])) {
			t.Errorf("aliases diverge: %v", pair)
		}
	}
}

func TestHotkeyEntryConflictIdentifiesPluginBinding(t *testing.T) {
	entries := []corehotkey.Entry{{Source: corehotkey.SourcePlugin, Owner: "plugin", ID: "plugin\x00hotkey\x00row\x00column", CombineKey: "ctrl+shift+o"}}
	got := hotkeyEntryConflict(hotkeyCompareKeys("shift+ctrl+o"), entries)
	if got.Available || got.ConflictType != "plugin" || got.PluginID != "plugin" || got.SettingKey != "hotkey" || got.ItemID != "row" || got.ColumnKey != "column" {
		t.Fatalf("plugin conflict = %+v", got)
	}
	if got := hotkeyEntryConflict(hotkeyCompareKeys("ctrl+shift+p"), entries); !got.Available {
		t.Fatalf("unrelated shortcut conflict = %+v", got)
	}
}
