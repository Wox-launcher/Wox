package launcher

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"wox/common"
	notesplugin "wox/plugin/system/notes"
	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/screen"
)

type notesWindowTestServices struct {
	contract.Services
	mu          sync.Mutex
	record      common.NoteRecord
	saveErr     error
	saveID      string
	conflict    bool
	saveCall    chan struct{}
	discardCall chan struct{}
	discardIDs  []string
	local       map[string]string
	records     map[string]common.NoteRecord
}

func TestClampNotesBoundsUsesLogicalMixedDPIDisplayWorkAreas(t *testing.T) {
	displays := []screen.Display{
		{ID: "left", WorkArea: screen.Rect{X: -1280, Y: 0, Width: 1280, Height: 720}, Scale: 1.25},
		{ID: "main", WorkArea: screen.Rect{X: 0, Y: 0, Width: 1920, Height: 1040}, Scale: 2, Primary: true},
	}
	got := clampNotesBoundsToDisplays(woxui.Rect{X: -1400, Y: 650, Width: notesDefaultWidth, Height: 700}, displays)
	if got.X != -1280 || got.Y != 80 || got.Width != notesDefaultWidth || got.Height != 640 {
		t.Fatalf("logical bounds were not clamped on negative-origin display: %#v", got)
	}
	got = clampNotesBoundsToDisplays(woxui.Rect{X: 2500, Y: -100, Width: notesDefaultWidth, Height: 320}, displays)
	if got.X != 1920-notesDefaultWidth || got.Y != 0 {
		t.Fatalf("disconnected display bounds were not recovered: %#v", got)
	}
}

func TestClampNotesBoundsEnforcesMinimumWidth(t *testing.T) {
	if notesMinimumWidth != 460 || notesDefaultWidth < notesMinimumWidth {
		t.Fatalf("notes width = default %.0f min %.0f, want a 460-unit minimum", notesDefaultWidth, notesMinimumWidth)
	}
	got := clampNotesBoundsToDisplays(woxui.Rect{X: 40, Y: 60, Width: 320, Height: 320}, []screen.Display{
		{ID: "main", WorkArea: screen.Rect{X: 0, Y: 0, Width: 1920, Height: 1040}, Scale: 1, Primary: true},
	})
	if got.Width != notesMinimumWidth {
		t.Fatalf("clamped width = %.0f, want minimum %.0f", got.Width, notesMinimumWidth)
	}
}

func TestNotesNativeMinSizeMatchesWindowFloor(t *testing.T) {
	got := notesNativeMinSize()
	if got.Width != notesMinimumWidth || got.Height != notesMinimumHeight {
		t.Fatalf("native min size = %+v, want width %.0f height %.0f", got, notesMinimumWidth, notesMinimumHeight)
	}
}

func (s *notesWindowTestServices) NotesSave(_ context.Context, id, _ string, document common.NoteDocument) (common.NoteSaveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return common.NoteSaveResult{}, s.saveErr
	}
	s.record.ID, s.record.Document, s.record.Revision = id, document, "saved"
	if s.saveID != "" {
		s.record.ID = s.saveID
	}
	select {
	case s.saveCall <- struct{}{}:
	default:
	}
	return common.NoteSaveResult{Record: s.record, Conflict: s.conflict}, nil
}

func TestNotesConflictRebindsOnlyTheEditedWindow(t *testing.T) {
	services := &notesWindowTestServices{saveID: "conflict", conflict: true, saveCall: make(chan struct{}, 1)}
	app := &App{services: services, palette: defaultPalette(), lifecycleCtx: context.Background(), noteWindows: map[string]*notesWindowController{}}
	record := common.NoteRecord{ID: "original", Revision: "initial", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}}}
	controller := newNotesWindowController(app, record)
	windowID := controller.windowID
	app.noteWindows[record.ID] = controller
	controller.document.Blocks[0].Text = "local edit"
	controller.dirty = true

	if err := controller.flush(); err != nil {
		t.Fatalf("flush conflict: %v", err)
	}
	if app.noteWindows["original"] != nil || app.noteWindows["conflict"] != controller {
		t.Fatalf("conflict window map = %#v, want controller rebound to conflict note", app.noteWindows)
	}
	if controller.windowID != windowID {
		t.Fatalf("native window ID changed from %q to %q during note conflict", windowID, controller.windowID)
	}
}

func TestNotesMenuRowUsesHoverHighlight(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	theme := woxcomponent.Theme{
		ActionBackground: woxui.Color{R: 40, G: 40, B: 40, A: 255},
		ActionText:       woxui.Color{R: 240, G: 240, B: 240, A: 255},
	}
	row := controller.menuRow("pin", "Pin note", 190, theme, func() {})
	semantics := row.(woxwidget.Semantics)
	if semantics.AutomationID != "notes.menu.pin" {
		t.Fatalf("menu automation id = %q, want notes.menu.pin", semantics.AutomationID)
	}
	if _, ok := semantics.Child.(woxwidget.Stateful); !ok {
		t.Fatalf("menu row child = %T, want hoverable Stateful for highlight", semantics.Child)
	}
}

func TestNotesSearchRowUsesHoverHighlight(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	theme := woxcomponent.Theme{
		ActionBackground: woxui.Color{R: 40, G: 40, B: 40, A: 255},
		ActionText:       woxui.Color{R: 240, G: 240, B: 240, A: 255},
	}
	row := controller.notesListRow(common.NoteSummary{ID: "note-a", Title: "Hello", Preview: "preview", UpdatedAt: 1_700_000_000_000}, 300, theme, false)
	semantics := row.(woxwidget.Semantics)
	if semantics.AutomationID != "notes.search.note-a" {
		t.Fatalf("search automation id = %q, want notes.search.note-a", semantics.AutomationID)
	}
	if _, ok := semantics.Child.(woxwidget.Stateful); !ok {
		t.Fatalf("search row child = %T, want hoverable Stateful for highlight", semantics.Child)
	}
}

func TestNotesSearchRowShowsCenteredUpdatedTime(t *testing.T) {
	updated := int64(1_700_000_000_000)
	titleColor := woxui.Color{R: 240, G: 240, B: 240, A: 255}
	metaColor := woxui.Color{R: 160, G: 160, B: 160, A: 255}
	child := notesSearchRowChild("Hello", util.FormatTimestamp(updated), titleColor, metaColor)
	align, ok := child.(woxwidget.Align)
	if !ok || align.Vertical != .5 || align.Height != notesSearchRowHeight {
		t.Fatalf("search row alignment = %#v, want vertically centered 36-high slot", child)
	}
	flex, ok := align.Child.(woxwidget.Flex)
	if !ok || len(flex.Children) != 2 {
		t.Fatalf("search row content = %#v, want title and updated time", align.Child)
	}
	title, ok := flex.Children[0].(woxwidget.Expanded).Child.(woxwidget.TextBlock)
	if !ok || title.Value != "Hello" || title.AlignmentY != .5 || title.Height != notesSearchTextHeight {
		t.Fatalf("search title = %#v, want optically centered Hello", flex.Children[0])
	}
	meta, ok := flex.Children[1].(woxwidget.TextBlock)
	if !ok || meta.Value != util.FormatTimestamp(updated) || meta.AlignmentY != .5 || meta.Value == "preview" {
		t.Fatalf("search meta = %#v, want optically centered updated time", flex.Children[1])
	}
}

func TestNotesSearchSectionIsDistinctFromRows(t *testing.T) {
	theme := woxcomponent.Theme{
		ActionHeader: woxui.Color{R: 140, G: 146, B: 154, A: 255},
		ActionText:   woxui.Color{R: 240, G: 240, B: 240, A: 255},
	}
	first := notesSearchSection("Pinned", 280, theme, true)
	follow := notesSearchSection("Recent", 280, theme, false)
	firstBox, ok := first.(woxwidget.Container)
	if !ok || firstBox.Height != notesSearchSectionHeight || firstBox.Padding.Top != 0 {
		t.Fatalf("first section = %#v, want a 28-high header without lead-in", first)
	}
	followBox, ok := follow.(woxwidget.Container)
	if !ok || followBox.Height != notesSearchSectionHeight+notesSearchSectionLead || followBox.Padding.Top != notesSearchSectionLead {
		t.Fatalf("follow section = %#v, want 12 extra lead-in before the next group", follow)
	}
	label := firstBox.Child.(woxwidget.Align).Child.(woxwidget.TextBlock)
	if label.Style.Size != woxcomponent.SettingsSectionTitleFontSize || label.Style.Weight != woxui.FontWeightSemibold || label.Color != theme.ActionHeader {
		t.Fatalf("section label = %#v, want 11 semibold ActionHeader", label)
	}
	row := notesSearchRowChild("Hello", "2026-08-24 15:43:57", theme.ActionText, theme.ActionHeader)
	title := row.(woxwidget.Align).Child.(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.TextBlock)
	if title.Style.Size != 13 || title.Style.Weight != woxui.FontWeightRegular || title.Color != theme.ActionText {
		t.Fatalf("row title = %#v, want 13 regular action text distinct from the section label", title)
	}
}

