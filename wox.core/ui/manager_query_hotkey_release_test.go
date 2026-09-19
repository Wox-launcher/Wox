package ui

import (
	"testing"
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
