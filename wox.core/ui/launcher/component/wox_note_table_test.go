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

func TestNoteActiveFormatsForTableIgnoreOutsideBullet(t *testing.T) {
	table := common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{{{Text: "A"}}, {{Text: "1"}}}}
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "t", Type: common.NoteBlockTable, Table: &table},
		{ID: "b", Type: common.NoteBlockBullet, Text: "example"},
	}}
	formats := NoteActiveFormatsForTable(document, 0, 1, 0)
	if formats["bullet"] || !formats["table"] {
		t.Fatalf("focused table cell = %#v, want table without leftover bullet", formats)
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

func TestEnsureNoteImageEditGapsInsertsCaretParagraphs(t *testing.T) {
	document := ensureNoteImageEditGaps(common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "img", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png"}},
	}})
	segments := NoteDocumentSegments(document)
	if len(segments) != 3 || segments[0].Structural() || !segments[1].Image || segments[2].Structural() {
		t.Fatalf("image edit gaps = %#v", segments)
	}
}

func TestNoteEditorImageTapHitsCenteredPicture(t *testing.T) {
	if !noteEditorImageTapHitsPicture(woxui.Point{X: 160, Y: 40}, 320, 200, 80) {
		t.Fatal("center of the picture should select the image")
	}
	if noteEditorImageTapHitsPicture(woxui.Point{X: 8, Y: 40}, 320, 200, 80) {
		t.Fatal("padding beside the picture should move the caret instead of selecting the image")
	}
}

func TestNoteDocumentSegmentsTreatImagesAsStructural(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "img", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png", FileName: "shot.png"}},
		{ID: "p", Type: common.NoteBlockParagraph, Text: "after"},
	}}
	segments := NoteDocumentSegments(document)
	if len(segments) != 2 || !segments[0].Image || segments[0].Table || segments[1].Structural() {
		t.Fatalf("image segments = %#v", segments)
	}
	value, _, ranges := ProjectNoteDocument(document, woxui.TextStyle{Size: 14}, Theme{})
	if value != "after" || len(ranges) != 1 || ranges[0].Block != 1 {
		t.Fatalf("image projection = %q %#v", value, ranges)
	}
}

func TestWoxNoteEditorHidesImageActionsUntilSelected(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "img", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png", FileName: "shot.png", Width: 400, Height: 200}},
		{ID: "p", Type: common.NoteBlockParagraph, Text: "caption"},
	}}
	props := func(focused int) NoteEditorProps {
		return NoteEditorProps{
			ID: "notes.editor", Document: document, Width: 320, Height: 240, LineHeight: 24,
			Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 24},
			Style:   woxui.TextStyle{Size: 14}, Theme: Theme{ResultSubtitle: woxui.Color{A: 255}, PreviewText: woxui.Color{A: 255}, Cursor: woxui.Color{R: 80, G: 160, B: 255, A: 255}},
			FocusedImageBlock: focused, OnImageFocus: func(int) {}, OnImageScale: func(int, int) {}, OnImageDelete: func(int) {},
			ImageActionLabels: NoteImageActionLabels{Smaller: "Smaller", Larger: "Larger", Delete: "Delete image"},
			ResolveImage:      func(common.NoteImage) *woxui.Image { return &woxui.Image{Width: 400, Height: 200} },
		}
	}
	idle := WoxNoteEditor(props(-1))
	focused := WoxNoteEditor(props(0))
	idleIDs := noteTableActionIDs(noteEditorImageChrome(t, idle))
	if idleIDs["notes.editor.image.img.image-smaller"] || idleIDs["notes.editor.image.img.image-delete"] {
		t.Fatalf("idle image actions = %#v, want them hidden until the image is selected", idleIDs)
	}
	idleSlot, ok := noteEditorImageChrome(t, idle).(woxwidget.Container)
	if !ok || idleSlot.Height != noteEditorImageToolbarHeight || idleSlot.Width != 288 {
		t.Fatalf("idle image toolbar slot = %#v, want a reserved 288x28 row above the picture", noteEditorImageChrome(t, idle))
	}
	focusedBar, ok := noteEditorImageChrome(t, focused).(woxwidget.Container)
	if !ok || focusedBar.Height != noteEditorImageToolbarHeight || focusedBar.Width != 288 {
		t.Fatalf("focused image toolbar = %#v, want the same reserved 288x28 row so the picture does not jump", noteEditorImageChrome(t, focused))
	}
	focusedIDs := noteTableActionIDs(noteEditorImageChrome(t, focused))
	for _, id := range []string{"notes.editor.image.img.image-smaller", "notes.editor.image.img.image-larger", "notes.editor.image.img.image-delete"} {
		if !focusedIDs[id] {
			t.Fatalf("focused image missing %s in %#v", id, focusedIDs)
		}
	}
	idlePicture := noteEditorImagePictureBox(t, idle)
	focusedPicture := noteEditorImagePictureBox(t, focused)
	if idlePicture.BorderWidth != 0 || focusedPicture.BorderWidth != 2 {
		t.Fatalf("image highlight idle/focused = %.0f/%.0f, want 0 then 2", idlePicture.BorderWidth, focusedPicture.BorderWidth)
	}
}