func TestNotesDeleteColumnUndoRestoresTheColumn(t *testing.T) {
	app := &App{palette: defaultPalette()}
	table := common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{{{Text: "A"}, {Text: "B"}}, {{Text: "1"}, {Text: "2"}}}}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
			{ID: "t", Type: common.NoteBlockTable, Table: &table},
		}},
	})
	controller.focusedTableBlock, controller.focusedTableRow, controller.focusedTableCol = 0, 0, 1
	controller.tableDeleteColumn(0)
	if got := noteTableColumnCount(*controller.document.Blocks[0].Table); got != 1 {
		t.Fatalf("delete column = %d columns, want 1", got)
	}
	if !controller.onKey(woxui.KeyEvent{Key: woxui.Key("z"), Modifiers: woxui.KeyModifierControl | woxui.KeyModifierMeta, Down: true}) {
		t.Fatal("Ctrl+Z should undo a table column delete after the toolbar keeps window focus")
	}
	restored := controller.document.Blocks[0].Table
	if restored == nil || noteTableColumnCount(*restored) != 2 || restored.Rows[0][1].Text != "B" {
		t.Fatalf("undone table = %#v, want both columns restored", restored)
	}
}

func TestNotesTableActionsWorkWithoutCellFocus(t *testing.T) {
	app := &App{palette: defaultPalette()}
	table := common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{{{Text: "A"}, {Text: "B"}}, {{Text: "1"}, {Text: "2"}}}}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
			{ID: "p", Type: common.NoteBlockParagraph, Text: "before"},
			{ID: "t", Type: common.NoteBlockTable, Table: &table},
		}},
	})
	controller.focusedTableBlock = -1
	controller.tableInsertRow(1)
	if got := len(controller.document.Blocks[1].Table.Rows); got != 3 {
		t.Fatalf("insert row without focus = %d rows, want 3", got)
	}
	controller.focusedTableBlock = -1
	controller.tableDelete(1)
	for _, block := range controller.document.Blocks {
		if block.Type == common.NoteBlockTable {
			t.Fatalf("delete table without focus left a table: %#v", controller.document.Blocks)
		}
	}
}

func TestNotesSearchHighlightsFirstResultAndMovesWithArrows(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	controller.summaries = []common.NoteSummary{
		{ID: "note-a", Title: "First", UpdatedAt: 2, PinnedAt: 1},
		{ID: "note-b", Title: "Second", UpdatedAt: 1},
	}
	theme := defaultPalette().componentTheme()
	selected := collectNotesSelectedIDs(controller.buildSearchOverlay(woxui.Size{Width: 420, Height: 320}, theme))
	if !selected["notes.search.note-a"] || selected["notes.search.note-b"] {
		t.Fatalf("default selection = %#v, want first result", selected)
	}
	if !controller.onSearchKey(woxui.KeyEvent{Key: woxui.KeyArrowDown, Down: true}) || controller.searchIndex != 1 {
		t.Fatalf("arrow down index = %d, want 1", controller.searchIndex)
	}
	selected = collectNotesSelectedIDs(controller.buildSearchOverlay(woxui.Size{Width: 420, Height: 320}, theme))
	if !selected["notes.search.note-b"] || selected["notes.search.note-a"] {
		t.Fatalf("arrow selection = %#v, want second result", selected)
	}
	if !controller.onSearchKey(woxui.KeyEvent{Key: woxui.KeyArrowUp, Down: true}) || controller.searchIndex != 0 {
		t.Fatalf("arrow up index = %d, want 0", controller.searchIndex)
	}
}

func TestNotesSearchUsesSharedScrollbar(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	controller.summaries = make([]common.NoteSummary, 8)
	for index := range controller.summaries {
		controller.summaries[index] = common.NoteSummary{ID: fmt.Sprintf("note-%d", index), Title: fmt.Sprintf("Note %d", index), UpdatedAt: int64(8 - index)}
	}
	props, ok := notesSearchScrollProps(controller.buildSearchOverlay(woxui.Size{Width: 420, Height: 320}, defaultPalette().componentTheme()))
	if !ok || props.Key != "notes.search.results" || props.ContentHeight <= props.Height || props.AlwaysShowScrollbar || props.KeepVisible == nil {
		t.Fatalf("search scroll = %#v, want overflowing fading WoxScrollView", props)
	}
}

func TestNotesSearchSwitchesNoteInCurrentWindow(t *testing.T) {
	target := common.NoteRecord{
		ID: "target", Revision: "r1",
		Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph, Text: "other note"}}},
	}
	services := &notesWindowTestServices{records: map[string]common.NoteRecord{"target": target}}
	app := &App{services: services, palette: defaultPalette(), noteWindows: map[string]*notesWindowController{}}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "current", Revision: "r0",
		Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph, Text: "this note"}}},
	})
	windowID := controller.windowID
	app.noteWindows[controller.record.ID] = controller
	controller.searchOpen = true

	controller.openSearchItem(common.NoteSummary{ID: "target", Title: "other note"})
	if controller.record.ID != "target" {
		t.Fatalf("search switched to %q, want target in the same window", controller.record.ID)
	}
	if controller.windowID != windowID {
		t.Fatalf("search opened a new native window %q", controller.windowID)
	}
	if app.noteWindows["current"] != nil || app.noteWindows["target"] != controller {
		t.Fatalf("window map = %#v, want current window rebound to target", app.noteWindows)
	}
	if controller.searchOpen {
		t.Fatal("search overlay stayed open after switching")
	}
	if controller.editor.Text() != "other note" {
		t.Fatalf("editor = %q, want the selected note in this window", controller.editor.Text())
	}
}

func TestNotesSearchRestoresDeletedNoteInCurrentWindow(t *testing.T) {
	target := common.NoteRecord{
		ID: "trashed", Revision: "r1", DeletedAt: 1,
		Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph, Text: "trashed note"}}},
	}
	services := &notesWindowTestServices{records: map[string]common.NoteRecord{"trashed": target}}
	app := &App{services: services, palette: defaultPalette(), noteWindows: map[string]*notesWindowController{}}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "current", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph, Text: "this note"}}},
	})
	windowID := controller.windowID
	app.noteWindows[controller.record.ID] = controller

	controller.openSearchItem(common.NoteSummary{ID: "trashed", Title: "trashed note", DeletedAt: 1})
	if controller.record.ID != "trashed" || controller.record.DeletedAt != 0 {
		t.Fatalf("restored note = id %q deleted %d, want trashed in this window", controller.record.ID, controller.record.DeletedAt)
	}
	if controller.windowID != windowID || app.noteWindows["trashed"] != controller {
		t.Fatalf("restore opened another window: id %q map %#v", controller.windowID, app.noteWindows)
	}
	if controller.editor.Text() != "trashed note" {
		t.Fatalf("editor = %q, want restored note text", controller.editor.Text())
	}
}

func TestNotesSearchResetsSelectionWhenOpened(t *testing.T) {
	services := &notesWindowTestServices{}
	app := &App{services: services, palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{ID: "note"})
	controller.searchIndex = 4
	controller.toggleSearch()
	if !controller.searchOpen || controller.searchIndex != 0 {
		t.Fatalf("open search index = %d open=%t, want 0 / true", controller.searchIndex, controller.searchOpen)
	}
}

func TestNotesToolbarRemovesHistoryControls(t *testing.T) {
	app := &App{palette: defaultPalette(), noteWindows: map[string]*notesWindowController{}}
	record := common.NoteRecord{ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}}}
	controller := newNotesWindowController(app, record)
	toolbar := controller.buildToolbar(420, true, woxcomponent.Theme{}).(woxwidget.Container).Child.(woxwidget.Stack)
	controls := map[woxwidget.Key]bool{}
	labels := map[woxwidget.Key]string{}
	for _, child := range toolbar.Children {
		if row, ok := child.Child.(woxwidget.Flex); ok {
			for _, control := range row.Children {
				if stateful, ok := control.(woxwidget.Stateful); ok {
					controls[stateful.Key] = true
					labels[stateful.Key] = stateful.Widget.(woxcomponent.IconButtonProps).Label
				}
			}
		}
	}
	if controls["notes.toolbar.back"] || controls["notes.toolbar.forward"] || !controls["notes.toolbar.new"] || !controls["notes.toolbar.pin"] {
		t.Fatalf("Notes title-bar controls = %#v, want new and pin without history navigation", controls)
	}
	if !controls["notes.toolbar.search"] {
		t.Fatalf("Notes title-bar controls = %#v, want search beside pin", controls)
	}
	var pinIcon woxwidget.Widget
	for _, child := range toolbar.Children {
		row, ok := child.Child.(woxwidget.Flex)
		if !ok {
			continue
		}
		for _, control := range row.Children {
			stateful, ok := control.(woxwidget.Stateful)
			if !ok || stateful.Key != "notes.toolbar.pin" {
				continue
			}
			pinIcon = stateful.Widget.(woxcomponent.IconButtonProps).Icon
		}
	}
	if image, ok := pinIcon.(woxwidget.Image); !ok || image.Source == nil || image.Width != 15 || image.Height != 15 {
		t.Fatalf("pin icon = %#v, want 15x15 thumbtack SVG beside search", pinIcon)
	}
	for key, hotkey := range map[woxwidget.Key]string{"notes.toolbar.search": primaryHotkey("p"), "notes.toolbar.pin": primaryHotkey("shift+p"), "notes.toolbar.new": primaryHotkey("n")} {
		shortcut := "(" + strings.Join(formatHotkeyLabels(hotkey), "+") + ")"
		if !strings.HasSuffix(labels[key], shortcut) {
			t.Fatalf("Notes title-bar label %q = %q, want shortcut suffix %q", key, labels[key], shortcut)
		}
	}
	title := toolbar.Children[2]
	alignment := title.Child.(woxwidget.Align)
	wantLeft, wantRight, wantAlign := notesTitleSlot(runtime.GOOS, woxcomponent.TitleBarChromeWidth(runtime.GOOS, true, true))
	if title.Left != wantLeft || title.Right != wantRight || alignment.Horizontal != wantAlign {
		t.Fatalf("Notes title slot = %.0f/%.0f alignment %.1f, want %.0f/%.0f/%.1f", title.Left, title.Right, alignment.Horizontal, wantLeft, wantRight, wantAlign)
	}
	label, ok := alignment.Child.(woxwidget.TextBlock)
	if !ok || label.MaxLines != 1 || !label.ShrinkWrap || label.AlignmentY != 0.5 {
		t.Fatalf("Notes title = %#v, want a single-line ellipsized TextBlock", alignment.Child)
	}
	if runtime.GOOS != "darwin" {
		icon := toolbar.Children[4]
		iconAlignment, ok := icon.Child.(woxwidget.Align)
		iconImage, imageOK := iconAlignment.Child.(woxwidget.Image)
		if !ok || !imageOK || icon.Left != 12 || iconAlignment.Width != 20 || iconImage.Source == nil || iconImage.Width != 20 || iconImage.Height != 20 {
			t.Fatalf("Notes title-bar icon slot = %#v, want 20x20 icon at left 12", icon)
		}
	}
}

