package component

import (
	"testing"

	"wox/common"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestNoteTextRunFieldRunUsesGenericDecorations(t *testing.T) {
	run := NoteTextRun{Start: 0, End: 1, Style: woxui.TextStyle{Size: 14}, Color: DocumentListMarkerColor, Checkbox: true, Checked: true}
	field := run.FieldRun()
	if field.Advance <= 0 || !field.HideText || field.Paint == nil {
		t.Fatalf("checkbox run was not converted to a generic paint hook: %#v", field)
	}
	quote := NoteTextRun{Start: 0, End: 4, Style: woxui.TextStyle{Size: 14}, Color: DocumentListMarkerColor, LeadingBar: true}.FieldRun()
	if !quote.LineGutter || quote.PaintLineGutter == nil {
		t.Fatalf("quote run was not converted to a line gutter: %#v", quote)
	}
	rule := NoteTextRun{Start: 0, End: 8, Color: woxui.Color{A: 255}, HorizontalRule: true}.FieldRun()
	if !rule.HideText || rule.Paint == nil {
		t.Fatalf("divider run was not converted to a custom paint: %#v", rule)
	}
}

func TestInsertAndDeleteTableRowsKeepAGrid(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{{Type: common.NoteBlockParagraph}}}
	document, index := InsertNoteTable(document, nil, woxui.TextSelection{})
	if document.Blocks[index].Table == nil || len(document.Blocks[index].Table.Rows) != 2 {
		t.Fatalf("inserted table = %#v", document.Blocks[index])
	}
	expanded := InsertNoteTableRow(document, index, 0)
	if len(expanded.Blocks[index].Table.Rows) != 3 {
		t.Fatalf("insert row = %#v", expanded.Blocks[index].Table)
	}
	expanded = InsertNoteTableColumn(expanded, index, 0)
	if noteTableColumns(*expanded.Blocks[index].Table) != 4 {
		t.Fatalf("insert column = %#v", expanded.Blocks[index].Table)
	}
	shrunk := DeleteNoteTableColumn(expanded, index, 0)
	if noteTableColumns(*shrunk.Blocks[index].Table) != 3 {
		t.Fatalf("delete column = %#v", shrunk.Blocks[index].Table)
	}
	removed := DeleteNoteTable(shrunk, index)
	if len(removed.Blocks) == 0 || removed.Blocks[0].Type == common.NoteBlockTable {
		t.Fatalf("delete table = %#v", removed.Blocks)
	}
}

func TestWoxNoteEditorKeepsDocumentInsetsOffTableGaps(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "p", Type: common.NoteBlockParagraph, Text: "before"},
		{ID: "t", Type: common.NoteBlockTable, Table: &common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{
			{{Text: "A"}, {Text: "B"}}, {{Text: "1"}, {Text: "2"}},
		}}},
	}}
	editor := WoxNoteEditor(NoteEditorProps{
		ID: "notes.editor", Document: document, Width: 320, Height: 240, LineHeight: 24,
		Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 24},
		Style:   woxui.TextStyle{Size: 14}, Theme: Theme{ResultSubtitle: woxui.Color{A: 255}},
	})
	column := noteEditorColumn(t, editor)
	if column.Gap != noteEditorSegmentGap || len(column.Children) != 2 {
		t.Fatalf("note editor column = %#v, want text then table with an 8-unit gap", column)
	}
	field := column.Children[0].(woxwidget.Stateful).Widget.(TextFieldProps)
	if field.Padding.Top != 12 || field.Padding.Bottom != 0 || field.Padding.Left != 16 {
		t.Fatalf("leading text padding = %#v, want top 12 and no bottom inset above the table", field.Padding)
	}
	if field.Height != 36 {
		t.Fatalf("leading text height = %.0f, want one line plus the top inset", field.Height)
	}
}