func TestWoxNoteEditorPlacesImageDeleteOnTheRight(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "img", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png", FileName: "shot.png", Width: 400, Height: 200}},
	}}
	chrome := noteEditorImageChrome(t, WoxNoteEditor(NoteEditorProps{
		ID: "notes.editor", Document: document, Width: 320, Height: 240, LineHeight: 24,
		Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 24},
		Style:   woxui.TextStyle{Size: 14}, Theme: Theme{ResultSubtitle: woxui.Color{A: 255}, PreviewText: woxui.Color{A: 255}},
		FocusedImageBlock: 0, OnImageFocus: func(int) {}, OnImageScale: func(int, int) {}, OnImageDelete: func(int) {},
		ImageActionLabels: NoteImageActionLabels{Smaller: "Smaller", Larger: "Larger", Delete: "Delete image"},
		ResolveImage:      func(common.NoteImage) *woxui.Image { return &woxui.Image{Width: 400, Height: 200} },
	})).(woxwidget.Container)
	toolbar, ok := chrome.Child.(woxwidget.Flex)
	if !ok || toolbar.Axis != woxwidget.Horizontal || toolbar.MainAxisAlignment != woxwidget.MainAxisSpaceBetween || len(toolbar.Children) != 2 {
		t.Fatalf("image toolbar = %#v, want left scale actions and a trailing delete", chrome.Child)
	}
	left := noteTableActionIDs(toolbar.Children[0])
	right := noteTableActionIDs(toolbar.Children[1])
	if left["notes.editor.image.img.image-delete"] || !right["notes.editor.image.img.image-delete"] {
		t.Fatalf("delete placement = left %#v right %#v, want delete only on the right", left, right)
	}
	if !left["notes.editor.image.img.image-smaller"] || !left["notes.editor.image.img.image-larger"] {
		t.Fatalf("left image actions = %#v, want scale controls on the left", left)
	}
}

func TestWoxNoteEditorKeepsImageSkeletonWithoutFilename(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "img", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png", FileName: "clipboard.png", Width: 400, Height: 200}},
	}}
	editor := WoxNoteEditor(NoteEditorProps{
		ID: "notes.editor", Document: document, Width: 320, Height: 240, LineHeight: 24,
		Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 24},
		Style:   woxui.TextStyle{Size: 14}, Theme: Theme{PreviewText: woxui.Color{A: 255}},
		MissingImageLabel: "Image is missing",
	})
	if _, ok := findNoteEditorTextBlock(noteEditorColumn(t, editor)); ok {
		t.Fatal("sized image should reserve a blank skeleton, not filename or missing-image text")
	}
}

func findNoteEditorTextBlock(widget woxwidget.Widget) (woxwidget.TextBlock, bool) {
	switch typed := widget.(type) {
	case woxwidget.TextBlock:
		return typed, true
	case woxwidget.Semantics:
		return findNoteEditorTextBlock(typed.Child)
	case woxwidget.Gesture:
		return findNoteEditorTextBlock(typed.Child)
	case woxwidget.Stateful:
		if child, ok := typed.Widget.(woxwidget.Widget); ok {
			return findNoteEditorTextBlock(child)
		}
	case woxwidget.Align:
		return findNoteEditorTextBlock(typed.Child)
	case woxwidget.Container:
		return findNoteEditorTextBlock(typed.Child)
	case woxwidget.Expanded:
		return findNoteEditorTextBlock(typed.Child)
	case woxwidget.Flex:
		for _, child := range typed.Children {
			if found, ok := findNoteEditorTextBlock(child); ok {
				return found, true
			}
		}
	}
	return woxwidget.TextBlock{}, false
}