func TestNotesTitleSlotLeavesRoomAtDefaultWidth(t *testing.T) {
	left, right, alignment := notesTitleSlot("darwin", 0)
	if left != notesToolbarActionsWidth || right != notesToolbarActionsWidth || alignment != 0.5 {
		t.Fatalf("macOS title slot = %.0f/%.0f/%.1f, want equal %.0f insets", left, right, alignment, notesToolbarActionsWidth)
	}
	if room := notesDefaultWidth - left - right; room < 80 {
		t.Fatalf("macOS title room at default width = %.0f, want enough for a short CJK title plus ellipsis", room)
	}
	left, right, alignment = notesTitleSlot("windows", woxcomponent.TitleBarChromeWidth("windows", true, true))
	if left != 40 || alignment != 0 || right != woxcomponent.TitleBarChromeWidth("windows", true, true)+notesToolbarActionsWidth {
		t.Fatalf("Windows title slot = %.0f/%.0f/%.1f, want a leading cluster", left, right, alignment)
	}
	left, right, alignment = notesTitleSlot("linux", woxcomponent.TitleBarChromeWidth("linux", true, true))
	if left != 40 || alignment != 0 || right != woxcomponent.TitleBarChromeWidth("linux", true, true)+notesToolbarActionsWidth {
		t.Fatalf("Linux title slot = %.0f/%.0f/%.1f, want a leading cluster", left, right, alignment)
	}
}

func TestNotesImageActionsScaleAndDelete(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
			{ID: "img", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png", FileName: "shot.png"}},
			{ID: "p", Type: common.NoteBlockParagraph, Text: "caption"},
		}},
	})
	imageIndex := notesTestImageIndex(controller.document)
	controller.scaleImage(imageIndex, -10)
	if got := notesTestImageBlock(controller.document).Scale; got != 90 {
		t.Fatalf("image scale = %d, want 90", got)
	}
	controller.deleteImage(imageIndex)
	if notesTestImageBlock(controller.document) != nil {
		t.Fatalf("delete image = %#v, want the remaining caption", controller.document.Blocks)
	}
	hasCaption := false
	for _, block := range controller.document.Blocks {
		if strings.Contains(block.Text, "caption") {
			hasCaption = true
			break
		}
	}
	if !hasCaption {
		t.Fatalf("delete image = %#v, want the remaining caption", controller.document.Blocks)
	}
}

func TestNotesMoreMenuOmitsListPin(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	overlay := controller.buildMoreOverlay(woxui.Size{Width: 420, Height: 320}, woxcomponent.Theme{})
	ids := collectNotesAutomationIDs(overlay)
	if ids["notes.menu.pin"] {
		t.Fatal("list pin must live on launcher result actions, not the Notes more menu")
	}
	if !ids["notes.menu.copy-link"] {
		t.Fatalf("more menu ids = %#v, want copy-link after removing list pin", ids)
	}
	if !ids["notes.menu.view"] {
		t.Fatalf("more menu ids = %#v, want the Markdown/preview toggle", ids)
	}
}

func TestNotesMoreMenuViewLabelFollowsEditorMode(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph, Text: "hello"}}},
	})
	shortcut := strings.Join(formatHotkeyLabels(primaryHotkey("e")), "+")
	preview := controller.viewModeMenuLabel()
	if !strings.Contains(preview, shortcut) || !strings.Contains(strings.ToLower(preview), "markdown") {
		t.Fatalf("preview-mode view label = %q, want Markdown plus %s", preview, shortcut)
	}
	controller.markdownView = true
	source := controller.viewModeMenuLabel()
	if !strings.Contains(source, shortcut) || !strings.Contains(strings.ToLower(source), "preview") {
		t.Fatalf("markdown-mode view label = %q, want Preview plus %s", source, shortcut)
	}
}

func TestNotesTogglesMarkdownViewWithPrimaryE(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
			{ID: "h", Type: common.NoteBlockHeading1, Text: "Title"},
			{ID: "p", Type: common.NoteBlockParagraph, Text: "body"},
		}},
	})
	event := woxui.KeyEvent{Key: woxui.Key("e"), Modifiers: woxui.KeyModifierControl | woxui.KeyModifierMeta, Down: true}
	if !controller.onKey(event) || !controller.markdownView {
		t.Fatal("Ctrl/Cmd+E should open the Markdown source view")
	}
	if got := controller.editor.Text(); got != "# Title\n\nbody" {
		t.Fatalf("markdown source = %q", got)
	}
	if !controller.onKey(event) || controller.markdownView {
		t.Fatal("Ctrl/Cmd+E should return to the preview editor")
	}
	if got := controller.editor.Text(); got != "Title\nbody" {
		t.Fatalf("preview text = %q", got)
	}
}

func TestNotesMarkdownViewRoundTripsTables(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
			{ID: "p", Type: common.NoteBlockParagraph, Text: "intro"},
			{ID: "t", Type: common.NoteBlockTable, Table: &common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{
				{{Text: "A"}, {Text: "B"}},
				{{Text: "1"}, {Text: "2"}},
			}}},
		}},
	})
	controller.toggleMarkdownView()
	controller.toggleMarkdownView()
	if len(controller.document.Blocks) != 3 || controller.document.Blocks[1].Type != common.NoteBlockTable || controller.document.Blocks[1].Table == nil {
		t.Fatalf("preview after Ctrl+E = %#v, want the table to stay structural", controller.document.Blocks)
	}
	if tail := controller.document.Blocks[2]; tail.Type != common.NoteBlockParagraph || tail.Text != "" {
		t.Fatalf("preview after Ctrl+E needs an empty paragraph below the table: %+v", tail)
	}
	if got := controller.document.Blocks[1].Table.Rows[1][1].Text; got != "2" {
		t.Fatalf("table cell after Ctrl+E = %q", got)
	}
}

func TestNotesMarkdownViewRoundTripsImages(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
			{ID: "img", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png", FileName: "clipboard.png", Width: 800, Height: 400}},
		}},
	})
	controller.toggleMarkdownView()
	if !strings.Contains(controller.editor.Text(), "notes-image:shot.png") || !strings.Contains(controller.editor.Text(), "width=800") {
		t.Fatalf("markdown source = %q", controller.editor.Text())
	}
	controller.toggleMarkdownView()
	image := notesTestImageBlock(controller.document)
	if image == nil || image.ID != "shot.png" {
		t.Fatalf("preview after Ctrl+E = %#v, want the image block kept", controller.document.Blocks)
	}
	if image.Width != 800 || image.Height != 400 {
		t.Fatalf("image size after Ctrl+E = %#v", image)
	}
}

func TestNotesArrowKeysLeaveAFocusedImage(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
			{ID: "img", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png", FileName: "shot.png"}},
		}},
	})
	imageIndex := notesTestImageIndex(controller.document)
	controller.focusedImageBlock = imageIndex
	if !controller.onKey(woxui.KeyEvent{Key: woxui.KeyArrowRight, Down: true}) || controller.focusedImageBlock >= 0 {
		t.Fatal("right arrow should move the caret below the image")
	}
	if controller.activeTextSegment.Start <= imageIndex {
		t.Fatalf("active text after right arrow = %#v, want the paragraph below the image", controller.activeTextSegment)
	}
	controller.focusedImageBlock = notesTestImageIndex(controller.document)
	if !controller.onKey(woxui.KeyEvent{Key: woxui.KeyArrowLeft, Down: true}) || controller.focusedImageBlock >= 0 {
		t.Fatal("left arrow should move the caret above the image")
	}
	if controller.activeTextSegment.Start >= notesTestImageIndex(controller.document) {
		t.Fatalf("active text after left arrow = %#v, want the paragraph above the image", controller.activeTextSegment)
	}
}