func TestWoxNoteEditorScrollsWhenATableOverflows(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "p", Type: common.NoteBlockParagraph, Text: "before"},
		{ID: "t", Type: common.NoteBlockTable, Table: &common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{
			{{Text: "A"}, {Text: "B"}}, {{Text: "1"}, {Text: "2"}}, {{Text: "3"}, {Text: "4"}}, {{Text: "5"}, {Text: "6"}},
		}}},
	}}
	editor := WoxNoteEditor(NoteEditorProps{
		ID: "notes.editor", Document: document, Width: 320, Height: 80, LineHeight: 24,
		Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 24},
		Style:   woxui.TextStyle{Size: 14}, Theme: Theme{ResultSubtitle: woxui.Color{A: 255}},
	})
	stateful, ok := editor.(woxwidget.Stateful)
	if !ok {
		t.Fatalf("overflowing note editor = %T, want a stateful WoxScrollView", editor)
	}
	props, ok := stateful.Widget.(ScrollViewProps)
	if !ok || props.Key != "notes.editor.scroll" || props.Height != 80 || props.AlwaysShowScrollbar {
		t.Fatalf("note editor scroll = %#v, want a fading WoxScrollView", stateful.Widget)
	}
	inset, ok := props.Content.(woxwidget.Container)
	if !ok || inset.Padding.Bottom != 24 {
		t.Fatalf("note editor bottom inset = %#v, want 24 so the last row stays readable", props.Content)
	}
}

func TestWoxNoteTableHidesStructureActionsUntilCaretEnters(t *testing.T) {
	table := common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{{{Text: "A"}, {Text: "B"}}, {{Text: "1"}, {Text: "2"}}}}
	actions := NoteTableActions{
		Labels:         NoteTableActionLabels{InsertRow: "Insert row", DeleteTable: "Delete table"},
		OnInsertRow:    func() {},
		OnInsertColumn: func() {},
		OnDeleteRow:    func() {},
		OnDeleteColumn: func() {},
		OnDeleteTable:  func() {},
	}
	idle := WoxNoteTable(NoteTableProps{
		ID: "notes.table.demo", Table: table, Width: 360, Theme: Theme{ResultSubtitle: woxui.Color{A: 255}}, Style: woxui.TextStyle{Size: 14},
		Actions: actions,
	})
	focused := WoxNoteTable(NoteTableProps{
		ID: "notes.table.demo", Table: table, Width: 360, Theme: Theme{ResultSubtitle: woxui.Color{A: 255}}, Style: woxui.TextStyle{Size: 14},
		Actions: actions, Focused: true,
	})
	idleIDs := noteTableActionIDs(idle)
	if idleIDs["notes.table.demo.table-insert-row"] || idleIDs["notes.table.demo.table-delete"] {
		t.Fatalf("idle table actions = %#v, want them hidden until the caret enters", idleIDs)
	}
	idleSlot, ok := noteTableChrome(t, idle).(woxwidget.Container)
	if !ok || idleSlot.Height != noteTableToolbarHeight || idleSlot.Width != 360 {
		t.Fatalf("idle toolbar slot = %#v, want a reserved 360x28 row above the grid", noteTableChrome(t, idle))
	}
	focusedBar, ok := noteTableChrome(t, focused).(woxwidget.Container)
	if !ok || focusedBar.Height != noteTableToolbarHeight || focusedBar.Width != 360 {
		t.Fatalf("focused toolbar = %#v, want the same reserved 360x28 row so the grid does not move", noteTableChrome(t, focused))
	}
	focusedIDs := noteTableActionIDs(focused)
	for _, id := range []string{"notes.table.demo.table-insert-row", "notes.table.demo.table-insert-column", "notes.table.demo.table-delete-row", "notes.table.demo.table-delete-column", "notes.table.demo.table-delete"} {
		if !focusedIDs[id] {
			t.Fatalf("focused table missing %s in %#v", id, focusedIDs)
		}
	}
}

