package raycast

import (
	"testing"

	"wox/plugin/thirdparty/migrate"
)

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