// TestNotesListKeyboardAfterStructuralBlocks verifies caret offsets belong to the active text segment.
func TestNotesListKeyboardAfterStructuralBlocks(t *testing.T) {
	for _, prefix := range []common.NoteBlockType{common.NoteBlockParagraph, common.NoteBlockTable, common.NoteBlockImage} {
		for _, kind := range []common.NoteBlockType{common.NoteBlockBullet, common.NoteBlockOrdered, common.NoteBlockTask, common.NoteBlockQuote} {
			t.Run(string(prefix)+"/"+string(kind), func(t *testing.T) {
				image := common.NoteImage{ID: "shot.png", Width: 400, Height: 200}
				source := woxImage{ImageType: "absolute", ImageData: notesplugin.ResolveNoteImagePath(image)}
				key := fmt.Sprintf("%s-svg-%d", imageKey(source), previewImageRequestSize(800, woxcomponent.NoteEditorImageMaxHeight))
				app := &App{palette: defaultPalette(), images: map[string]*woxui.Image{key: {Width: 400, Height: 200}}, imageLastUsed: map[string]uint64{}}
				blocks := []common.NoteBlock{{ID: "intro", Type: common.NoteBlockParagraph, Text: "intro"}}
				switch prefix {
				case common.NoteBlockTable:
					blocks = append(blocks, common.NoteBlock{ID: "table", Type: prefix, Table: &common.NoteTable{Rows: [][]common.NoteTableCell{{{Text: "cell"}}}}})
				case common.NoteBlockImage:
					blocks = append(blocks, common.NoteBlock{ID: "image", Type: prefix, Image: &image})
				}
				item := len(blocks)
				blocks = append(blocks, common.NoteBlock{ID: "first", Type: kind, Text: "first"})
				controller := newNotesWindowController(app, common.NoteRecord{ID: "note", Document: common.NoteDocument{Version: 1, Blocks: blocks}})
				t.Cleanup(func() {
					if controller.saveTimer != nil {
						controller.saveTimer.Stop()
					}
				})
				controller.bindActiveText(item, true)
				host := woxwidget.NewHost(controller.buildNotes)
				host.AttachServices(&notesEditorHostServices{})
				render := func() {
					for range 3 {
						host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 800, Height: 640}, PixelSize: woxui.PixelSize{Width: 1200, Height: 960}, Scale: 1.5})
					}
				}
				press := func(key woxui.Key, modifiers woxui.KeyModifiers) {
					if !host.Key(woxui.KeyEvent{Key: key, Modifiers: modifiers, Down: true}) {
						t.Fatalf("key %q was not handled", key)
					}
					render()
				}
				assertCaret := func(block int) {
					for _, span := range controller.blockRanges {
						if span.Block == block {
							if !controller.editorFocus.HasFocus() || controller.editor.State().Selection.Focus != span.TextEnd {
								t.Fatalf("caret=%+v focused=%v, want end of block %d at %d", controller.editor.State().Selection, controller.editorFocus.HasFocus(), block, span.TextEnd)
							}
							return
						}
					}
					t.Fatalf("missing text range for block %d", block)
				}
				render()
				press(woxui.KeyEnter, 0)
				assertCaret(item + 1)
				host.TextInput(woxui.TextInputEvent{Text: "second"})
				render()
				if controller.document.Blocks[item].Text != "first" || controller.document.Blocks[item+1].Text != "second" || controller.document.Blocks[item+1].Type != kind {
					t.Fatalf("typing after Enter changed the wrong item: %+v", controller.document.Blocks)
				}
				if kind != common.NoteBlockQuote {
					press(woxui.KeyTab, 0)
					assertCaret(item + 1)
					if controller.document.Blocks[item+1].Indent != 1 {
						t.Fatal("Tab must indent the new list item")
					}
					press(woxui.KeyTab, woxui.KeyModifierShift)
					assertCaret(item + 1)
					if controller.document.Blocks[item+1].Indent != 0 {
						t.Fatal("Shift+Tab must restore the list item's indentation")
					}
				}
				press(woxui.KeyEnter, 0)
				assertCaret(item + 2)
				press(woxui.KeyEnter, 0)
				assertCaret(item + 2)
				if controller.document.Blocks[item+2].Type != common.NoteBlockParagraph {
					t.Fatal("Enter on an empty list item must exit into a paragraph")
				}
			})
		}
	}
}

// TestNotesListCaretScrollsDocument exercises the outer scroller while a trailing text segment grows.
func TestNotesListCaretScrollsDocument(t *testing.T) {
	for _, kind := range []common.NoteBlockType{common.NoteBlockBullet, common.NoteBlockOrdered, common.NoteBlockTask, common.NoteBlockQuote} {
		t.Run(string(kind), func(t *testing.T) {
			controller := newNotesWindowController(&App{palette: defaultPalette()}, common.NoteRecord{
				ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
					{ID: "table", Type: common.NoteBlockTable, Table: &common.NoteTable{Rows: [][]common.NoteTableCell{{{Text: "cell"}}}}},
					{ID: "first", Type: kind, Text: "first"},
				}},
			})
			t.Cleanup(func() {
				if controller.saveTimer != nil {
					controller.saveTimer.Stop()
				}
			})
			controller.bindActiveText(1, true)
			window := &woxui.Window{}
			services := &notesEditorHostServices{}
			host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
				return woxcomponent.WoxNoteEditor(woxcomponent.NoteEditorProps{
					ID: "notes.editor", Document: controller.document, Width: 400, Height: 240, LineHeight: 24, Zoom: 1,
					Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 24}, Style: controller.editorStyle(),
					Theme: controller.app.palette.componentTheme(), Window: window, Autofocus: true,
					Controller: controller.editor, FocusNode: controller.editorFocus, Focused: controller.editorFocus.HasFocus(),
					FocusedTableBlock: -1, FocusedImageBlock: -1, ActiveSegmentStart: controller.activeTextSegment.Start,
					OnTextFocus: controller.focusNoteText, OnChanged: controller.onSegmentChanged, OnKey: controller.onKey,
				})
			})
			host.AttachServices(services)
			t.Cleanup(host.Dispose)
			render := func() {
				for range 3 {
					host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 400, Height: 240}, PixelSize: woxui.PixelSize{Width: 600, Height: 360}, Scale: 1.5})
				}
			}
			assertVisible := func() {
				t.Helper()
				caret := services.textInput.CursorRect
				if !services.textInput.Enabled || caret.Y < 0 || caret.Y+caret.Height > 240 {
					t.Fatalf("caret must remain inside the document viewport: %+v", services.textInput)
				}
			}
			render()
			for range 20 {
				if !host.Key(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) {
					t.Fatal("Enter must continue the list")
				}
				render()
				assertVisible()
				if !host.TextInput(woxui.TextInputEvent{Text: "next"}) {
					t.Fatal("list item must accept text")
				}
				render()
				assertVisible()
			}
			host.Pointer(woxui.PointerEvent{Kind: woxui.PointerScroll, Position: woxui.Point{X: 200, Y: 100}, Scroll: woxui.Point{Y: 100}})
			render()
			if services.textInput.CursorRect.Y < 240 {
				t.Fatal("manual scrolling must not snap back to the caret")
			}
			host.TextInput(woxui.TextInputEvent{Text: "more"})
			render()
			assertVisible()
		})
	}
}