func TestWoxNoteTablePlacesDeleteActionOnTheRight(t *testing.T) {
	table := common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{{{Text: "A"}, {Text: "B"}}, {{Text: "1"}, {Text: "2"}}}}
	chrome := noteTableChrome(t, WoxNoteTable(NoteTableProps{
		ID: "notes.table.demo", Table: table, Width: 360, Theme: Theme{ResultSubtitle: woxui.Color{A: 255}}, Style: woxui.TextStyle{Size: 14}, Focused: true,
		Actions: NoteTableActions{
			Labels:         NoteTableActionLabels{InsertRow: "Insert row", DeleteTable: "Delete table"},
			OnInsertRow:    func() {},
			OnInsertColumn: func() {},
			OnDeleteRow:    func() {},
			OnDeleteColumn: func() {},
			OnDeleteTable:  func() {},
		},
	})).(woxwidget.Container)
	toolbar, ok := chrome.Child.(woxwidget.Flex)
	if !ok || toolbar.Axis != woxwidget.Horizontal || toolbar.MainAxisAlignment != woxwidget.MainAxisSpaceBetween || len(toolbar.Children) != 2 {
		t.Fatalf("table toolbar = %#v, want left structure actions and a trailing delete", chrome.Child)
	}
	left := noteTableActionIDs(toolbar.Children[0])
	right := noteTableActionIDs(toolbar.Children[1])
	if left["notes.table.demo.table-delete"] || !right["notes.table.demo.table-delete"] {
		t.Fatalf("delete placement = left %#v right %#v, want delete only on the right", left, right)
	}
	if !left["notes.table.demo.table-insert-row"] || !left["notes.table.demo.table-insert-column"] {
		t.Fatalf("left table actions = %#v, want insert controls on the left", left)
	}
}

func TestWoxNoteTableExposesStructureActions(t *testing.T) {
	table := common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{{{Text: "A"}, {Text: "B"}}, {{Text: "1"}, {Text: "2"}}}}
	ids := noteTableActionIDs(WoxNoteTable(NoteTableProps{
		ID: "notes.table.demo", Table: table, Width: 360, Theme: Theme{ResultSubtitle: woxui.Color{A: 255}}, Style: woxui.TextStyle{Size: 14}, Focused: true,
		Actions: NoteTableActions{
			Labels:         NoteTableActionLabels{InsertRow: "Insert row", DeleteTable: "Delete table"},
			OnInsertRow:    func() {},
			OnInsertColumn: func() {},
			OnDeleteRow:    func() {},
			OnDeleteColumn: func() {},
			OnDeleteTable:  func() {},
		},
	}))
	for _, id := range []string{"notes.table.demo.table-insert-row", "notes.table.demo.table-insert-column", "notes.table.demo.table-delete-row", "notes.table.demo.table-delete-column", "notes.table.demo.table-delete"} {
		if !ids[id] {
			t.Fatalf("missing table action %s in %#v", id, ids)
		}
	}
}

func noteTableActionIDs(widget woxwidget.Widget) map[string]bool {
	ids := map[string]bool{}
	collectNoteTableIDs(widget, ids)
	return ids
}

func collectNoteTableIDs(widget woxwidget.Widget, ids map[string]bool) {
	switch typed := widget.(type) {
	case woxwidget.Semantics:
		if typed.AutomationID != "" {
			ids[typed.AutomationID] = true
		}
		collectNoteTableIDs(typed.Child, ids)
	case woxwidget.Gesture:
		collectNoteTableIDs(typed.Child, ids)
	case woxwidget.Stateful:
		if child, ok := typed.Widget.(woxwidget.Widget); ok {
			collectNoteTableIDs(child, ids)
		} else if props, ok := typed.Widget.(IconButtonProps); ok {
			ids[props.ID] = true
		}
	case woxwidget.Align:
		collectNoteTableIDs(typed.Child, ids)
	case woxwidget.Container:
		collectNoteTableIDs(typed.Child, ids)
	case woxwidget.Expanded:
		collectNoteTableIDs(typed.Child, ids)
	case woxwidget.Stack:
		for _, child := range typed.Children {
			collectNoteTableIDs(child.Child, ids)
		}
	case woxwidget.ScrollView:
		collectNoteTableIDs(typed.Child, ids)
	case woxwidget.Flex:
		for _, child := range typed.Children {
			collectNoteTableIDs(child, ids)
		}
	}
}

