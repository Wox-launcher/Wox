package launcher

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"wox/common"
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
	got := clampNotesBoundsToDisplays(woxui.Rect{X: -1400, Y: 650, Width: 420, Height: 700}, displays)
	if got.X != -1280 || got.Y != 80 || got.Width != 420 || got.Height != 640 {
		t.Fatalf("logical bounds were not clamped on negative-origin display: %#v", got)
	}
	got = clampNotesBoundsToDisplays(woxui.Rect{X: 2500, Y: -100, Width: 420, Height: 320}, displays)
	if got.X != 1500 || got.Y != 0 {
		t.Fatalf("disconnected display bounds were not recovered: %#v", got)
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
	if runtime.GOOS == "windows" {
		if title.Left != 40 || title.Right != 220 || alignment.Horizontal != 0 {
			t.Fatalf("Windows Notes title slot = %.0f/%.0f alignment %.1f, want left-aligned 40/220/0", title.Left, title.Right, alignment.Horizontal)
		}
		icon := toolbar.Children[4]
		iconAlignment, ok := icon.Child.(woxwidget.Align)
		iconImage, imageOK := iconAlignment.Child.(woxwidget.Image)
		if !ok || !imageOK || icon.Left != 12 || iconAlignment.Width != 20 || iconImage.Source == nil || iconImage.Width != 20 || iconImage.Height != 20 {
			t.Fatalf("Windows Notes title-bar icon slot = %#v, want 20x20 icon at left 12", icon)
		}
	} else if title.Left != 220 || title.Right != 220 || alignment.Horizontal != .5 {
		t.Fatalf("Notes title slot = %.0f/%.0f alignment %.1f, want centered 220/220/0.5", title.Left, title.Right, alignment.Horizontal)
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
	if len(items) != 12 {
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