// TestNotesClickBelowTextBlocks uses pointer hit testing to leave each text block through blank space.
func TestNotesClickBelowTextBlocks(t *testing.T) {
	for _, blockType := range []common.NoteBlockType{
		common.NoteBlockParagraph, common.NoteBlockHeading1, common.NoteBlockHeading2, common.NoteBlockHeading3,
		common.NoteBlockTask, common.NoteBlockBullet, common.NoteBlockOrdered, common.NoteBlockQuote, common.NoteBlockCode, common.NoteBlockDivider,
	} {
		t.Run(string(blockType), func(t *testing.T) {
			block := common.NoteBlock{ID: "body", Type: blockType, Text: "body"}
			if blockType == common.NoteBlockDivider {
				block.Text = ""
			}
			controller := newNotesWindowController(&App{palette: defaultPalette()}, common.NoteRecord{
				ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{block}},
			})
			t.Cleanup(func() {
				if controller.saveTimer != nil {
					controller.saveTimer.Stop()
				}
			})
			// Vertical line hit testing needs a Window handle but no native text measurements.
			window := &woxui.Window{}
			host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
				return woxcomponent.WoxNoteEditor(woxcomponent.NoteEditorProps{
					ID: "notes.editor", Document: controller.document, Width: 400, Height: 240, LineHeight: 24,
					Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 24}, Style: controller.editorStyle(),
					Window: window, Autofocus: true, Controller: controller.editor, FocusNode: controller.editorFocus,
					Focused: controller.editorFocus.HasFocus(), FocusedTableBlock: -1, FocusedImageBlock: -1,
					ActiveSegmentStart: controller.activeTextSegment.Start, OnTextFocus: controller.focusNoteText,
					OnChanged: controller.onSegmentChanged, OnTapOffset: controller.handleBlockTap, CursorAtOffset: controller.editorCursorAt,
					OnTapBelowText: controller.appendParagraphBelowText, OnKey: controller.onKey,
				})
			})
			host.AttachServices(&notesEditorHostServices{})
			render := func() {
				for range 3 {
					host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 400, Height: 240}, PixelSize: woxui.PixelSize{Width: 600, Height: 360}, Scale: 1.5})
				}
			}
			click := func(point woxui.Point) {
				host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
				render()
				host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
				render()
			}
			render()
			click(woxui.Point{X: 350, Y: 20})
			if len(controller.document.Blocks) != 1 {
				t.Fatal("clicking beside the current line must not append a paragraph")
			}
			if blockType == common.NoteBlockTask {
				click(woxui.Point{X: 20, Y: 20})
				if !controller.document.Blocks[0].Checked {
					t.Fatal("checkbox click must still toggle the task")
				}
			}
			click(woxui.Point{X: 20, Y: 100})
			if len(controller.document.Blocks) != 2 || controller.document.Blocks[1].Type != common.NoteBlockParagraph || controller.editor.State().Selection.Focus != controller.blockRanges[1].TextStart {
				t.Fatalf("click below %s did not enter a new paragraph: blocks=%+v selection=%+v", blockType, controller.document.Blocks, controller.editor.State().Selection)
			}
			click(woxui.Point{X: 20, Y: 140})
			if len(controller.document.Blocks) != 2 {
				t.Fatal("repeated blank-space clicks must reuse the empty paragraph")
			}
			host.TextInput(woxui.TextInputEvent{Text: "next"})
			render()
			if controller.document.Blocks[0].Type != blockType || controller.document.Blocks[0].Text != block.Text || controller.document.Blocks[1].Type != common.NoteBlockParagraph || controller.document.Blocks[1].Text != "next" {
				t.Fatalf("typing below %s changed the original block: %+v", blockType, controller.document.Blocks)
			}
			if blockType == common.NoteBlockTask && !controller.document.Blocks[0].Checked {
				t.Fatal("typing below the checkbox must preserve its checked state")
			}
		})
	}
}

// TestNotesTypingBelowTrailingTable exercises the trailing input field after loading, pasting, and deleting a gap.
func TestNotesTypingBelowTrailingTable(t *testing.T) {
	for _, source := range []string{"loaded", "existing-paragraph", "pasted"} {
		t.Run(source, func(t *testing.T) {
			document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
				{ID: "above", Type: common.NoteBlockParagraph, Text: "above"},
				{ID: "table", Type: common.NoteBlockTable, Table: &common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{{{Text: "Header"}}, {{Text: "Cell"}}}}},
			}}
			if source == "existing-paragraph" {
				document.Blocks = append(document.Blocks, common.NoteBlock{ID: "below", Type: common.NoteBlockParagraph})
			}
			if source == "pasted" {
				document.Blocks = document.Blocks[:1]
			}
			controller := newNotesWindowController(&App{palette: defaultPalette()}, common.NoteRecord{ID: "note", Document: document})
			t.Cleanup(func() {
				if controller.saveTimer != nil {
					controller.saveTimer.Stop()
				}
			})
			if source == "pasted" && !controller.pasteDocument("| Header |\n| --- |\n| Cell |") {
				t.Fatal("table paste was not handled")
			}
			host := woxwidget.NewHost(controller.buildNotes)
			host.AttachServices(&notesEditorHostServices{})
			render := func() {
				for range 3 {
					host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 800, Height: 640}, PixelSize: woxui.PixelSize{Width: 1200, Height: 960}, Scale: 1.5})
				}
			}
			click := func(id string) {
				for _, node := range host.Snapshot().Tree.Nodes {
					if node.AutomationID != id {
						continue
					}
					point := woxui.Point{X: node.Bounds.X + 20, Y: node.Bounds.Y + node.Bounds.Height/2}
					host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
					render()
					host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
					render()
					return
				}
				t.Fatalf("missing clickable node %q", id)
			}
			render()
			click("notes.editor.1")
			click("notes.editor.table." + controller.document.Blocks[1].ID + ".1.0")
			if controller.focusedTableBlock != 1 {
				t.Fatal("clicking the table must focus its cell")
			}
			click("notes.editor.1")
			if !host.HasFocus("notes.editor.1") || !controller.editorFocus.HasFocus() || controller.focusedTableBlock >= 0 {
				t.Fatalf("click below table did not enter text: focus=%q table=%d", host.FocusedKey(), controller.focusedTableBlock)
			}
			if !controller.deleteEmptyTextSegment(2) {
				t.Fatal("empty trailing paragraph was not removed")
			}
			render()
			click("notes.editor.1")
			host.TextInput(woxui.TextInputEvent{Text: "after table"})
			render()
			click("notes.editor")
			if controller.editor.Text() != "above" || controller.document.Blocks[2].Text != "after table" || controller.document.Blocks[1].Table.Rows[1][0].Text != "Cell" {
				t.Fatalf("typing below the table changed other content: %+v", controller.document.Blocks)
			}
		})
	}
}

// TestNotesImageSelectionSurvivesFrames exercises focus reconciliation after a real pointer click.
func TestNotesImageSelectionSurvivesFrames(t *testing.T) {
	image := common.NoteImage{ID: "shot.png", Width: 400, Height: 200}
	source := woxImage{ImageType: "absolute", ImageData: notesplugin.ResolveNoteImagePath(image)}
	key := fmt.Sprintf("%s-svg-%d", imageKey(source), previewImageRequestSize(800, woxcomponent.NoteEditorImageMaxHeight))
	app := &App{palette: defaultPalette(), images: map[string]*woxui.Image{key: {Width: 400, Height: 200}}, imageLastUsed: map[string]uint64{}}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
			{ID: "above", Type: common.NoteBlockParagraph, Text: "above"},
			{ID: "img", Type: common.NoteBlockImage, Image: &image},
		}},
	})
	t.Cleanup(func() {
		if controller.saveTimer != nil {
			controller.saveTimer.Stop()
		}
	})
	host := woxwidget.NewHost(controller.buildNotes)
	host.AttachServices(&notesEditorHostServices{})
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 800, Height: 640}, PixelSize: woxui.PixelSize{Width: 1200, Height: 960}, Scale: 1.5}
	render := func() {
		for range 3 {
			host.Frame(&woxui.DisplayList{}, frame)
		}
	}
	bounds := func(id string) woxui.Rect {
		for _, node := range host.Snapshot().Tree.Nodes {
			if node.AutomationID == id {
				return node.Bounds
			}
		}
		t.Fatalf("missing node %q", id)
		return woxui.Rect{}
	}
	click := func(position woxui.Point) {
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: position})
		render()
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: position})
		render()
	}
	render()
	if !host.HasFocus("notes.editor") || !controller.editorFocus.HasFocus() {
		t.Fatal("text above the image must initially accept typing")
	}
	below := bounds("notes.editor.1")
	click(woxui.Point{X: below.X + 20, Y: below.Y + below.Height/2})
	if !host.HasFocus("notes.editor.1") || controller.activeTextSegment.Start <= notesTestImageIndex(controller.document) {
		t.Fatalf("click below image kept focus at %q, segment=%+v", host.FocusedKey(), controller.activeTextSegment)
	}
	picture := bounds("notes.editor.image.img")
	click(woxui.Point{X: picture.X + picture.Width/2, Y: picture.Y + picture.Height/2})
	if controller.focusedImageBlock != notesTestImageIndex(controller.document) || controller.editorFocus.HasFocus() {
		t.Fatalf("image selection lost after frames: image=%d textFocused=%v", controller.focusedImageBlock, controller.editorFocus.HasFocus())
	}
	bounds("notes.editor.image.img.image-delete")
	for _, key := range []woxui.Key{woxui.KeyArrowUp, woxui.KeyArrowDown} {
		if !controller.onKey(woxui.KeyEvent{Key: key, Down: true}) {
			t.Fatalf("image did not handle %q", key)
		}
		render()
		wantFocus := "notes.editor"
		if key == woxui.KeyArrowDown {
			wantFocus = "notes.editor.1"
		}
		if !host.HasFocus(woxwidget.Key(wantFocus)) || !controller.editorFocus.HasFocus() || controller.focusedImageBlock >= 0 {
			t.Fatalf("image exit with %q: focus=%q, want %q", key, host.FocusedKey(), wantFocus)
		}
		click(woxui.Point{X: picture.X + picture.Width/2, Y: picture.Y + picture.Height/2})
	}

	text := bounds("notes.editor.1")
	click(woxui.Point{X: text.X + 20, Y: text.Y + 12})
	if controller.focusedImageBlock >= 0 || !controller.editorFocus.HasFocus() {
		t.Fatal("clicking text must leave the image and restore editor focus")
	}
	host.TextInput(woxui.TextInputEvent{Text: "caption"})
	render()
	if controller.editor.Text() != "caption" {
		t.Fatalf("text after leaving image = %q, want caption", controller.editor.Text())
	}
	above := bounds("notes.editor")
	click(woxui.Point{X: above.X + 20, Y: above.Y + 12})
	if !host.HasFocus("notes.editor") || controller.editor.Text() != "above" {
		t.Fatalf("return to text above image: focus=%q text=%q", host.FocusedKey(), controller.editor.Text())
	}
	if controller.document.Blocks[2].Text != "caption" {
		t.Fatalf("text below image was not preserved: %+v", controller.document.Blocks)
	}
}

