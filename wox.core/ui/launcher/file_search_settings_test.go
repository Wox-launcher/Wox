package launcher

import "testing"

func TestFileSearchServiceVolumeRowsAreReadOnlyPrefix(t *testing.T) {
	columns := []formTableColumn{{Key: "Path"}, {Key: "Extra"}}
	rows := fileSearchServiceVolumeRows(columns, []string{`C:`, `D:\`, "  "}, "service volume")
	if len(rows) != 2 {
		t.Fatalf("volume rows = %d, want two formatted drive roots", len(rows))
	}
	if rows[0].Index != -1 || !rows[0].ReadOnly || rows[0].Cells[0].Text != `C:\` || rows[0].Cells[0].Tooltip != "service volume" {
		t.Fatalf("first volume row = %+v, want a locked C:\\ cell", rows[0])
	}
	if rows[1].Cells[0].Text != `D:\` || rows[1].Cells[1].Text != "" {
		t.Fatalf("second volume row = %+v, want D:\\ only in the Path column", rows[1])
	}
}

func TestIsFileSearchRootsTable(t *testing.T) {
	definition := formDefinition{Value: formDefinitionValue{Key: "roots"}}
	if !isFileSearchRootsTable("plugin-settings", definition, fileSearchPluginID) {
		t.Fatal("expected the File Search roots table to match")
	}
	if isFileSearchRootsTable("plugin-settings", definition, windowManagerPluginID) {
		t.Fatal("other plugins must not receive service volume rows")
	}
	if isFileSearchRootsTable("plugin-settings", formDefinition{Value: formDefinitionValue{Key: "contentRoots"}}, fileSearchPluginID) {
		t.Fatal("content roots must stay user-editable")
	}
}