func TestWoxNoteEditorRendersImageSegments(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "img", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png", FileName: "shot.png", Width: 400, Height: 200}},
		{ID: "p", Type: common.NoteBlockParagraph, Text: "caption"},
	}}
	editor := WoxNoteEditor(NoteEditorProps{
		ID: "notes.editor", Document: document, Width: 320, Height: 240, LineHeight: 24,
		Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 24},
		Style:   woxui.TextStyle{Size: 14}, Theme: Theme{ResultSubtitle: woxui.Color{A: 255}, PreviewText: woxui.Color{A: 255}},
		ResolveImage: func(common.NoteImage) *woxui.Image {
			return &woxui.Image{Width: 400, Height: 200}
		},
	})
	column := noteEditorColumn(t, editor)
	if column.Gap != noteEditorSegmentGap || len(column.Children) != 2 {
		t.Fatalf("image editor column = %#v, want image then text", column)
	}
	if _, ok := column.Children[1].(woxwidget.Stateful); !ok {
		t.Fatalf("trailing text = %T, want a text field", column.Children[1])
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

func noteEditorImageChrome(t *testing.T, widget woxwidget.Widget) woxwidget.Widget {
	t.Helper()
	box, ok := noteEditorColumn(t, widget).Children[0].(woxwidget.Container)
	if !ok {
		t.Fatalf("image slot = %T, want a padded container", noteEditorColumn(t, widget).Children[0])
	}
	column, ok := box.Child.(woxwidget.Flex)
	if !ok || column.Axis != woxwidget.Vertical || len(column.Children) != 2 {
		t.Fatalf("image chrome = %#v, want a reserved toolbar above the picture", box.Child)
	}
	return column.Children[0]
}

func noteEditorImagePictureBox(t *testing.T, widget woxwidget.Widget) woxwidget.Container {
	t.Helper()
	box, ok := noteEditorColumn(t, widget).Children[0].(woxwidget.Container)
	if !ok {
		t.Fatalf("image slot = %T, want a padded container", noteEditorColumn(t, widget).Children[0])
	}
	column, ok := box.Child.(woxwidget.Flex)
	if !ok || len(column.Children) != 2 {
		t.Fatalf("image chrome = %#v, want toolbar plus picture", box.Child)
	}
	picture := column.Children[1]
	for {
		switch typed := picture.(type) {
		case woxwidget.Semantics:
			picture = typed.Child
		case woxwidget.Gesture:
			picture = typed.Child
		default:
			container, ok := picture.(woxwidget.Container)
			if !ok {
				t.Fatalf("image picture = %T, want a highlight container", picture)
			}
			return container
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
	scroll := noteTableScroll(t, widget)
	if scroll.Key != "notes.table.demo-hscroll" || !scroll.Horizontal {
		t.Fatalf("table horizontal scroll = %#v", scroll)
	}
	cell := noteTableFirstCell(t, widget)
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
	field := noteTableFirstCell(t, widget).Child.(woxwidget.Stateful).Widget.(TextFieldProps)
	if field.OnUndo == nil || !field.OnUndo() || !undone {
		t.Fatal("table cells must use document undo so Ctrl+Z can restore a deleted column")
	}
}

func TestWoxNoteTableUsesCollapsedGridLines(t *testing.T) {
	theme := Theme{PreviewSplit: woxui.Color{R: 90, G: 90, B: 90, A: 255}, PreviewText: woxui.Color{A: 255}}
	table := common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{{{Text: "A"}, {Text: "B"}}, {{Text: "1"}, {Text: "2"}}}}
	widget := WoxNoteTable(NoteTableProps{ID: "notes.table.demo", Table: table, Width: 360, Theme: theme, Style: woxui.TextStyle{Size: 14}})
	frame := widget.(woxwidget.Semantics).Child.(woxwidget.Stack)
	if len(frame.Children) != 2 {
		t.Fatalf("note table frame children = %d, want content plus one outer stroke", len(frame.Children))
	}
	outline := frame.Children[1].Child.(woxwidget.Container)
	if outline.BorderWidth != TableGridBorderWidth || outline.BorderColor != noteTableBorder(theme) || outline.Color.A != 0 {
		t.Fatalf("note table stroke = %#v, want the shared 1px frame", outline)
	}

	header := noteTableCellAt(t, widget, 0, 0)
	if header.BorderWidth != 0 || header.RightBorderWidth != TableGridBorderWidth || header.BottomBorderWidth != TableGridBorderWidth {
		t.Fatalf("header cell = %#v, want collapsed right+bottom", header)
	}
	last := noteTableCellAt(t, widget, 1, 1)
	if last.BorderWidth != 0 || last.RightBorderWidth != 0 || last.BottomBorderWidth != 0 {
		t.Fatalf("last cell = %#v, want the outer frame to own that corner", last)
	}
}

func noteTableScroll(t *testing.T, widget woxwidget.Widget) woxwidget.ScrollView {
	t.Helper()
	child := widget.(woxwidget.Semantics).Child
	if column, ok := child.(woxwidget.Flex); ok && column.Axis == woxwidget.Vertical && len(column.Children) == 2 {
		child = column.Children[1]
	}
	frame, ok := child.(woxwidget.Stack)
	if !ok || len(frame.Children) < 1 {
		t.Fatalf("table grid = %T, want a stacked outer frame", child)
	}
	scroll, ok := frame.Children[0].Child.(woxwidget.ScrollView)
	if !ok {
		t.Fatalf("table grid body = %T, want a horizontal scroll view", frame.Children[0].Child)
	}
	return scroll
}

func noteTableFirstCell(t *testing.T, widget woxwidget.Widget) woxwidget.Container {
	t.Helper()
	return noteTableCellAt(t, widget, 0, 0)
}

func noteTableCellAt(t *testing.T, widget woxwidget.Widget, row, column int) woxwidget.Container {
	t.Helper()
	rows := noteTableScroll(t, widget).Child.(woxwidget.Flex)
	if row < 0 || row >= len(rows.Children) {
		t.Fatalf("table row %d, have %d", row, len(rows.Children))
	}
	cells := rows.Children[row].(woxwidget.Flex)
	if column < 0 || column >= len(cells.Children) {
		t.Fatalf("table column %d, have %d", column, len(cells.Children))
	}
	cell, ok := cells.Children[column].(woxwidget.Container)
	if !ok {
		t.Fatalf("table cell = %T, want a container", cells.Children[column])
	}
	return cell
}