func TestNotesClickingTextLeavesAFocusedImage(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
			{ID: "img", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png", FileName: "shot.png"}},
		}},
	})
	imageIndex := notesTestImageIndex(controller.document)
	controller.focusedImageBlock = imageIndex
	controller.focusNoteText(imageIndex + 1)
	if controller.focusedImageBlock >= 0 {
		t.Fatal("clicking the paragraph below an image should clear the image highlight")
	}
	if controller.activeTextSegment.Start <= imageIndex {
		t.Fatalf("active text after click = %#v, want the paragraph below the image", controller.activeTextSegment)
	}
}

func notesTestImageBlock(document common.NoteDocument) *common.NoteImage {
	for _, block := range document.Blocks {
		if block.Type == common.NoteBlockImage && block.Image != nil {
			return block.Image
		}
	}
	return nil
}

func notesTestImageIndex(document common.NoteDocument) int {
	for index, block := range document.Blocks {
		if block.Type == common.NoteBlockImage {
			return index
		}
	}
	return -1
}

func collectNotesAutomationIDs(node woxwidget.Widget) map[string]bool {
	ids := map[string]bool{}
	walkNotesWidgets(node, func(widget woxwidget.Widget) {
		if semantics, ok := widget.(woxwidget.Semantics); ok && semantics.AutomationID != "" {
			ids[semantics.AutomationID] = true
		}
	})
	return ids
}

func notesSearchScrollProps(node woxwidget.Widget) (woxcomponent.ScrollViewProps, bool) {
	var found woxcomponent.ScrollViewProps
	ok := false
	walkNotesWidgets(node, func(widget woxwidget.Widget) {
		stateful, isStateful := widget.(woxwidget.Stateful)
		if !isStateful {
			return
		}
		props, isScroll := stateful.Widget.(woxcomponent.ScrollViewProps)
		if isScroll && props.Key == "notes.search.results" {
			found = props
			ok = true
		}
	})
	return found, ok
}

func collectNotesSelectedIDs(node woxwidget.Widget) map[string]bool {
	ids := map[string]bool{}
	walkNotesWidgets(node, func(widget woxwidget.Widget) {
		if semantics, ok := widget.(woxwidget.Semantics); ok && semantics.Selected && semantics.AutomationID != "" {
			ids[semantics.AutomationID] = true
		}
	})
	return ids
}

func walkNotesWidgets(widget woxwidget.Widget, visit func(woxwidget.Widget)) {
	if widget == nil {
		return
	}
	visit(widget)
	switch typed := widget.(type) {
	case woxwidget.Semantics:
		walkNotesWidgets(typed.Child, visit)
	case woxwidget.Stateful:
		if child, ok := typed.Widget.(woxwidget.Widget); ok {
			walkNotesWidgets(child, visit)
		} else if props, ok := typed.Widget.(woxcomponent.ScrollViewProps); ok {
			walkNotesWidgets(props.Content, visit)
		}
	case woxwidget.Container:
		walkNotesWidgets(typed.Child, visit)
	case woxwidget.Align:
		walkNotesWidgets(typed.Child, visit)
	case woxwidget.Gesture:
		walkNotesWidgets(typed.Child, visit)
	case woxwidget.Expanded:
		walkNotesWidgets(typed.Child, visit)
	case woxwidget.ScrollView:
		walkNotesWidgets(typed.Child, visit)
	case woxwidget.Flex:
		for _, child := range typed.Children {
			walkNotesWidgets(child, visit)
		}
	case woxwidget.Stack:
		for _, child := range typed.Children {
			walkNotesWidgets(child.Child, visit)
		}
	}
}

func TestNotesFormatBarUsesSVGIconsAndHoverTooltips(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	theme := woxcomponent.Theme{ToolbarText: woxui.Color{R: 240, G: 240, B: 240, A: 255}}
	items := controller.buildFormatBar(420, theme).(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex).Children
	if len(items) != 13 {
		t.Fatalf("format items = %d, want full bar", len(items))
	}
	first := items[0].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if first.ID != "notes.format.block" || first.OnHoverAt == nil {
		t.Fatalf("heading control = id %q hover %v", first.ID, first.OnHoverAt != nil)
	}
	icon, ok := first.Icon.(woxwidget.Image)
	if !ok || icon.Source == nil || icon.Width != 16 || icon.Height != 16 {
		t.Fatalf("heading icon = %#v, want 16x16 SVG image", first.Icon)
	}
	for index, child := range items {
		button := child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		image, imageOK := button.Icon.(woxwidget.Image)
		if !imageOK || image.Source == nil || button.OnHoverAt == nil {
			t.Fatalf("format item %d = icon %#v hover %v, want SVG with tooltip", index, button.Icon, button.OnHoverAt != nil)
		}
	}
}

func TestNotesToolbarTooltipUsesLinuxInlineFallback(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only tooltip fallback")
	}
	app := &App{palette: defaultPalette(), lifecycleCtx: t.Context()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	anchor := woxui.Rect{X: 300, Y: 4, Width: 32, Height: 32}
	controller.updateToolbarTooltip(true, "  Search (Ctrl+P)  ", anchor)
	if controller.inlineTooltip != nil {
		t.Fatal("inline tooltip must wait for the shared hover dwell")
	}
	deadline := time.Now().Add(nativeHoverTooltipDelay + 300*time.Millisecond)
	for controller.inlineTooltip == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if controller.inlineTooltip == nil {
		t.Fatal("expected inline tooltip state on linux after the hover dwell")
	}
	if controller.inlineTooltip.Text != "Search (Ctrl+P)" {
		t.Fatalf("tooltip text = %q, want trimmed content", controller.inlineTooltip.Text)
	}
	if controller.inlineTooltip.Anchor != anchor {
		t.Fatalf("tooltip anchor = %#v, want %#v", controller.inlineTooltip.Anchor, anchor)
	}
	if controller.inlineTooltip.Side != "top" {
		t.Fatalf("tooltip side = %q, want top", controller.inlineTooltip.Side)
	}

	controller.updateToolbarTooltip(false, "", woxui.Rect{})
	if controller.inlineTooltip != nil {
		t.Fatalf("tooltip state = %#v, want nil after hide", controller.inlineTooltip)
	}
}

func TestNotesLinuxInlineTooltipPaintsInsideWindow(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	controller.inlineTooltip = &settingsInlineTooltipState{
		Text: "Search", Side: "top", Anchor: woxui.Rect{X: 300, Y: 4, Width: 32, Height: 32},
	}
	theme := app.palette.componentTheme()
	overlay := controller.composeLinuxInlineTooltip(woxui.Size{Width: 460, Height: 320}, theme, nil)
	if !notesOverlayContainsText(overlay, "Search") {
		t.Fatal("expected in-window tooltip overlay")
	}
	root := controller.buildNotes(woxui.FrameInfo{Size: woxui.Size{Width: 460, Height: 320}})
	if !notesOverlayContainsText(root, "Search") {
		t.Fatal("expected tooltip text in the notes window tree")
	}
}

func TestNotesLinuxInlineTooltipStacksAboveSearchOverlay(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	controller.inlineTooltip = &settingsInlineTooltipState{
		Text: "Search", Side: "top", Anchor: woxui.Rect{X: 300, Y: 4, Width: 32, Height: 32},
	}
	search := woxwidget.Container{Width: 460, Height: 320}
	overlay := controller.composeLinuxInlineTooltip(woxui.Size{Width: 460, Height: 320}, app.palette.componentTheme(), search)
	stack, ok := overlay.(woxwidget.Stack)
	if !ok || len(stack.Children) != 2 {
		t.Fatalf("overlay = %#v, want search under the tooltip", overlay)
	}
	if _, isSearch := stack.Children[0].Child.(woxwidget.Container); !isSearch {
		t.Fatalf("bottom overlay = %#v, want the existing search layer", stack.Children[0].Child)
	}
	if !notesOverlayContainsText(stack.Children[1].Child, "Search") {
		t.Fatal("expected tooltip stacked above the search overlay")
	}
}

func notesOverlayContainsText(widget woxwidget.Widget, want string) bool {
	found := false
	walkNotesWidgets(widget, func(child woxwidget.Widget) {
		if text, ok := child.(woxwidget.TextBlock); ok && text.Value == want {
			found = true
		}
	})
	return found
}