func noteEditorColumn(t *testing.T, widget woxwidget.Widget) woxwidget.Flex {
	t.Helper()
	stateful, ok := widget.(woxwidget.Stateful)
	if !ok {
		t.Fatalf("note editor = %T, want a stateful scroll view", widget)
	}
	props, ok := stateful.Widget.(ScrollViewProps)
	if !ok {
		t.Fatalf("note editor scroll = %T, want ScrollViewProps", stateful.Widget)
	}
	inset, ok := props.Content.(woxwidget.Container)
	if !ok {
		t.Fatalf("note editor inset = %T, want a trailing-padding container", props.Content)
	}
	column, ok := inset.Child.(woxwidget.Flex)
	if !ok {
		t.Fatalf("note editor column = %T, want a vertical flex", inset.Child)
	}
	return column
}

func noteTableChrome(t *testing.T, widget woxwidget.Widget) woxwidget.Widget {
	t.Helper()
	semantics, ok := widget.(woxwidget.Semantics)
	if !ok {
		t.Fatalf("table widget = %T, want semantics", widget)
	}
	column, ok := semantics.Child.(woxwidget.Flex)
	if !ok || column.Axis != woxwidget.Vertical || len(column.Children) != 2 {
		t.Fatalf("table chrome = %#v, want a reserved toolbar above the grid", semantics.Child)
	}
	return column.Children[0]
}

func TestWoxNoteTableExposesCellFields(t *testing.T) {
	table := common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{{{Text: "A"}, {Text: "B"}}, {{Text: "1"}, {Text: "2"}}}}
	widget := WoxNoteTable(NoteTableProps{ID: "notes.table.demo", Table: table, Width: 360, Theme: Theme{}, Style: woxui.TextStyle{Size: 14}})
	semantics, ok := widget.(woxwidget.Semantics)
	if !ok || semantics.AutomationID != "notes.table.demo" {
		t.Fatalf("table widget = %#v", widget)
	}
	scroll, ok := semantics.Child.(woxwidget.ScrollView)
	if !ok || scroll.Key != "notes.table.demo-hscroll" || !scroll.Horizontal {
		t.Fatalf("table horizontal scroll = %#v", semantics.Child)
	}
	cell := scroll.Child.(woxwidget.Flex).Children[0].(woxwidget.Flex).Children[0].(woxwidget.Container)
	field := cell.Child.(woxwidget.Stateful).Widget.(TextFieldProps)
	innerHeight := field.Height - field.Padding.Top - field.Padding.Bottom
	if field.Padding == (woxwidget.Insets{}) || innerHeight < field.LineHeight || field.LineHeight < 24 {
		t.Fatalf("table cell field = height %.0f padding %#v line %.0f, want room for glyphs", field.Height, field.Padding, field.LineHeight)
	}
}

func TestWoxNoteTableCellsUseDocumentUndo(t *testing.T) {
	undone := false
	table := common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{{{Text: "A"}, {Text: "B"}}, {{Text: "1"}, {Text: "2"}}}}
	widget := WoxNoteTable(NoteTableProps{
		ID: "notes.table.demo", Table: table, Width: 360, Theme: Theme{}, Style: woxui.TextStyle{Size: 14},
		OnUndo: func() bool { undone = true; return true },
	})
	cell := widget.(woxwidget.Semantics).Child.(woxwidget.ScrollView).Child.(woxwidget.Flex).Children[0].(woxwidget.Flex).Children[0].(woxwidget.Container)
	field := cell.Child.(woxwidget.Stateful).Widget.(TextFieldProps)
	if field.OnUndo == nil || !field.OnUndo() || !undone {
		t.Fatal("table cells must use document undo so Ctrl+Z can restore a deleted column")
	}
}
