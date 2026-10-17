package raycast

import (
	"testing"

	"wox/plugin/thirdparty/migrate"
)

func TestCatalogPlacesShortcutsAfterQuicklinksAndGeneralNearTheTop(t *testing.T) {
	catalog := catalogFrom(mappedExport{
		hotkeys:    []mappedHotkey{{id: "setting:main_hotkey", title: "Main", selectable: true}},
		general:    []mappedSetting{{id: "setting:general:0", titleKey: "autostart", selectable: true}},
		quicklinks: []mappedQuicklink{{id: "quicklink:0", title: "wox", selectable: true}},
		shortcuts:  []mappedShortcut{{id: "setting:result_binding:0", title: "百度网盘", selectable: true}},
		snippets:   []mappedSnippet{{id: "snippet:0", title: "note", selectable: true}},
	})
	got := make([]string, 0, len(catalog))
	for _, category := range catalog {
		got = append(got, category.ID)
	}
	want := []string{migrate.CategoryHotkeys, migrate.CategoryGeneral, migrate.CategoryQuicklinks, migrate.CategoryShortcuts, migrate.CategorySnippets}
	if len(got) != len(want) {
		t.Fatalf("order = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v", got)
		}
	}
}

func TestCatalogHidesUnimportableRowsAndClipboardEntries(t *testing.T) {
	catalog := catalogFrom(mappedExport{
		hotkeys: []mappedHotkey{
			{id: "setting:main_hotkey", status: statusUnsupported},
			{id: "setting:query_hotkey:1", title: "Apps", selectable: true},
		},
		queries: []mappedQuery{{id: "setting:query_alias:0", status: statusUnsupported}},
		clipboard: []mappedClipboard{
			{id: "clipboard:1", text: "keep", selectable: true},
			{id: "clipboard:2", status: statusUnsupported},
		},
		summary: clipboardSummary{importable: 1, expired: 4},
	})
	if len(catalog) != 2 || catalog[0].ID != migrate.CategoryHotkeys || len(catalog[0].Items) != 1 || catalog[0].Items[0].ID != "setting:query_hotkey:1" {
		t.Fatalf("categories = %+v", catalog)
	}
	clipboard := catalog[1]
	if clipboard.ID != migrate.CategoryClipboard || len(clipboard.Items) != 1 || clipboard.Items[0].ID != migrate.ClipboardSummaryID || !clipboard.Items[0].Selectable {
		t.Fatalf("clipboard = %+v", clipboard.Items)
	}
	if clipboard.Items[0].Detail != "1|0|4|0|0" {
		t.Fatalf("summary = %q", clipboard.Items[0].Detail)
	}

	imported := importSelected(mappedExport{
		clipboard: []mappedClipboard{
			{id: "clipboard:1", text: "keep", selectable: true},
			{id: "clipboard:2", text: "old", status: statusUnsupported},
		},
	}, []string{migrate.ClipboardSummaryID})
	if len(imported.Clipboard) != 1 || imported.Clipboard[0].Text != "keep" {
		t.Fatalf("import = %+v", imported.Clipboard)
	}
}