func TestNotesFormatBarInTableDoesNotHighlightOutsideBullet(t *testing.T) {
	app := &App{palette: defaultPalette()}
	table := common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{{{Text: "A"}}, {{Text: "1"}}}}
	document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
		{ID: "t", Type: common.NoteBlockTable, Table: &table},
		{ID: "b", Type: common.NoteBlockBullet, Text: "官方提供少而精的真实示例。"},
	}}
	controller := newNotesWindowController(app, common.NoteRecord{ID: "note", Document: document})
	_, _, ranges := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{})
	controller.document, controller.blockRanges = document, ranges
	controller.selection = woxui.TextSelection{Anchor: ranges[0].TextStart + 1, Focus: ranges[0].TextStart + 1}
	controller.focusedTableBlock, controller.focusedTableRow, controller.focusedTableCol = 0, 1, 0
	items := controller.buildFormatBar(420, woxcomponent.Theme{ToolbarText: woxui.Color{A: 255}}).(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex).Children
	var bullet, tableButton woxcomponent.IconButtonProps
	for _, child := range items {
		button := child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		switch button.ID {
		case "notes.format.bullet":
			bullet = button
		case "notes.format.table":
			tableButton = button
		}
	}
	if bullet.Selected {
		t.Fatal("list control must stay inactive while the caret is in a table cell")
	}
	if !tableButton.Selected {
		t.Fatal("table control should stay active while the caret is in a table cell")
	}
}

func TestNotesFormatBarHighlightsActiveUnderline(t *testing.T) {
	app := &App{palette: defaultPalette()}
	document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
		{ID: "block", Type: common.NoteBlockParagraph, Text: "hello dsafdsfa", Spans: []common.NoteSpan{{Start: 6, End: 14, Underline: true}}},
	}}
	controller := newNotesWindowController(app, common.NoteRecord{ID: "note", Document: document})
	_, _, ranges := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{})
	controller.document, controller.blockRanges = document, ranges
	controller.selection = woxui.TextSelection{Anchor: ranges[0].TextStart + 8, Focus: ranges[0].TextStart + 8}
	theme := woxcomponent.Theme{ToolbarText: woxui.Color{R: 40, G: 40, B: 40, A: 255}, Cursor: woxui.Color{R: 19, G: 121, B: 210, A: 255}}
	items := controller.buildFormatBar(420, theme).(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex).Children
	var underline, bold woxcomponent.IconButtonProps
	for _, child := range items {
		button := child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		switch button.ID {
		case "notes.format.underline":
			underline = button
		case "notes.format.bold":
			bold = button
		}
	}
	if !underline.Selected || underline.SelectedBackground.A == 0 {
		t.Fatalf("underline button = %#v, want selected highlight", underline)
	}
	if bold.Selected {
		t.Fatal("plain bold control must stay inactive in underlined text")
	}
}

func TestNotesToolbarSupportsCaptionButtonsAndTitleBarDoubleClick(t *testing.T) {
	app := &App{palette: defaultPalette(), noteWindows: map[string]*notesWindowController{}}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	toolbar := controller.buildToolbar(420, true, woxcomponent.Theme{}).(woxwidget.Container).Child.(woxwidget.Stack)
	var drag woxwidget.Gesture
	var chrome woxcomponent.WindowCloseChromeProps
	for _, child := range toolbar.Children {
		if semantics, ok := child.Child.(woxwidget.Semantics); ok && semantics.AutomationID == "notes.toolbar.drag" {
			drag = semantics.Child.(woxwidget.Gesture)
		}
		if stateful, ok := child.Child.(woxwidget.Stateful); ok {
			if props, ok := stateful.Widget.(woxcomponent.WindowCloseChromeProps); ok {
				chrome = props
			}
		}
	}
	if drag.OnDoubleTap == nil {
		t.Fatal("Notes title-bar drag must toggle maximize on double-click")
	}
	if chrome.OnMinimize == nil || chrome.OnMaximize == nil || chrome.OnClose == nil {
		t.Fatalf("Notes caption chrome = %#v, want minimize, maximize, and close", chrome)
	}
}

func TestNotesBuildKeepsThemeBackground(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	root := controller.buildNotes(woxui.FrameInfo{Size: woxui.Size{Width: 460, Height: 320}}).(woxwidget.Semantics)
	body := root.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Container)
	want := defaultPalette().componentTheme().Background
	if body.Color != want || body.Color.A == 0 {
		t.Fatalf("notes window fill = %#v, want theme.Background %#v", body.Color, want)
	}
}

func TestNotesFormatBarDoesNotPaintToolbarBackground(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	bar := controller.buildFormatBar(420, woxcomponent.Theme{ToolbarBackground: woxui.Color{R: 245, G: 245, B: 245, A: 255}}).(woxwidget.Container)
	if bar.Color != (woxui.Color{}) {
		t.Fatalf("format bar fill = %#v, want the window theme wash instead of ToolbarBackground", bar.Color)
	}
}

func TestNotesMaximizeBoundsUsesLogicalWorkArea(t *testing.T) {
	displays := []screen.Display{
		{ID: "left", WorkArea: screen.Rect{X: -1280, Y: 0, Width: 1280, Height: 720}, Scale: 1.25},
		{ID: "main", WorkArea: screen.Rect{X: 0, Y: 0, Width: 1920, Height: 1040}, Scale: 2, Primary: true},
	}
	got := notesMaximizeBoundsToDisplays(woxui.Rect{X: -400, Y: 80, Width: 420, Height: 320}, displays)
	if got.X != -1280 || got.Y != 0 || got.Width != 1280 || got.Height != 720 {
		t.Fatalf("maximize bounds on negative-origin display = %#v", got)
	}
	got = notesMaximizeBoundsToDisplays(woxui.Rect{X: 200, Y: 80, Width: 420, Height: 320}, displays)
	if got.X != 0 || got.Y != 0 || got.Width != 1920 || got.Height != 1040 {
		t.Fatalf("maximize bounds on primary display = %#v", got)
	}
}

func TestNotesWindowMaximizePersistsPerNote(t *testing.T) {
	services := &notesWindowTestServices{local: map[string]string{}}
	app := &App{services: services, palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{ID: "note"})
	controller.windowMaximized = true
	controller.restoreFrame = woxui.Rect{X: 40, Y: 60, Width: 420, Height: 320}
	controller.persistMaximizePreference()
	if services.local["windowMaximized:note"] != "1" {
		t.Fatalf("window maximize preference = %#v", services.local)
	}
	reopened := newNotesWindowController(app, common.NoteRecord{ID: "note"})
	if !reopened.readWindowMaximized() {
		t.Fatal("reopened note lost maximized preference")
	}
}

func TestNotesWindowPinPersistsPerNote(t *testing.T) {
	services := &notesWindowTestServices{local: map[string]string{}}
	app := &App{services: services, palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{ID: "note"})
	if controller.windowPinned {
		t.Fatal("new notes must start unpinned")
	}
	controller.toggleWindowPin()
	if !controller.windowPinned {
		t.Fatal("toggle did not pin the window")
	}
	if services.local["windowPinned:note"] != "1" {
		t.Fatalf("window pin preference = %#v", services.local)
	}
	reopened := newNotesWindowController(app, common.NoteRecord{ID: "note"})
	if !reopened.windowPinned {
		t.Fatal("reopened note lost window pin")
	}
	controller.toggleWindowPin()
	if controller.windowPinned {
		t.Fatal("toggle did not unpin the window")
	}
	if services.local["windowPinned:note"] != "0" {
		t.Fatalf("window unpin preference = %#v", services.local)
	}
}

func TestNotesWindowUsesApplicationRoleForTaskbar(t *testing.T) {
	if notesWindowRole != woxui.WindowRoleApplication {
		t.Fatalf("notes window role = %v, want application so minimized notes stay on the taskbar", notesWindowRole)
	}
}

func TestNotesWindowUsesNotesPluginIconOnTaskbar(t *testing.T) {
	if notesWindowIcon() == nil {
		t.Fatal("notes window icon failed to decode")
	}
	if notesWindowIcon() != notesTitleBarIcon {
		t.Fatal("taskbar icon must use the notes plugin glyph, not the Wox app icon")
	}
}

func TestNotesUseIndependentNativeWindowIDs(t *testing.T) {
	app := &App{palette: defaultPalette()}
	first := newNotesWindowController(app, common.NoteRecord{ID: "first"})
	second := newNotesWindowController(app, common.NoteRecord{ID: "second"})
	if first.windowID == second.windowID {
		t.Fatalf("two notes shared native window ID %q", first.windowID)
	}
}

func (s *notesWindowTestServices) NotesList(context.Context, string, bool) ([]common.NoteSummary, error) {
	return nil, nil
}

func (s *notesWindowTestServices) NotesGet(_ context.Context, id string) (common.NoteRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if record, ok := s.records[id]; ok {
		return record, nil
	}
	if s.record.ID == id {
		return s.record, nil
	}
	return common.NoteRecord{}, errors.New("note not found")
}

func (s *notesWindowTestServices) NotesRestore(_ context.Context, id string) (common.NoteRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[id]
	if !ok {
		return common.NoteRecord{}, errors.New("note not found")
	}
	record.DeletedAt = 0
	if s.records == nil {
		s.records = map[string]common.NoteRecord{}
	}
	s.records[id] = record
	return record, nil
}

func (s *notesWindowTestServices) NotesSetLocal(_ context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.local == nil {
		s.local = map[string]string{}
	}
	s.local[key] = value
	return nil
}

func (s *notesWindowTestServices) NotesGetLocal(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.local == nil {
		return "", nil
	}
	return s.local[key], nil
}

