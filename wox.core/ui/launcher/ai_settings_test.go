package launcher

import "testing"

func TestNewAISettingsFormMatchesFlutterTableDefinitions(t *testing.T) {
	form := newAISettingsForm(settingsData{})
	if len(form.definitions) != 3 {
		t.Fatalf("AI table count = %d, want 3", len(form.definitions))
	}

	providers := form.definitions[0].Value
	if !providers.InlineTable || providers.SortColumnKey != "Name" {
		t.Fatalf("provider table options = inline %v, sort %q; want inline and Name", providers.InlineTable, providers.SortColumnKey)
	}
	assertFormTableColumnWidths(t, providers.Columns, []int{40, 100, 120, 160, 0})
	if providers.Columns[0].Type != "aiModelStatus" || !providers.Columns[0].HideInUpdate {
		t.Fatalf("provider status column = type %q, hide in update %v", providers.Columns[0].Type, providers.Columns[0].HideInUpdate)
	}

	mcp := form.definitions[1].Value
	if !mcp.InlineTable || mcp.SortColumnKey != "Name" {
		t.Fatalf("MCP table options = inline %v, sort %q; want inline and Name", mcp.InlineTable, mcp.SortColumnKey)
	}
	assertFormTableColumnWidths(t, mcp.Columns, []int{100, 50, 80, 80, 100, 160, 120})

	skills := form.definitions[2].Value
	if !skills.InlineTable || skills.SortColumnKey != "Name" || skills.MaxHeight != 360 {
		t.Fatalf("skills table options = inline %v, sort %q, max height %d; want inline, Name, 360", skills.InlineTable, skills.SortColumnKey, skills.MaxHeight)
	}
	assertFormTableColumnWidths(t, skills.Columns[:3], []int{200, 100, 400})
}

func assertFormTableColumnWidths(t *testing.T, columns []formTableColumn, want []int) {
	t.Helper()
	if len(columns) != len(want) {
		t.Fatalf("column count = %d, want %d", len(columns), len(want))
	}
	for index, width := range want {
		if columns[index].Width != width {
			t.Fatalf("column %d width = %d, want %d", index, columns[index].Width, width)
		}
	}
}
