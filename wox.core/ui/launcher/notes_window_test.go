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
	"wox/util/screen"
)

type notesWindowTestServices struct {
	contract.Services
	mu       sync.Mutex
	record   common.NoteRecord
	saveErr  error
	saveID   string
	conflict bool
	saveCall chan struct{}
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
	row := controller.notesListRow(common.NoteSummary{ID: "note-a", Title: "Hello", Preview: "preview"}, 300, theme)
	semantics := row.(woxwidget.Semantics)
	if semantics.AutomationID != "notes.search.note-a" {
		t.Fatalf("search automation id = %q, want notes.search.note-a", semantics.AutomationID)
	}
	if _, ok := semantics.Child.(woxwidget.Stateful); !ok {
		t.Fatalf("search row child = %T, want hoverable Stateful for highlight", semantics.Child)
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
	if controls["notes.toolbar.back"] || controls["notes.toolbar.forward"] || !controls["notes.toolbar.new"] {
		t.Fatalf("Notes title-bar controls = %#v, want new without history navigation", controls)
	}
	for key, hotkey := range map[woxwidget.Key]string{"notes.toolbar.search": primaryHotkey("p"), "notes.toolbar.new": primaryHotkey("n")} {
		shortcut := "(" + strings.Join(formatHotkeyLabels(hotkey), "+") + ")"
		if !strings.HasSuffix(labels[key], shortcut) {
			t.Fatalf("Notes title-bar label %q = %q, want shortcut suffix %q", key, labels[key], shortcut)
		}
	}
	title := toolbar.Children[2]
	alignment := title.Child.(woxwidget.Align)
	if runtime.GOOS == "windows" {
		if title.Left != 40 || title.Right != 188 || alignment.Horizontal != 0 {
			t.Fatalf("Windows Notes title slot = %.0f/%.0f alignment %.1f, want left-aligned 40/188/0", title.Left, title.Right, alignment.Horizontal)
		}
		icon := toolbar.Children[4]
		iconAlignment, ok := icon.Child.(woxwidget.Align)
		iconImage, imageOK := iconAlignment.Child.(woxwidget.Image)
		if !ok || !imageOK || icon.Left != 12 || iconAlignment.Width != 20 || iconImage.Source == nil || iconImage.Width != 20 || iconImage.Height != 20 {
			t.Fatalf("Windows Notes title-bar icon slot = %#v, want 20x20 icon at left 12", icon)
		}
	} else if title.Left != 188 || title.Right != 188 || alignment.Horizontal != .5 {
		t.Fatalf("Notes title slot = %.0f/%.0f alignment %.1f, want centered 188/188/0.5", title.Left, title.Right, alignment.Horizontal)
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

func (s *notesWindowTestServices) NotesSetLocal(context.Context, string, string) error { return nil }

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