func (s *notesWindowTestServices) NotesDiscard(_ context.Context, id string) error {
	s.mu.Lock()
	s.discardIDs = append(s.discardIDs, id)
	s.mu.Unlock()
	select {
	case s.discardCall <- struct{}{}:
	default:
	}
	return nil
}

func TestNotesWindowAutosavesAndRetainsDirtyStateOnFailure(t *testing.T) {
	services := &notesWindowTestServices{saveCall: make(chan struct{}, 1)}
	app := &App{services: services, palette: defaultPalette(), lifecycleCtx: context.Background(), uiCall: func(callback func()) error { callback(); return nil }, noteWindows: map[string]*notesWindowController{}}
	record := common.NoteRecord{ID: "note", Revision: "initial", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}}}
	controller := newNotesWindowController(app, record)
	app.noteWindows[controller.record.ID] = controller
	controller.onEditorChanged("hello")
	select {
	case <-services.saveCall:
	case <-time.After(2 * time.Second):
		t.Fatal("500ms autosave did not run")
	}
	if controller.dirty {
		t.Fatal("successful autosave left note dirty")
	}

	services.mu.Lock()
	services.saveErr = errors.New("disk full")
	services.mu.Unlock()
	controller.onEditorChanged("hello again")
	if err := controller.flush(); err == nil || !controller.dirty {
		t.Fatalf("failed save must remain retryable: dirty=%t err=%v", controller.dirty, err)
	}
	services.mu.Lock()
	services.saveErr = nil
	services.mu.Unlock()
	if err := controller.flush(); err != nil || controller.dirty {
		t.Fatalf("retry did not persist note: dirty=%t err=%v", controller.dirty, err)
	}
}

func TestNotesFlushSkipsEmptyDraft(t *testing.T) {
	services := &notesWindowTestServices{saveCall: make(chan struct{}, 1)}
	app := &App{services: services, palette: defaultPalette(), lifecycleCtx: context.Background(), noteWindows: map[string]*notesWindowController{}}
	record := common.NoteRecord{ID: "draft", Revision: "initial", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}}}
	controller := newNotesWindowController(app, record)
	controller.dirty = true
	if err := controller.flush(); err != nil {
		t.Fatalf("flush empty draft: %v", err)
	}
	select {
	case <-services.saveCall:
		t.Fatal("empty draft must not be saved")
	default:
	}
}

func TestNotesDiscardEmptyDraftOnClose(t *testing.T) {
	services := &notesWindowTestServices{discardCall: make(chan struct{}, 1)}
	app := &App{services: services, palette: defaultPalette(), lifecycleCtx: context.Background(), noteWindows: map[string]*notesWindowController{}}
	record := common.NoteRecord{ID: "draft", Revision: "initial", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}}}
	controller := newNotesWindowController(app, record)
	app.noteWindows[record.ID] = controller
	controller.discardEmptyNote()
	select {
	case <-services.discardCall:
	case <-time.After(time.Second):
		t.Fatal("empty draft was not discarded")
	}
	if len(services.discardIDs) != 1 || services.discardIDs[0] != "draft" {
		t.Fatalf("discard ids = %#v, want [draft]", services.discardIDs)
	}
	if controller.dirty {
		t.Fatal("discarded empty draft stayed dirty")
	}
}

func TestNotesEditorRepeatedBackspaceKeepsCaret(t *testing.T) {
	app := &App{palette: defaultPalette(), noteWindows: map[string]*notesWindowController{}}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
			{ID: "block", Type: common.NoteBlockParagraph, Text: "hello world"},
		}},
	})
	controller.editor.SetCaret(5)
	if handled, changed := controller.editor.HandleKey(woxui.KeyEvent{Key: woxui.KeyBackspace, Down: true}); !handled || !changed {
		t.Fatal("first Backspace must delete a character")
	}
	controller.onSegmentChanged(0, controller.editor.Text())
	if controller.editor.Text() != "hell world" || controller.editor.State().Selection.Focus != 4 {
		t.Fatalf("after first delete text=%q caret=%d, want %q at 4", controller.editor.Text(), controller.editor.State().Selection.Focus, "hell world")
	}

	if handled, changed := controller.editor.HandleKey(woxui.KeyEvent{Key: woxui.KeyBackspace, Down: true}); !handled || !changed {
		t.Fatal("second Backspace must keep deleting at the same caret")
	}
	controller.onSegmentChanged(0, controller.editor.Text())
	if controller.editor.Text() != "hel world" || controller.editor.State().Selection.Focus != 3 {
		t.Fatalf("after second delete text=%q caret=%d, want %q at 3", controller.editor.Text(), controller.editor.State().Selection.Focus, "hel world")
	}
}

func TestNotesEditorHostBackspaceKeepsFocus(t *testing.T) {
	app := &App{palette: defaultPalette(), noteWindows: map[string]*notesWindowController{}}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
			{ID: "block", Type: common.NoteBlockParagraph, Text: "hello"},
		}},
	})
	controller.editor.SetCaret(3)
	theme := app.palette.componentTheme()
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return woxcomponent.WoxNoteEditor(woxcomponent.NoteEditorProps{
			ID: "notes.editor", Document: controller.document, Width: 400, Height: 240,
			Style: controller.editorStyle(), LineHeight: 24, Zoom: 1, TextColor: theme.PreviewText, Theme: theme,
			Autofocus: true, Controller: controller.editor, FocusNode: controller.editorFocus,
			Focused: controller.editorFocus.HasFocus() && controller.focusedTableBlock < 0, Selection: controller.selection,
			OnChanged: controller.onSegmentChanged, OnSelectionChanged: func(selection woxui.TextSelection) {
				if selection == controller.selection {
					return
				}
				controller.selection = selection
				controller.focusedTableBlock = -1
			},
		})
	})
	host.AttachServices(&notesEditorHostServices{})
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 400, Height: 240}, PixelSize: woxui.PixelSize{Width: 400, Height: 240}, Scale: 1}
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, frame)
	if !host.RequestFocus("notes.editor") {
		t.Fatal("notes editor must accept focus")
	}
	host.Frame(displayList, frame)

	if !host.Key(woxui.KeyEvent{Key: woxui.KeyBackspace, Down: true}) {
		t.Fatal("first Backspace must be handled")
	}
	host.Frame(displayList, frame)
	if !host.HasFocus("notes.editor") || !controller.editorFocus.HasFocus() {
		t.Fatalf("focus after first delete: host=%v node=%v", host.HasFocus("notes.editor"), controller.editorFocus.HasFocus())
	}
	if controller.editor.Text() != "helo" || controller.editor.State().Selection.Focus != 2 {
		t.Fatalf("after first delete text=%q caret=%d, want helo at 2", controller.editor.Text(), controller.editor.State().Selection.Focus)
	}

	if !host.Key(woxui.KeyEvent{Key: woxui.KeyBackspace, Down: true}) {
		t.Fatal("second Backspace must be handled while the caret stays in the editor")
	}
	host.Frame(displayList, frame)
	if !host.HasFocus("notes.editor") || controller.editor.Text() != "hlo" || controller.editor.State().Selection.Focus != 1 {
		t.Fatalf("after second delete text=%q caret=%d focus=%v, want hlo at 1 with editor focus", controller.editor.Text(), controller.editor.State().Selection.Focus, host.HasFocus("notes.editor"))
	}
}

type notesEditorHostServices struct {
	textInput woxui.TextInputState
}

func (s *notesEditorHostServices) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{Size: woxui.Size{Width: float32(len([]rune(text))) * style.Size / 2, Height: style.Size}}, nil
}

func (s *notesEditorHostServices) Invalidate() error               { return nil }
func (s *notesEditorHostServices) InvalidateRect(woxui.Rect) error { return nil }

func (s *notesEditorHostServices) SetTextInputState(state woxui.TextInputState) error {
	s.textInput = state
	return nil
}

func (s *notesEditorHostServices) SetPointerCursor(woxui.PointerCursor) error { return nil }

func (s *notesEditorHostServices) UpdateAccessibility(woxui.AccessibilityTree, woxui.AccessibilityActionHandler) error {
	return nil
}

func TestNotesSearchOverlayDoesNotOverflow(t *testing.T) {
	app := &App{palette: defaultPalette()}
	controller := newNotesWindowController(app, common.NoteRecord{
		ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "block", Type: common.NoteBlockParagraph}}},
	})
	controller.summaries = []common.NoteSummary{{ID: "note", Title: "Wox Notes Smoke", UpdatedAt: 1, DeletedAt: 1}}
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return controller.buildSearchOverlay(woxui.Size{Width: notesDefaultWidth, Height: notesDefaultHeight}, defaultPalette().componentTheme())
	})
	host.AttachServices(&notesEditorHostServices{})
	if err := host.SetRepaintDebugMode(woxwidget.RepaintDebugVerify); err != nil {
		t.Fatal(err)
	}
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{
		Size:      woxui.Size{Width: notesDefaultWidth, Height: notesDefaultHeight},
		PixelSize: woxui.PixelSize{Width: int(notesDefaultWidth), Height: int(notesDefaultHeight)},
		Scale:     1,
	})
	if diagnostics := host.Snapshot().Diagnostics; len(diagnostics) != 0 {
		t.Fatalf("search overlay diagnostics = %v, want none", diagnostics)
	}
}
