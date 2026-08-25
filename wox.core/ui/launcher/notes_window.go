package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"wox/common"
	notesplugin "wox/plugin/system/notes"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/overlay"
	"wox/util/screen"
)

// notesWindowRole keeps minimized notes on the taskbar and dock. Utility windows
// are hidden from the shell (WS_EX_TOOLWINDOW / skip-taskbar), so they cannot be restored.
const notesWindowRole = woxui.WindowRoleApplication

const (
	notesDefaultWidth            = float32(460)
	notesDefaultHeight           = float32(320)
	notesMinimumWidth            = float32(460)
	notesMinimumHeight           = float32(240)
	notesMaximumHeight           = float32(640)
	notesAutosaveDelay           = 500 * time.Millisecond
	notesSearchRowHeight         = float32(36)
	notesSearchSectionHeight     = float32(28)
	notesSearchSectionLead       = float32(12)
	notesSearchSectionTextHeight = float32(16)
	notesSearchTextHeight        = float32(18)
	notesSearchListGap           = float32(4)
	notesSearchFieldHeight       = float32(36)
	notesSearchOverlayPadding    = float32(12)
	notesSearchOverlayGap        = float32(8)
)

var notesTitleBarIcon, _ = decodeWoxImageWithTint(fromCoreImage(common.PluginNotesIcon), nil, 256)

// notesWindowIcon is the notes plugin glyph used both in the title bar and on the taskbar.
func notesWindowIcon() *woxui.Image {
	return notesTitleBarIcon
}

// notesNativeMinSize is the live resize floor passed to the native window.
func notesNativeMinSize() woxui.Size {
	return woxui.Size{Width: notesMinimumWidth, Height: notesMinimumHeight}
}

type notesWindowBounds struct {
	X, Y, Width, Height float32
}

type notesWindowController struct {
	app               *App
	windowID          woxui.WindowID
	managed           *woxui.ManagedWindow
	host              *woxwidget.Host
	editor            *woxwidget.TextEditingController
	editorFocus       *woxwidget.FocusNode
	searchEditor      *woxwidget.TextEditingController
	searchFocus       *woxwidget.FocusNode
	linkEditor        *woxwidget.TextEditingController
	linkFocus         *woxwidget.FocusNode
	record            common.NoteRecord
	document          common.NoteDocument
	richRuns          []woxcomponent.NoteTextRun
	blockRanges       []noteBlockRange
	selection         woxui.TextSelection
	focusedTableBlock int
	focusedTableRow   int
	focusedTableCol   int
	activeTextSegment woxcomponent.NoteDocumentSegment
	summaries         []common.NoteSummary
	searchIndex       int
	searchOpen        bool
	moreOpen          bool
	formatMore        bool
	linkOpen          bool
	formatVisible     bool
	dirty             bool
	saving            bool
	errorText         string
	saveTimer         *time.Timer
	zoom              float32
	lastFrame         woxui.Size
	requestedSize     woxui.Size
	manualSize        bool
	undoDocuments     []common.NoteDocument
	redoDocuments     []common.NoteDocument
	lastTextEdit      time.Time
	tooltipRev        atomic.Uint64
	windowPinned      bool
	windowMaximized   bool
	restoreFrame      woxui.Rect
}

func newNotesWindowController(app *App, record common.NoteRecord) *notesWindowController {
	controller := &notesWindowController{
		app: app, editor: woxwidget.NewTextEditingController(""), editorFocus: woxwidget.NewFocusNode(),
		searchEditor: woxwidget.NewTextEditingController(""), searchFocus: woxwidget.NewFocusNode(),
		linkEditor: woxwidget.NewTextEditingController("https://"), linkFocus: woxwidget.NewFocusNode(),
		windowID: woxui.WindowID("wox.notes." + newID()), formatVisible: true, zoom: 1, focusedTableBlock: -1,
	}
	controller.applyRecord(record)
	return controller
}

// activeNotesController returns the last active note window, then falls back to another live note window.
func (a *App) activeNotesController() *notesWindowController {
	if a.activeNote != nil && a.activeNote.managed != nil && a.activeNote.managed.Lifecycle() != woxui.WindowLifecycleClosed {
		return a.activeNote
	}
	for _, controller := range a.noteWindows {
		if controller.managed != nil && controller.managed.Lifecycle() == woxui.WindowLifecycleVisible {
			return controller
		}
	}
	for _, controller := range a.noteWindows {
		if controller.managed != nil && controller.managed.Lifecycle() != woxui.WindowLifecycleClosed {
			return controller
		}
	}
	return nil
}

func (a *App) activateNoteWindow(controller *notesWindowController) {
	if controller == nil {
		return
	}
	a.activeNote = controller
	_ = a.services.NotesSetLocal(context.Background(), "currentNoteId", controller.record.ID)
}

func (a *App) removeNoteWindow(controller *notesWindowController) {
	if controller == nil || a.noteWindows[controller.record.ID] != controller {
		return
	}
	delete(a.noteWindows, controller.record.ID)
	if a.activeNote == controller {
		a.activeNote = nil
	}
}

// openNewNoteWindow opens an empty draft and reuses one that the user has not typed into yet.
func (a *App) openNewNoteWindow() error {
	for _, controller := range a.noteWindows {
		if controller != nil && controller.record.DeletedAt == 0 && notesplugin.DocumentIsEmpty(controller.document) {
			return controller.open(common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: controller.record.ID})
		}
	}
	record, err := a.services.NotesCreate(context.Background())
	if err != nil {
		return err
	}
	return a.openNoteRecord(record, common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID})
}

// openNoteRequest resolves one note without replacing the document shown by another window.
func (a *App) openNoteRequest(request common.NotesWindowRequest) error {
	if request.Action == common.NotesWindowToggle {
		anyVisible := false
		for _, controller := range a.noteWindows {
			anyVisible = anyVisible || controller.managed != nil && controller.managed.Lifecycle() == woxui.WindowLifecycleVisible
		}
		if anyVisible {
			for _, controller := range a.noteWindows {
				if controller.managed != nil && controller.managed.Lifecycle() == woxui.WindowLifecycleVisible {
					controller.hide()
				}
			}
			return nil
		}
		active := a.activeNotesController()
		for _, controller := range a.noteWindows {
			if controller != active {
				if err := controller.open(request); err != nil {
					return err
				}
			}
		}
		if active != nil {
			return active.open(request)
		}
	}
	if request.Action == common.NotesWindowNew {
		return a.openNewNoteWindow()
	}
	explicitNoteID := request.NoteID != ""
	if request.NoteID == "" {
		request.NoteID, _ = a.services.NotesGetLocal(context.Background(), "currentNoteId")
	}
	if request.NoteID != "" {
		if controller := a.noteWindows[request.NoteID]; controller != nil {
			controller.refresh(request.NoteID)
			return controller.open(request)
		}
		record, err := a.services.NotesGet(context.Background(), request.NoteID)
		if err != nil {
			if explicitNoteID {
				return err
			}
		} else {
			return a.openNoteRecord(record, request)
		}
	}
	items, err := a.services.NotesList(context.Background(), "", false)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return a.openNewNoteWindow()
	}
	record, err := a.services.NotesGet(context.Background(), items[0].ID)
	if err != nil {
		return err
	}
	request.Action, request.NoteID = common.NotesWindowOpen, record.ID
	return a.openNoteRecord(record, request)
}

// openNoteRecord reuses an existing window for the same note and otherwise creates a new controller.
func (a *App) openNoteRecord(record common.NoteRecord, request common.NotesWindowRequest) error {
	if record.ID == "" {
		return fmt.Errorf("note ID is required")
	}
	if controller := a.noteWindows[record.ID]; controller != nil {
		controller.refresh(record.ID)
		return controller.open(request)
	}
	controller := newNotesWindowController(a, record)
	if err := controller.reloadSummaries(); err != nil {
		return err
	}
	a.noteWindows[record.ID] = controller
	a.activeNote = controller
	if err := controller.open(request); err != nil {
		if controller.managed != nil {
			_ = controller.managed.Close()
		}
		delete(a.noteWindows, record.ID)
		if a.activeNote == controller {
			a.activeNote = nil
		}
		return err
	}
	return nil
}

// ensure creates the independent Notes utility window once per native lifetime.
func (c *notesWindowController) ensure() (*woxui.ManagedWindow, error) {
	if c == nil || c.app == nil {
		return nil, fmt.Errorf("Notes controller is not initialized")
	}
	var managed *woxui.ManagedWindow
	var openErr error
	created := false
	if err := woxui.Call(func() {
		if c.managed != nil && c.managed.Lifecycle() != woxui.WindowLifecycleClosed {
			managed = c.managed
			return
		}
		host := woxwidget.NewHost(c.buildNotes)
		managed, _, openErr = c.app.windows.Open(c.windowID, woxui.WindowOptions{
			Title: c.app.translate("i18n:notes_title"), Size: woxui.Size{Width: notesDefaultWidth, Height: notesDefaultHeight},
			MinSize: notesNativeMinSize(),
			Role:    notesWindowRole, Icon: notesWindowIcon(), Resizable: true, TransientOverlay: true, Topmost: c.windowPinned, HideOnBlur: false,
			OnFrame: host.Frame, OnPointer: host.Pointer,
			OnFocus: func(event woxui.FocusEvent) {
				host.SetWindowFocused(event.Active)
				if event.Active {
					c.app.activateNoteWindow(c)
				}
			},
			OnKey: func(event woxui.KeyEvent) bool {
				if host.Key(event) {
					return true
				}
				return c.onKey(event)
			},
			OnTextInput:      func(event woxui.TextInputEvent) { host.TextInput(event) },
			OnCloseRequested: c.requestClose,
			OnClosed: func() {
				c.discardEmptyNote()
				if !notesplugin.DocumentIsEmpty(c.document) {
					_ = c.flush()
				}
				host.Dispose()
				c.managed, c.host = nil, nil
				c.app.removeNoteWindow(c)
			},
		})
		if openErr == nil {
			host.Attach(managed.Window())
			c.managed, c.host = managed, host
			created = true
		}
	}); err != nil {
		return nil, err
	}
	if openErr != nil || !created {
		return managed, openErr
	}
	window := managed.Window()
	if err := window.SetAppearance(themeColorIsDark(c.app.palette.background)); err != nil {
		return nil, err
	}
	if err := window.SetFontFamily(c.app.generalSettings.Data().AppFontFamily); err != nil {
		return nil, err
	}
	if err := window.SetTopmost(c.windowPinned); err != nil {
		return nil, err
	}
	c.windowMaximized = c.readWindowMaximized()
	if !c.restoreBounds(window) {
		_ = window.CenterOnMouseScreen(woxui.Size{Width: notesDefaultWidth, Height: notesDefaultHeight})
		if bounds, err := window.Bounds(); err == nil {
			offset := float32((len(c.app.noteWindows) - 1) % 8 * 24)
			bounds.X, bounds.Y = bounds.X+offset, bounds.Y+offset
			_ = window.SetBounds(clampNotesBounds(bounds))
		}
	}
	c.applyRestoredMaximize(window)
	return managed, nil
}

// open presents this note's independent utility window.
func (c *notesWindowController) open(request common.NotesWindowRequest) error {
	managed, err := c.ensure()
	if err != nil {
		return err
	}
	if request.ExportFormat == "choose" {
		c.moreOpen = true
	}
	c.resizeForDocument()
	if _, err := managed.Show(); err != nil {
		return err
	}
	c.app.activateNoteWindow(c)
	c.editorFocus.RequestFocus()
	if request.ExportFormat != "" && request.ExportFormat != "choose" {
		c.export(request.ExportFormat)
	}
	return managed.Window().Invalidate()
}

// applyRecord projects a durable record into this window's editor state.
func (c *notesWindowController) applyRecord(record common.NoteRecord) {
	c.record, c.document = record, record.Document
	c.focusedTableBlock = -1
	value, runs, ranges := c.projectActiveText()
	c.richRuns, c.blockRanges = runs, ranges
	c.editor.SetText(value, false)
	c.selection = c.editor.State().Selection
	c.dirty, c.errorText = false, ""
	c.undoDocuments, c.redoDocuments = nil, nil
	c.windowPinned = c.readWindowPinned()
	c.applyWindowTopmost()
	c.resizeForDocument()
}

func (c *notesWindowController) editorStyle() woxui.TextStyle {
	return woxui.TextStyle{Size: 14 * c.zoom, Family: woxui.FontFamilyUI}
}

// onEditorChanged converts the plain backing value into rich blocks and schedules persistence.
func (c *notesWindowController) onEditorChanged(value string) {
	c.onSegmentChanged(c.activeTextSegment.Start, value)
}

func (c *notesWindowController) onSegmentChanged(segmentStart int, value string) {
	previous := c.document
	c.rememberDocumentUndo(previous, time.Since(c.lastTextEdit) < 750*time.Millisecond)
	c.lastTextEdit = time.Now()
	segment := woxcomponent.NoteSegmentAtBlock(previous, segmentStart)
	parsed := documentFromEditor(value, woxcomponent.NoteSegmentDocument(previous, segment))
	c.document = woxcomponent.ReplaceNoteSegment(previous, segment, parsed.Blocks)
	projected, runs, ranges := c.projectActiveText()
	c.richRuns, c.blockRanges = runs, ranges
	if segment.Start == c.activeTextSegment.Start && projected != value {
		focus := c.editor.State().Selection.Focus - (utf8.RuneCountInString(value) - utf8.RuneCountInString(projected))
		c.editor.SetText(projected, false)
		c.editor.SetCaret(max(0, focus))
	} else if segment.Start == c.activeTextSegment.Start {
		c.editor.SetText(projected, false)
	}
	c.selection = c.editor.State().Selection
	c.dirty, c.errorText = true, ""
	c.scheduleSave()
	c.invalidate()
}

func (c *notesWindowController) projectActiveText() (string, []woxcomponent.NoteTextRun, []noteBlockRange) {
	c.activeTextSegment = c.firstTextSegment()
	return woxcomponent.ProjectNoteSegment(c.document, c.activeTextSegment, c.editorStyle(), c.app.palette.componentTheme())
}

func (c *notesWindowController) firstTextSegment() woxcomponent.NoteDocumentSegment {
	for _, segment := range woxcomponent.NoteDocumentSegments(c.document) {
		if !segment.Table {
			return segment
		}
	}
	return woxcomponent.NoteDocumentSegment{Start: 0, End: len(c.document.Blocks)}
}

// scheduleSave coalesces typing into the 500 ms autosave boundary.
func (c *notesWindowController) scheduleSave() {
	if c.saveTimer != nil {
		c.saveTimer.Stop()
	}
	c.saveTimer = time.AfterFunc(notesAutosaveDelay, func() {
		_ = c.app.runOnUI("autosave note", func() {
			if err := c.flush(); err != nil {
				util.GetLogger().Error(context.Background(), fmt.Sprintf("autosave note: %v", err))
			}
			c.invalidate()
		})
	})
}

// flush persists dirty state while keeping failed edits retryable.
func (c *notesWindowController) flush() error {
	if !c.dirty || c.record.ID == "" || c.app == nil || c.app.services == nil {
		return nil
	}
	if notesplugin.DocumentIsEmpty(c.document) {
		if c.saveTimer != nil {
			c.saveTimer.Stop()
			c.saveTimer = nil
		}
		return nil
	}
	if c.saveTimer != nil {
		c.saveTimer.Stop()
		c.saveTimer = nil
	}
	c.saving = true
	previousID := c.record.ID
	result, err := c.app.services.NotesSave(context.Background(), c.record.ID, c.record.Revision, c.document)
	c.saving = false
	if err != nil {
		c.errorText = c.app.translate("i18n:notes_save_failed") + ": " + err.Error()
		return err
	}
	c.record = result.Record
	c.document = result.Record.Document
	c.dirty = false
	if c.record.ID != previousID {
		if c.app.noteWindows[previousID] == c {
			delete(c.app.noteWindows, previousID)
		}
		c.app.noteWindows[c.record.ID] = c
	}
	if result.Conflict {
		c.errorText = c.app.translate("i18n:notes_conflict_copy")
	} else {
		c.errorText = ""
	}
	return c.reloadSummaries()
}

// reloadSummaries refreshes search, pin, and trash rows for the current filter.
func (c *notesWindowController) reloadSummaries() error {
	query := ""
	if c.searchOpen {
		query = strings.TrimSpace(c.searchEditor.Text())
	}
	items, err := c.app.services.NotesList(context.Background(), query, true)
	if err != nil {
		c.errorText = err.Error()
		return err
	}
	c.summaries = items
	c.clampSearchIndex()
	return nil
}

// refresh applies external sync changes only when no dirty local edit would be replaced.
func (c *notesWindowController) refresh(noteID string) {
	_ = c.reloadSummaries()
	if c.record.ID == "" || (noteID != "" && noteID != c.record.ID) || c.dirty {
		c.invalidate()
		return
	}
	if record, err := c.app.services.NotesGet(context.Background(), c.record.ID); err == nil && record.Revision != c.record.Revision {
		c.applyRecord(record)
	} else if err != nil && !notesplugin.DocumentIsEmpty(c.document) {
		c.errorText = err.Error()
	}
	c.invalidate()
}

func (c *notesWindowController) hide() {
	if c == nil || c.managed == nil {
		return
	}
	c.updateToolbarTooltip(false, "", woxui.Rect{})
	if notesplugin.DocumentIsEmpty(c.document) {
		c.discardEmptyNote()
		if err := c.managed.Close(); err != nil {
			util.GetLogger().Error(context.Background(), fmt.Sprintf("close empty Notes window: %v", err))
		}
		return
	}
	_ = c.flush()
	c.persistBounds()
	if err := c.managed.Hide(); err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("hide Notes window: %v", err))
	}
}

// close flushes the bound note before releasing this native window.
func (c *notesWindowController) close() error {
	if c == nil || c.managed == nil {
		return nil
	}
	c.updateToolbarTooltip(false, "", woxui.Rect{})
	if notesplugin.DocumentIsEmpty(c.document) {
		c.discardEmptyNote()
	} else if err := c.flush(); err != nil {
		return err
	}
	c.persistBounds()
	return c.managed.Close()
}

// discardEmptyNote drops an untitled draft so opening a note without typing does not persist it.
func (c *notesWindowController) discardEmptyNote() {
	if c == nil || c.record.ID == "" || !notesplugin.DocumentIsEmpty(c.document) {
		return
	}
	if c.saveTimer != nil {
		c.saveTimer.Stop()
		c.saveTimer = nil
	}
	c.dirty = false
	if err := c.app.services.NotesDiscard(context.Background(), c.record.ID); err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("discard empty note: %v", err))
	}
}

// requestClose avoids re-entering native destruction from a platform close-request callback.
func (c *notesWindowController) requestClose() {
	util.Go(c.app.lifecycleCtx, "close Notes window", func() {
		if err := c.app.runOnUI("close Notes window", func() {
			if err := c.close(); err != nil {
				c.fail(err)
			}
		}); err != nil {
			util.GetLogger().Error(context.Background(), fmt.Sprintf("close Notes window: %v", err))
		}
	})
}

// persistBounds stores logical coordinates and zoom as device-local preferences.
func (c *notesWindowController) persistBounds() {
	if c.managed == nil {
		return
	}
	bounds, err := c.managed.Window().Bounds()
	if err != nil {
		return
	}
	saved := notesWindowBounds{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height}
	if c.windowMaximized && c.restoreFrame.Width > 0 && c.restoreFrame.Height > 0 {
		saved = notesWindowBounds{X: c.restoreFrame.X, Y: c.restoreFrame.Y, Width: c.restoreFrame.Width, Height: c.restoreFrame.Height}
	}
	clamped := clampNotesBounds(woxui.Rect{X: saved.X, Y: saved.Y, Width: saved.Width, Height: saved.Height})
	saved = notesWindowBounds{X: clamped.X, Y: clamped.Y, Width: clamped.Width, Height: clamped.Height}
	encoded, _ := json.Marshal(saved)
	_ = c.app.services.NotesSetLocal(context.Background(), c.preferenceKey("windowBounds"), string(encoded))
	_ = c.app.services.NotesSetLocal(context.Background(), c.preferenceKey("zoom"), fmt.Sprintf("%.2f", c.zoom))
	_ = c.app.services.NotesSetLocal(context.Background(), c.preferenceKey("windowPinned"), map[bool]string{true: "1", false: "0"}[c.windowPinned])
	_ = c.app.services.NotesSetLocal(context.Background(), c.preferenceKey("windowMaximized"), map[bool]string{true: "1", false: "0"}[c.windowMaximized])
}

// restoreBounds clamps saved logical geometry into a currently connected work area.
func (c *notesWindowController) restoreBounds(window *woxui.Window) bool {
	raw := c.localPreference("windowBounds")
	var saved notesWindowBounds
	if zoom, err := strconv.ParseFloat(c.localPreference("zoom"), 32); err == nil {
		c.zoom = max(float32(.75), min(float32(2), float32(zoom)))
	}
	if raw == "" || json.Unmarshal([]byte(raw), &saved) != nil || saved.Width <= 0 || saved.Height <= 0 {
		return false
	}
	c.manualSize = true
	bounds := clampNotesBounds(woxui.Rect{X: saved.X, Y: saved.Y, Width: saved.Width, Height: saved.Height})
	c.restoreFrame = bounds
	return window.SetBounds(bounds) == nil
}

func clampNotesBounds(bounds woxui.Rect) woxui.Rect {
	bounds.Width = max(notesMinimumWidth, bounds.Width)
	bounds.Height = max(notesMinimumHeight, min(notesMaximumHeight, bounds.Height))
	displays, err := screen.ListDisplays()
	if err != nil || len(displays) == 0 {
		return bounds
	}
	return clampNotesBoundsToDisplays(bounds, displays)
}

// clampNotesBoundsToDisplays chooses the largest logical overlap, including negative origins.
func clampNotesBoundsToDisplays(bounds woxui.Rect, displays []screen.Display) woxui.Rect {
	bounds.Width = max(notesMinimumWidth, bounds.Width)
	bounds.Height = max(notesMinimumHeight, min(notesMaximumHeight, bounds.Height))
	best := displays[0].WorkArea
	bestArea := -1
	for _, display := range displays {
		work := display.WorkArea
		left, top := max(int(bounds.X), work.X), max(int(bounds.Y), work.Y)
		right, bottom := min(int(bounds.X+bounds.Width), work.Right()), min(int(bounds.Y+bounds.Height), work.Bottom())
		area := max(0, right-left) * max(0, bottom-top)
		if area > bestArea || (area == bestArea && display.Primary) {
			best, bestArea = work, area
		}
	}
	bounds.Width = min(bounds.Width, float32(best.Width))
	bounds.Height = min(bounds.Height, float32(best.Height))
	bounds.X = max(float32(best.X), min(bounds.X, float32(best.Right())-bounds.Width))
	bounds.Y = max(float32(best.Y), min(bounds.Y, float32(best.Bottom())-bounds.Height))
	return bounds
}

// resizeForDocument auto-fits content until the user manually resizes this window lifetime.
func (c *notesWindowController) resizeForDocument() {
	if c.manualSize || c.windowMaximized || c.managed == nil || len(c.document.Blocks) == 0 {
		return
	}
	lineCount := 0
	for _, block := range c.document.Blocks {
		lineCount += max(1, (utf8.RuneCountInString(block.Text)+35)/36)
	}
	height := max(notesMinimumHeight, min(notesMaximumHeight, float32(98+lineCount*24)))
	bounds, err := c.managed.Window().Bounds()
	if err != nil {
		return
	}
	if bounds.Width <= 0 {
		bounds.Width = notesDefaultWidth
	}
	bounds.Height = height
	bounds = clampNotesBounds(bounds)
	c.requestedSize = woxui.Size{Width: bounds.Width, Height: bounds.Height}
	_ = c.managed.Window().SetBounds(bounds)
}

func abs32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func (c *notesWindowController) local(key string) (string, error) {
	return c.app.services.NotesGetLocal(context.Background(), key)
}

func (c *notesWindowController) preferenceKey(key string) string {
	return key + ":" + c.record.ID
}

// localPreference reads per-note state and migrates the former singleton preference for the last active note.
func (c *notesWindowController) localPreference(key string) string {
	if value, _ := c.local(c.preferenceKey(key)); value != "" {
		return value
	}
	currentNoteID, _ := c.local("currentNoteId")
	if currentNoteID == c.record.ID {
		value, _ := c.local(key)
		return value
	}
	return ""
}

func (c *notesWindowController) invalidate() {
	if c.managed != nil {
		_ = c.managed.Window().Invalidate()
	}
}

// buildNotes derives this window's complete widget tree from controller snapshot state.
func (c *notesWindowController) buildNotes(frame woxui.FrameInfo) woxwidget.Widget {
	a := c.app
	if c.lastFrame.Width > 0 && (abs32(frame.Size.Width-c.lastFrame.Width) > 1 || abs32(frame.Size.Height-c.lastFrame.Height) > 1) {
		requested := c.requestedSize.Width > 0 && abs32(frame.Size.Width-c.requestedSize.Width) <= 1 && abs32(frame.Size.Height-c.requestedSize.Height) <= 1
		if !requested {
			c.manualSize = true
			if c.windowMaximized {
				c.syncMaximizedFromFrame(frame.Size)
			}
		}
	}
	c.lastFrame = frame.Size
	if c.requestedSize.Width > 0 && abs32(frame.Size.Width-c.requestedSize.Width) <= 1 && abs32(frame.Size.Height-c.requestedSize.Height) <= 1 {
		c.requestedSize = woxui.Size{}
	}
	theme := a.palette.componentTheme()
	// Match shared text overlays: let native acrylic or vibrancy paint the window surface.
	theme.Background = overlay.PanelFill(runtime.GOOS, !themeColorIsDark(theme.Background))
	toolbar := c.buildToolbar(frame.Size.Width, frame.WindowFocused, theme)
	formatHeight := float32(0)
	var formatBar woxwidget.Widget
	if c.formatVisible {
		formatHeight = launcherview.NotesFormatBarHeight
		formatBar = c.buildFormatBar(frame.Size.Width, theme)
	}
	statusHeight := float32(0)
	var status woxwidget.Widget
	if c.errorText != "" || c.saving {
		statusHeight = launcherview.NotesStatusHeight
		status = c.buildStatus(frame.Size.Width, theme)
	}
	editorHeight := max(float32(0), frame.Size.Height-launcherview.NotesToolbarHeight-formatHeight-statusHeight)
	editor := woxcomponent.WoxNoteEditor(woxcomponent.NoteEditorProps{
		ID: "notes.editor", Label: a.translate("i18n:notes_editor"), Document: c.document,
		Width: frame.Size.Width, Height: editorHeight, Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 24},
		Style: c.editorStyle(), LineHeight: 24 * c.zoom, Zoom: c.zoom, TextColor: theme.PreviewText, Theme: theme,
		Window: c.managed.Window(), ReadOnly: c.record.DeletedAt > 0, Autofocus: true, Controller: c.editor,
		FocusNode: c.editorFocus, Focused: c.editorFocus.HasFocus() && c.focusedTableBlock < 0, Selection: c.selection,
		OnChanged: c.onSegmentChanged, OnSelectionChanged: func(selection woxui.TextSelection) {
			if selection == c.selection {
				return
			}
			c.selection = selection
			c.focusedTableBlock = -1
			c.invalidate()
		}, OnTapOffset: c.handleBlockTap, CursorAtOffset: c.editorCursorAt, OnKey: c.onKey,
		OnUndo: c.undoDocument, OnRedo: c.redoDocument, OnPaste: c.pasteDocument,
		TransformPaste: func(value string) string { return notesplugin.ToMarkdown(notesplugin.ParseMarkdown(value)) },
		OnTableChange:  c.replaceTable, OnTableFocus: c.focusTableCell, OnTableKey: c.onTableKey, OnTablePaste: c.pasteTableClipboard,
		OnDeleteEmptySegment: c.deleteEmptyTextSegment,
		OnTableInsertRow:     c.tableInsertRow, OnTableInsertColumn: c.tableInsertColumn, OnTableDeleteRow: c.tableDeleteRow,
		OnTableDeleteColumn: c.tableDeleteColumn, OnTableDelete: c.tableDelete, OnTableActionHover: c.updateTableActionTooltip,
		TableActionLabels: woxcomponent.NoteTableActionLabels{
			InsertRow: c.app.translate("i18n:notes_table_insert_row"), InsertColumn: c.app.translate("i18n:notes_table_insert_column"),
			DeleteRow: c.app.translate("i18n:notes_table_delete_row"), DeleteColumn: c.app.translate("i18n:notes_table_delete_column"),
			DeleteTable: c.app.translate("i18n:notes_table_delete"),
		},
		FocusedTableBlock: c.focusedTableBlock, FocusedTableRow: c.focusedTableRow, FocusedTableCol: c.focusedTableCol,
	})
	var overlay woxwidget.Widget
	if c.linkOpen {
		overlay = c.buildLinkOverlay(frame.Size, theme)
	} else if c.searchOpen {
		overlay = c.buildSearchOverlay(frame.Size, theme)
	} else if c.moreOpen {
		overlay = c.buildMoreOverlay(frame.Size, theme)
	}
	return launcherview.NotesWindow(launcherview.NotesWindowProps{
		Width: frame.Size.Width, Height: frame.Size.Height, Label: a.translate("i18n:notes_title"), Toolbar: toolbar, Editor: editor,
		FormatBar: formatBar, Status: status, Overlay: overlay, Theme: theme,
	})
}

func (c *notesWindowController) buildToolbar(width float32, active bool, theme woxcomponent.Theme) woxwidget.Widget {
	const buttonSize = float32(32)
	hoverBackground := woxcomponent.TitleBarAlpha(theme.ToolbarText, 20)
	button := func(id, label string, icon woxwidget.Widget, disabled bool, action func()) woxwidget.Widget {
		onTap := func() {
			c.updateToolbarTooltip(false, "", woxui.Rect{})
			if action != nil {
				action()
			}
		}
		return woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "notes.toolbar." + id, Label: label, Icon: icon, Width: buttonSize, Height: buttonSize, Radius: 5,
			HoverBackground: hoverBackground, FocusRingColor: theme.Cursor, Disabled: disabled, OnTap: onTap,
			OnHoverAt: func(inside bool, bounds woxui.Rect) { c.updateToolbarTooltip(inside, label, bounds) },
		})
	}
	color := theme.ToolbarText
	title := c.app.translate("i18n:notes_untitled")
	if len(c.document.Blocks) > 0 && strings.TrimSpace(c.document.Blocks[0].Text) != "" {
		title = strings.TrimSpace(strings.SplitN(c.document.Blocks[0].Text, "\n", 2)[0])
	}
	searchLabel := c.app.translate("i18n:notes_search") + " (" + strings.Join(formatHotkeyLabels(primaryHotkey("p")), "+") + ")"
	pinHotkey := "(" + strings.Join(formatHotkeyLabels(primaryHotkey("shift+p")), "+") + ")"
	pinLabel := c.app.translate("i18n:notes_pin_window") + " " + pinHotkey
	if c.windowPinned {
		pinLabel = c.app.translate("i18n:notes_unpin_window") + " " + pinHotkey
	}
	pinColor := color
	if c.windowPinned {
		pinColor = woxcomponent.DocumentListMarkerColor
	}
	newLabel := c.app.translate("i18n:notes_new") + " (" + strings.Join(formatHotkeyLabels(primaryHotkey("n")), "+") + ")"
	right := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 2, Children: []woxwidget.Widget{
		button("search", searchLabel, woxcomponent.SearchGlyph(15, color), false, c.toggleSearch),
		button("pin", pinLabel, woxcomponent.PinGlyph(15, pinColor), false, c.toggleWindowPin),
		button("format", c.app.translate("i18n:notes_format"), woxwidget.Text{Value: "Aa", Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: color}, false, func() { c.formatVisible = !c.formatVisible; c.invalidate() }),
		button("new", newLabel, woxcomponent.AddGlyph(15, color), false, func() { c.runAction(c.app.openNewNoteWindow) }),
		button("more", c.app.translate("i18n:notes_more"), woxcomponent.MenuGlyph(15, color), false, func() { c.moreOpen = !c.moreOpen; c.formatMore = false; c.searchOpen = false; c.invalidate() }),
	}}
	contentRight := woxcomponent.TitleBarChromeWidth(runtime.GOOS, true, true)
	const titleSlot = float32(220)
	titleLeft, titleRight, titleAlignment := titleSlot, titleSlot, float32(.5)
	if runtime.GOOS == "windows" {
		titleLeft, titleRight, titleAlignment = 40, contentRight+174, 0
	}
	drag := woxwidget.Semantics{AutomationID: "notes.toolbar.drag", Role: woxui.AccessibilityRoleGroup, Label: c.app.translate("i18n:notes_title"), Child: woxwidget.Gesture{
		ID: "notes.toolbar.drag",
		OnDragStart: func() {
			if c.windowMaximized {
				c.restoreFromMaximize()
			}
			if c.managed != nil {
				_ = c.managed.Window().StartDragging()
			}
		},
		OnDoubleTap: c.toggleMaximize,
		Child:       woxwidget.Container{Width: width, Height: launcherview.NotesToolbarHeight},
	}}
	children := []woxwidget.StackChild{
		{Child: drag},
		{AnchorBottom: true, Child: woxwidget.Container{Width: width, Height: 1, Color: woxcomponent.TitleBarAlpha(theme.PreviewSplit, 76)}},
		{Left: titleLeft, Right: titleRight, StretchWidth: true, Child: woxwidget.Align{Height: launcherview.NotesToolbarHeight, Horizontal: titleAlignment, Vertical: .5, Child: woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ToolbarText}}},
		{Right: contentRight + 6, AnchorRight: true, Top: 4, Child: right},
	}
	if runtime.GOOS == "windows" {
		children = append(children, woxwidget.StackChild{Left: 12, Child: woxwidget.Align{Width: 20, Height: launcherview.NotesToolbarHeight, Vertical: .5, Child: woxwidget.Image{Source: notesTitleBarIcon, Width: 20, Height: 20}}})
	}
	children = append(children, woxwidget.StackChild{Child: woxcomponent.WindowCloseChrome(woxcomponent.WindowCloseChromeProps{
		ID: "notes.toolbar.close", Width: width, Platform: runtime.GOOS, Theme: theme, Active: active, Maximized: c.windowMaximized,
		OnMinimize: c.minimizeWindow, OnMaximize: c.toggleMaximize, OnClose: c.requestClose,
	})})
	content := woxwidget.Stack{Width: width, Height: launcherview.NotesToolbarHeight, Children: children}
	return woxwidget.Container{Width: width, Height: launcherview.NotesToolbarHeight, Color: theme.Background, Child: content}
}

// updateToolbarTooltip keeps Notes chrome hints in the same native overlay used by the launcher.
func (c *notesWindowController) updateToolbarTooltip(inside bool, text string, anchor woxui.Rect) {
	c.app.setNativeHoverTooltip(&c.tooltipRev, "go-ui-notes-titlebar", "update Notes chrome tooltip", inside, text, anchor, "top", func() *woxui.Window {
		if c.managed == nil {
			return nil
		}
		return c.managed.Window()
	})
}

func (c *notesWindowController) buildFormatBar(width float32, theme woxcomponent.Theme) woxwidget.Widget {
	formats := noteActiveFormats(c.document, c.blockRanges, c.selection)
	if c.focusedTableBlock >= 0 {
		formats = noteActiveFormatsForTable(c.document, c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol)
	}
	item := func(id string, action func()) woxwidget.Widget {
		label := c.app.translate("i18n:notes_format_" + id)
		onTap := func() {
			c.updateToolbarTooltip(false, "", woxui.Rect{})
			if action != nil {
				action()
			}
		}
		iconColor := theme.ToolbarText
		if formats[id] && theme.Cursor.A != 0 {
			iconColor = theme.Cursor
		}
		return woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "notes.format." + id, Label: label, Icon: woxcomponent.FormatGlyph(id, 16, iconColor),
			Width: 28, Height: 28, Radius: 6, HoverBackground: woxcomponent.TitleBarAlpha(theme.ToolbarText, 20),
			Selected: formats[id], SelectedBackground: woxcomponent.TitleBarAlpha(theme.ToolbarText, 40),
			FocusRingColor: theme.Cursor, Disabled: c.record.DeletedAt > 0, OnTap: onTap,
			OnHoverAt: func(inside bool, bounds woxui.Rect) { c.updateToolbarTooltip(inside, label, bounds) },
		})
	}
	items := []woxwidget.Widget{
		// Heading cycles the current block through paragraph, H1-H3, and code.
		item("block", func() { c.cycleBlock() }),
		item("bold", func() { c.toggleInline("bold") }),
		item("italic", func() { c.toggleInline("italic") }),
		item("underline", func() { c.toggleInline("underline") }),
		item("strike", func() { c.toggleInline("strike") }),
		item("code", func() { c.toggleInline("code") }),
		item("link", c.openLink),
		item("bullet", func() { c.setBlock(common.NoteBlockBullet) }),
		item("ordered", func() { c.setBlock(common.NoteBlockOrdered) }),
		item("task", func() { c.setBlock(common.NoteBlockTask) }),
		item("quote", func() { c.setBlock(common.NoteBlockQuote) }),
		item("divider", func() { c.setBlock(common.NoteBlockDivider) }),
		item("table", c.insertTable),
	}
	if width < 390 {
		items = append(items[:7], item("more", func() { c.moreOpen, c.formatMore = true, true; c.invalidate() }))
	}
	return woxwidget.Container{Width: width, Height: launcherview.NotesFormatBarHeight, Color: theme.ToolbarBackground, BorderColor: theme.PreviewSplit, BorderWidth: 1,
		Child: woxwidget.Align{Width: width, Height: launcherview.NotesFormatBarHeight, Horizontal: .5, Vertical: .5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 2, Children: items}}}
}

func (c *notesWindowController) buildStatus(width float32, theme woxcomponent.Theme) woxwidget.Widget {
	text, color := c.errorText, theme.ErrorText
	if c.saving {
		text, color = c.app.translate("i18n:notes_saving"), theme.ResultSubtitle
	}
	children := []woxwidget.Widget{woxwidget.Expanded{Child: woxwidget.Text{Value: text, Style: woxui.TextStyle{Size: 11}, Color: color}}}
	if c.errorText != "" && c.dirty {
		children = append(children, woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "notes.retry", Label: c.app.translate("i18n:notes_retry"), Width: 52, FontSize: 11, Theme: theme, OnTap: func() { c.runAction(c.flush) }}))
	}
	return woxwidget.Container{Width: width, Height: launcherview.NotesStatusHeight, Padding: woxwidget.Insets{Left: 12, Right: 8}, Color: theme.ToolbarBackground, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}}
}

func (c *notesWindowController) buildSearchOverlay(size woxui.Size, theme woxcomponent.Theme) woxwidget.Widget {
	width := max(float32(260), size.Width-24)
	height := min(float32(300), size.Height-40)
	items := c.searchItems()
	c.clampSearchIndex()
	rows := make([]woxwidget.Widget, 0, len(items)+6)
	contentHeight := float32(0)
	var keepVisible *woxwidget.ScrollRange
	appendSearchRow := func(child woxwidget.Widget, height float32, selected bool) {
		if len(rows) > 0 {
			contentHeight += notesSearchListGap
		}
		if selected {
			keepVisible = &woxwidget.ScrollRange{Start: contentHeight, End: contentHeight + height}
		}
		rows = append(rows, child)
		contentHeight += height
	}
	section := ""
	for index, summary := range items {
		label := c.searchSectionLabel(summary)
		if label != section {
			first := section == ""
			section = label
			headerHeight := notesSearchSectionHeight
			if !first {
				headerHeight += notesSearchSectionLead
			}
			appendSearchRow(notesSearchSection(label, width-notesSearchOverlayPadding*2, theme, first), headerHeight, false)
		}
		selected := index == c.searchIndex
		appendSearchRow(c.notesListRow(summary, width-notesSearchOverlayPadding*2, theme, selected), notesSearchRowHeight, selected)
	}
	if len(rows) == 0 {
		empty := woxwidget.Container{Width: width - notesSearchOverlayPadding*2, Height: 48, Child: woxwidget.Align{Width: width - notesSearchOverlayPadding*2, Height: 48, Horizontal: .5, Vertical: .5, Child: woxwidget.Text{Value: c.app.translate("i18n:notes_no_results"), Style: woxui.TextStyle{Size: 12}, Color: theme.ResultSubtitle}}}
		appendSearchRow(empty, 48, false)
	}
	var window *woxui.Window
	if c.managed != nil {
		window = c.managed.Window()
	}
	innerWidth := width - notesSearchOverlayPadding*2
	scrollHeight := height - notesSearchOverlayPadding*2 - notesSearchFieldHeight - notesSearchOverlayGap
	search := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "notes.search", Label: c.app.translate("i18n:notes_search"), Hint: c.app.translate("i18n:notes_search_placeholder"),
		Width: innerWidth, Height: notesSearchFieldHeight, Style: woxui.TextStyle{Size: 13}, Value: c.searchEditor.Text(), Controller: c.searchEditor,
		FocusNode: c.searchFocus, Focused: true, Autofocus: true, MaxLines: 1, Window: window, Theme: theme,
		OnChanged: func(string) { _ = c.reloadSummaries(); c.searchIndex = 0; c.invalidate() }, OnKey: c.onSearchKey,
	})
	results := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "notes.search.results", AutomationID: "notes.search.results", Label: c.app.translate("i18n:notes_search"),
		Width: innerWidth, Height: scrollHeight, ContentHeight: contentHeight, KeepVisible: keepVisible,
		Content:    woxwidget.Flex{Axis: woxwidget.Vertical, Gap: notesSearchListGap, Children: rows},
		ThumbColor: theme.ResultSubtitle,
	})
	panel := woxwidget.Container{Width: width, Height: height, Radius: 10, Color: theme.ActionBackground, BorderColor: theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.UniformInsets(notesSearchOverlayPadding), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: notesSearchOverlayGap, Children: []woxwidget.Widget{
		search, results,
	}}}
	return c.overlayScrim(size, panel, 12, 44, func() { c.searchOpen = false; c.invalidate() })
}

func (c *notesWindowController) notesListRow(summary common.NoteSummary, width float32, theme woxcomponent.Theme, selected bool) woxwidget.Widget {
	label := summary.Title
	if summary.DeletedAt > 0 {
		label += " · " + c.app.translate("i18n:notes_restore")
	}
	radius := float32(6)
	background := theme.ActionBackground
	titleColor := theme.ActionText
	metaColor := theme.ResultSubtitle
	if selected {
		background = theme.SelectedBackground
		titleColor = theme.SelectedTitle
		metaColor = theme.SelectedSubtitle
	}
	hoverBackground := woxcomponent.ControlHoverColor(theme.ActionBackground, theme.ActionText)
	return woxcomponent.WoxListItem(woxcomponent.ListItemProps{
		ID: "notes.search." + summary.ID, Label: label, Width: width, Height: notesSearchRowHeight, Radius: &radius,
		Background: &background, HoverBackground: &hoverBackground, Selected: selected, SkipFocus: true,
		Padding: woxwidget.Insets{Left: 8, Right: 8}, Theme: theme,
		OnTap: func() { c.openSearchItem(summary) },
		Child: notesSearchRowChild(label, util.FormatTimestamp(summary.UpdatedAt), titleColor, metaColor),
	})
}

// notesSearchRowChild centers the title and updated time in one search result row.
func notesSearchRowChild(label, updated string, titleColor, metaColor woxui.Color) woxwidget.Widget {
	return woxwidget.Align{Height: notesSearchRowHeight, Vertical: .5, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter,
		Children: []woxwidget.Widget{
			woxwidget.Expanded{Child: woxwidget.TextBlock{
				Value: label, Height: notesSearchTextHeight, LineHeight: notesSearchTextHeight, MaxLines: 1, AlignmentY: 0.5,
				Style: woxui.TextStyle{Size: 13}, Color: titleColor,
			}},
			woxwidget.TextBlock{
				Value: updated, Height: notesSearchTextHeight, LineHeight: notesSearchTextHeight, MaxLines: 1, AlignmentY: 0.5, ShrinkWrap: true,
				Style: woxui.TextStyle{Size: woxcomponent.TailFontSize}, Color: metaColor,
			},
		},
	}}
}

// searchItems returns notes in overlay order: pinned, recent, then recently deleted.
func (c *notesWindowController) searchItems() []common.NoteSummary {
	items := make([]common.NoteSummary, 0, len(c.summaries))
	appendMatching := func(match func(common.NoteSummary) bool) {
		for _, item := range c.summaries {
			if match(item) {
				items = append(items, item)
			}
		}
	}
	appendMatching(func(item common.NoteSummary) bool { return item.PinnedAt > 0 && item.DeletedAt == 0 })
	appendMatching(func(item common.NoteSummary) bool { return item.PinnedAt == 0 && item.DeletedAt == 0 })
	appendMatching(func(item common.NoteSummary) bool { return item.DeletedAt > 0 })
	return items
}

// notesSearchSection renders a muted section label, with extra lead-in space after the first group.
func notesSearchSection(label string, width float32, theme woxcomponent.Theme, first bool) woxwidget.Widget {
	top := float32(0)
	if !first {
		top = notesSearchSectionLead
	}
	return woxwidget.Container{Width: width, Height: notesSearchSectionHeight + top, Padding: woxwidget.Insets{Left: 8, Top: top}, Child: woxwidget.Align{Height: notesSearchSectionHeight, Vertical: .5, Child: woxwidget.TextBlock{
		Value: label, Height: notesSearchSectionTextHeight, LineHeight: notesSearchSectionTextHeight, MaxLines: 1, AlignmentY: 0.5,
		Style: woxui.TextStyle{Size: woxcomponent.SettingsSectionTitleFontSize, Weight: woxui.FontWeightSemibold}, Color: theme.ActionHeader,
	}}}
}

func (c *notesWindowController) searchSectionLabel(item common.NoteSummary) string {
	switch {
	case item.DeletedAt > 0:
		return c.app.translate("i18n:notes_deleted")
	case item.PinnedAt > 0:
		return c.app.translate("i18n:notes_pinned")
	default:
		return c.app.translate("i18n:notes_recent")
	}
}

// clampSearchIndex keeps keyboard selection on a visible row after the filtered list changes.
func (c *notesWindowController) clampSearchIndex() {
	count := len(c.searchItems())
	if count == 0 {
		c.searchIndex = 0
		return
	}
	c.searchIndex = min(max(0, c.searchIndex), count-1)
}

func (c *notesWindowController) moveSearchSelection(delta int) {
	c.searchIndex += delta
	c.clampSearchIndex()
	c.invalidate()
}

// openSearchItem shows the chosen note in this window instead of opening another utility window.
func (c *notesWindowController) openSearchItem(summary common.NoteSummary) {
	if summary.ID == c.record.ID && summary.DeletedAt == 0 {
		c.searchOpen = false
		c.editorFocus.RequestFocus()
		c.invalidate()
		return
	}
	if err := c.persistCurrentNote(); err != nil {
		c.fail(err)
		return
	}
	var record common.NoteRecord
	var err error
	if summary.DeletedAt > 0 {
		record, err = c.app.services.NotesRestore(context.Background(), summary.ID)
	} else {
		record, err = c.app.services.NotesGet(context.Background(), summary.ID)
	}
	if err != nil {
		c.fail(err)
		return
	}
	if err := c.showNoteInCurrentWindow(record); err != nil {
		c.fail(err)
		return
	}
	c.searchOpen = false
	c.editorFocus.RequestFocus()
	c.invalidate()
}

// persistCurrentNote saves edits, or drops an unused empty draft, before this window changes notes.
func (c *notesWindowController) persistCurrentNote() error {
	if notesplugin.DocumentIsEmpty(c.document) {
		c.discardEmptyNote()
		return nil
	}
	return c.flush()
}

// showNoteInCurrentWindow rebinds this native window to another note.
func (c *notesWindowController) showNoteInCurrentWindow(record common.NoteRecord) error {
	if record.ID == "" {
		return fmt.Errorf("note ID is required")
	}
	if record.ID == c.record.ID {
		return nil
	}
	if existing := c.app.noteWindows[record.ID]; existing != nil && existing != c && existing.managed != nil && existing.managed.Lifecycle() != woxui.WindowLifecycleClosed {
		return existing.open(common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID})
	}
	previousID := c.record.ID
	c.persistBounds()
	if c.app.noteWindows[previousID] == c {
		delete(c.app.noteWindows, previousID)
	}
	if c.app.noteWindows == nil {
		c.app.noteWindows = map[string]*notesWindowController{}
	}
	c.applyRecord(record)
	c.app.noteWindows[record.ID] = c
	c.app.activateNoteWindow(c)
	c.searchEditor.SetText("", false)
	c.searchIndex = 0
	return c.reloadSummaries()
}

func (c *notesWindowController) activateSearchSelection() {
	items := c.searchItems()
	if len(items) == 0 {
		return
	}
	c.openSearchItem(items[c.searchIndex])
}

func (c *notesWindowController) buildMoreOverlay(size woxui.Size, theme woxcomponent.Theme) woxwidget.Widget {
	width := float32(190)
	if c.formatMore {
		rows := []woxwidget.Widget{
			c.menuRow("format-bullet", c.app.translate("i18n:notes_format_bullet"), width, theme, func() { c.setBlock(common.NoteBlockBullet) }),
			c.menuRow("format-ordered", c.app.translate("i18n:notes_format_ordered"), width, theme, func() { c.setBlock(common.NoteBlockOrdered) }),
			c.menuRow("format-task", c.app.translate("i18n:notes_format_task"), width, theme, func() { c.setBlock(common.NoteBlockTask) }),
			c.menuRow("format-quote", c.app.translate("i18n:notes_format_quote"), width, theme, func() { c.setBlock(common.NoteBlockQuote) }),
			c.menuRow("format-divider", c.app.translate("i18n:notes_format_divider"), width, theme, func() { c.setBlock(common.NoteBlockDivider) }),
			c.menuRow("format-table", c.app.translate("i18n:notes_format_table"), width, theme, c.insertTable),
		}
		if c.focusedTableBlock >= 0 {
			rows = append(rows,
				c.menuRow("table-insert-row", c.app.translate("i18n:notes_table_insert_row"), width, theme, c.insertTableRow),
				c.menuRow("table-insert-column", c.app.translate("i18n:notes_table_insert_column"), width, theme, c.insertTableColumn),
				c.menuRow("table-delete-row", c.app.translate("i18n:notes_table_delete_row"), width, theme, c.deleteTableRow),
				c.menuRow("table-delete-column", c.app.translate("i18n:notes_table_delete_column"), width, theme, c.deleteTableColumn),
				c.menuRow("table-delete", c.app.translate("i18n:notes_table_delete"), width, theme, c.deleteFocusedTable),
			)
		}
		return c.moreOverlay(size, width, rows, theme)
	}
	rows := []woxwidget.Widget{
		c.menuRow("copy-link", c.app.translate("i18n:notes_copy_link"), width, theme, c.copyLink),
		c.menuRow("export-md", c.app.translate("i18n:notes_export_markdown"), width, theme, func() { c.export("md") }),
		c.menuRow("export-txt", c.app.translate("i18n:notes_export_text"), width, theme, func() { c.export("txt") }),
		c.menuRow("export-html", c.app.translate("i18n:notes_export_html"), width, theme, func() { c.export("html") }),
	}
	if c.record.DeletedAt > 0 {
		rows = append(rows, c.menuRow("restore", c.app.translate("i18n:notes_restore"), width, theme, c.restore))
	} else {
		rows = append(rows, c.menuRow("delete", c.app.translate("i18n:notes_delete"), width, theme, c.delete))
	}
	return c.moreOverlay(size, width, rows, theme)
}

func (c *notesWindowController) moreOverlay(size woxui.Size, width float32, rows []woxwidget.Widget, theme woxcomponent.Theme) woxwidget.Widget {
	panelHeight := float32(len(rows))*32 + 12
	panel := woxwidget.Container{Width: width, Height: panelHeight, Radius: 9, Color: theme.ActionBackground, BorderColor: theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.UniformInsets(6), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
	return c.overlayScrim(size, panel, max(float32(8), size.Width-width-8), 40, func() { c.moreOpen, c.formatMore = false, false; c.invalidate() })
}

func (c *notesWindowController) menuRow(id, label string, width float32, theme woxcomponent.Theme, action func()) woxwidget.Widget {
	radius := float32(6)
	background := theme.ActionBackground
	hoverBackground := woxcomponent.ControlHoverColor(background, theme.ActionText)
	return woxcomponent.WoxListItem(woxcomponent.ListItemProps{
		ID: "notes.menu." + id, Label: label, Width: width - 12, Height: 32, Radius: &radius,
		Background: &background, HoverBackground: &hoverBackground, SkipFocus: true,
		OnTap: func() { c.moreOpen = false; action(); c.invalidate() }, Theme: theme,
		Padding: woxwidget.Insets{Left: 9},
		Child:   woxwidget.Align{Height: 32, Vertical: .5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12}, Color: theme.ActionText}},
	})
}

func (c *notesWindowController) buildLinkOverlay(size woxui.Size, theme woxcomponent.Theme) woxwidget.Widget {
	width := min(float32(340), size.Width-32)
	field := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{ID: "notes.link", Label: c.app.translate("i18n:notes_link"), Hint: "https://", Width: width - 20, Height: 36, Style: woxui.TextStyle{Size: 13}, Value: c.linkEditor.Text(), Controller: c.linkEditor, FocusNode: c.linkFocus, Focused: true, Autofocus: true, Window: c.managed.Window(), Theme: theme, OnKey: c.onLinkKey})
	buttons := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, MainAxisAlignment: woxwidget.MainAxisEnd, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "notes.link.cancel", Label: c.app.translate("i18n:cancel"), Width: 70, Theme: theme, OnTap: func() { c.linkOpen = false; c.invalidate() }}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "notes.link.apply", Label: c.app.translate("i18n:notes_apply"), Width: 70, Theme: theme, Variant: woxcomponent.ButtonPrimary, OnTap: c.applyLink}),
	}}
	panel := woxwidget.Container{Width: width, Height: 100, Radius: 10, Color: theme.ActionBackground, BorderColor: theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.UniformInsets(10), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{field, buttons}}}
	return c.overlayScrim(size, panel, (size.Width-width)/2, 52, func() { c.linkOpen = false; c.invalidate() })
}

func (c *notesWindowController) overlayScrim(size woxui.Size, panel woxwidget.Widget, left, top float32, dismiss func()) woxwidget.Widget {
	return woxwidget.Stack{Width: size.Width, Height: size.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Semantics{AutomationID: "notes.overlay.dismiss", Role: woxui.AccessibilityRoleButton, Label: c.app.translate("i18n:close"), Child: woxwidget.Gesture{ID: "notes.overlay.dismiss", OnTap: dismiss, Child: woxwidget.Container{Width: size.Width, Height: size.Height, Color: woxui.Color{A: 70}}}}},
		{Left: left, Top: top, Child: panel},
	}}
}

func (c *notesWindowController) runAction(action func() error) {
	if err := action(); err != nil {
		c.fail(err)
	}
	c.invalidate()
}

func (c *notesWindowController) fail(err error) {
	if err != nil {
		c.errorText = err.Error()
		util.GetLogger().Error(context.Background(), fmt.Sprintf("Notes action: %v", err))
	}
	c.invalidate()
}

// toggleSearch switches focus between the editor and keyboard-first note browser.
func (c *notesWindowController) toggleSearch() {
	c.searchOpen, c.moreOpen, c.linkOpen = !c.searchOpen, false, false
	if c.searchOpen {
		c.searchIndex = 0
		_ = c.reloadSummaries()
		c.searchFocus.RequestFocus()
	} else {
		c.editorFocus.RequestFocus()
	}
	c.invalidate()
}

func (c *notesWindowController) onSearchKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		c.searchOpen = false
		c.editorFocus.RequestFocus()
		c.invalidate()
		return true
	case woxui.KeyArrowDown:
		c.moveSearchSelection(1)
		return true
	case woxui.KeyArrowUp:
		c.moveSearchSelection(-1)
		return true
	case woxui.KeyEnter:
		c.activateSearchSelection()
		return true
	default:
		return false
	}
}

func (c *notesWindowController) onLinkKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		c.linkOpen = false
		c.editorFocus.RequestFocus()
		c.invalidate()
		return true
	case woxui.KeyEnter:
		c.applyLink()
		return true
	default:
		return false
	}
}

func (c *notesWindowController) onKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	if event.Key == woxui.KeyEscape {
		if c.searchOpen || c.moreOpen || c.linkOpen {
			c.searchOpen, c.moreOpen, c.linkOpen = false, false, false
			c.editorFocus.RequestFocus()
			c.invalidate()
			return true
		}
		c.requestClose()
		return true
	}
	if event.Key == woxui.KeyEnter && event.Modifiers == 0 && c.continueBlock() {
		return true
	}
	if event.Key == woxui.KeyTab && event.Modifiers&^woxui.KeyModifierShift == 0 {
		delta := 1
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		return c.changeListIndent(delta)
	}
	if !event.Modifiers.HasPrimary() {
		return false
	}
	switch event.Key {
	case woxui.Key("p"):
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			c.toggleWindowPin()
		} else {
			c.toggleSearch()
		}
	case woxui.Key("n"):
		c.runAction(c.app.openNewNoteWindow)
	case woxui.Key("b"):
		c.toggleInline("bold")
	case woxui.Key("i"):
		c.toggleInline("italic")
	case woxui.Key("u"):
		c.toggleInline("underline")
	case woxui.Key("x"):
		if event.Modifiers&woxui.KeyModifierShift == 0 {
			return false
		}
		c.toggleInline("strike")
	case woxui.Key("e"):
		c.toggleInline("code")
	case woxui.Key("k"):
		c.openLink()
	case woxui.Key("z"):
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			return c.redoDocument()
		}
		return c.undoDocument()
	case woxui.Key("y"):
		return c.redoDocument()
	case woxui.KeyEnter:
		c.toggleTask()
	case woxui.Key("0"):
		c.setZoom(1)
	case woxui.Key("+"), woxui.Key("="):
		c.setZoom(c.zoom + .1)
	case woxui.Key("-"):
		c.setZoom(c.zoom - .1)
	default:
		if event.Key >= woxui.Key("1") && event.Key <= woxui.Key("9") {
			index := int(event.Key[0] - '1')
			pinned := make([]common.NoteSummary, 0)
			for _, item := range c.summaries {
				if item.PinnedAt > 0 && item.DeletedAt == 0 {
					pinned = append(pinned, item)
				}
			}
			if index < len(pinned) {
				c.runAction(func() error {
					return c.app.openNoteRequest(common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: pinned[index].ID})
				})
			}
			return true
		}
		return false
	}
	return true
}

// continueBlock applies Enter directly to the block model before the plain text field inserts a newline.
func (c *notesWindowController) continueBlock() bool {
	if c.record.DeletedAt > 0 {
		return false
	}
	document, block, handled := continueNoteBlock(c.document, c.blockRanges, c.editor.State().Selection)
	if !handled {
		return false
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = document
	c.reproject(false)
	if block < len(c.blockRanges) {
		c.editor.SetCaret(c.blockRanges[block].TextStart)
		c.selection = c.editor.State().Selection
		c.invalidate()
	}
	return true
}

func (c *notesWindowController) changeListIndent(delta int) bool {
	if c.record.DeletedAt > 0 {
		return false
	}
	selection := c.editor.State().Selection
	document, block, changed, handled := adjustNoteListIndent(c.document, c.blockRanges, selection, delta)
	if !handled || !changed {
		return handled
	}
	oldTextStart := c.blockRanges[block].TextStart
	c.rememberDocumentUndo(c.document, false)
	c.document = document
	c.reproject(false)
	shift := c.blockRanges[block].TextStart - oldTextStart
	c.editor.SetCaret(selection.Focus + shift)
	c.selection = c.editor.State().Selection
	c.invalidate()
	return true
}

func (c *notesWindowController) toggleInline(kind string) {
	if c.record.DeletedAt > 0 {
		return
	}
	if c.focusedTableBlock >= 0 {
		c.rememberDocumentUndo(c.document, false)
		c.document = woxcomponent.ToggleNoteTableInline(c.document, c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol, kind, "")
		c.reproject(false)
		return
	}
	if c.selection.Collapsed() {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = toggleNoteInline(c.document, c.blockRanges, c.selection, kind, "")
	c.reproject(false)
}

func (c *notesWindowController) openLink() {
	if c.record.DeletedAt > 0 || (c.selection.Collapsed() && c.focusedTableBlock < 0) {
		return
	}
	c.linkEditor.SetText("https://", true)
	c.linkOpen, c.moreOpen, c.searchOpen = true, false, false
	c.linkFocus.RequestFocus()
	c.invalidate()
}

func (c *notesWindowController) applyLink() {
	link := strings.TrimSpace(c.linkEditor.Text())
	if link == "" {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	if c.focusedTableBlock >= 0 {
		c.document = woxcomponent.ToggleNoteTableInline(c.document, c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol, "link", link)
	} else {
		c.document = toggleNoteInline(c.document, c.blockRanges, c.selection, "link", link)
	}
	c.linkOpen = false
	c.reproject(false)
	c.editorFocus.RequestFocus()
}

func (c *notesWindowController) setBlock(blockType common.NoteBlockType) {
	if c.record.DeletedAt > 0 || len(c.document.Blocks) == 0 || c.focusedTableBlock >= 0 {
		return
	}
	index := noteBlockAt(c.blockRanges, c.selection.Focus)
	if index >= 0 && index < len(c.document.Blocks) && c.document.Blocks[index].Type != common.NoteBlockTable {
		c.rememberDocumentUndo(c.document, false)
		c.document.Blocks[index].Type = blockType
		if blockType != common.NoteBlockTask {
			c.document.Blocks[index].Checked = false
		}
		if blockType != common.NoteBlockBullet && blockType != common.NoteBlockOrdered && blockType != common.NoteBlockTask {
			c.document.Blocks[index].Indent = 0
		}
		c.document.Blocks[index].Table = nil
		c.reproject(false)
	}
}

func (c *notesWindowController) insertTable() {
	if c.record.DeletedAt > 0 {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	document, index := woxcomponent.InsertNoteTable(c.document, c.blockRanges, c.selection)
	c.document = document
	c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol = index, 0, 0
	c.reproject(false)
}

func (c *notesWindowController) cycleBlock() {
	if len(c.document.Blocks) == 0 || c.focusedTableBlock >= 0 {
		return
	}
	index := noteBlockAt(c.blockRanges, c.selection.Focus)
	if index < 0 || index >= len(c.document.Blocks) {
		return
	}
	sequence := []common.NoteBlockType{common.NoteBlockParagraph, common.NoteBlockHeading1, common.NoteBlockHeading2, common.NoteBlockHeading3, common.NoteBlockCode}
	current := slices.Index(sequence, c.document.Blocks[index].Type)
	c.setBlock(sequence[(current+1)%len(sequence)])
}

func (c *notesWindowController) toggleTask() {
	if len(c.document.Blocks) == 0 {
		return
	}
	c.toggleTaskBlock(noteBlockAt(c.blockRanges, c.selection.Focus))
}

func (c *notesWindowController) handleBlockTap(offset int) bool {
	if index, ok := noteTaskAtOffset(c.document, c.blockRanges, offset); ok {
		return c.toggleTaskBlock(index)
	}
	if noteDividerAtOffset(c.document, c.blockRanges, offset) {
		return true
	}
	if target := noteLinkAtOffset(c.document, c.blockRanges, offset); target != "" {
		c.openNoteLink(target)
		return true
	}
	return false
}

// editorCursorAt distinguishes links and task markers from ordinary editable text.
func (c *notesWindowController) editorCursorAt(offset int) woxui.PointerCursor {
	if _, ok := noteTaskAtOffset(c.document, c.blockRanges, offset); ok {
		return woxui.PointerCursorHand
	}
	if noteLinkAtOffset(c.document, c.blockRanges, offset) != "" {
		return woxui.PointerCursorHand
	}
	if noteDividerAtOffset(c.document, c.blockRanges, offset) {
		return woxui.PointerCursorDefault
	}
	return woxui.PointerCursorText
}

// openNoteLink opens a persisted note URL in the desktop browser.
func (c *notesWindowController) openNoteLink(target string) {
	if c.managed == nil {
		return
	}
	if err := c.managed.Window().OpenExternalURL(target); err != nil {
		c.fail(err)
	}
}

// toggleTaskBlock updates one task through the same undo and autosave path as keyboard formatting.
func (c *notesWindowController) toggleTaskBlock(index int) bool {
	if c.record.DeletedAt > 0 || index < 0 || index >= len(c.document.Blocks) || c.document.Blocks[index].Type != common.NoteBlockTask {
		return false
	}
	c.rememberDocumentUndo(c.document, false)
	c.document.Blocks[index].Checked = !c.document.Blocks[index].Checked
	c.reproject(false)
	return true
}

// reproject updates visible rich runs after a document-level formatting change.
func (c *notesWindowController) reproject(resetSelection bool) {
	state := c.editor.State()
	value, runs, ranges := c.projectActiveText()
	c.richRuns, c.blockRanges = runs, ranges
	if value != state.Text {
		c.editor.SetText(value, false)
		if !resetSelection {
			c.editor.SetSelection(min(state.Selection.Anchor, utf8.RuneCountInString(value)), min(state.Selection.Focus, utf8.RuneCountInString(value)))
		}
	}
	c.selection = c.editor.State().Selection
	c.dirty, c.errorText = true, ""
	c.scheduleSave()
	c.invalidate()
}

func (c *notesWindowController) pasteDocument(value string) bool {
	pasted := notesplugin.ParseClipboard(value)
	if !noteClipboardHasStructure(pasted) {
		return false
	}
	c.rememberDocumentUndo(c.document, false)
	index := 0
	if c.focusedTableBlock >= 0 {
		index = c.focusedTableBlock
	} else if len(c.blockRanges) > 0 {
		index = noteBlockAt(c.blockRanges, c.selection.Focus)
	}
	if index < 0 || index >= len(c.document.Blocks) {
		c.document.Blocks = append(c.document.Blocks, pasted.Blocks...)
	} else if c.document.Blocks[index].Type == common.NoteBlockParagraph && strings.TrimSpace(c.document.Blocks[index].Text) == "" {
		c.document.Blocks = slices.Replace(c.document.Blocks, index, index+1, pasted.Blocks...)
	} else {
		c.document.Blocks = slices.Insert(c.document.Blocks, index+1, pasted.Blocks...)
	}
	if pasted.Blocks[0].Type == common.NoteBlockTable {
		c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol = index+1, 0, 0
		if index < len(c.document.Blocks) && c.document.Blocks[index].Type == common.NoteBlockTable && strings.TrimSpace(c.document.Blocks[index].Text) == "" {
			c.focusedTableBlock = index
		}
	}
	c.reproject(false)
	return true
}

func noteClipboardHasStructure(document common.NoteDocument) bool {
	if len(document.Blocks) != 1 {
		return true
	}
	return document.Blocks[0].Type == common.NoteBlockTable
}

func (c *notesWindowController) replaceTable(block int, table common.NoteTable) {
	c.rememberDocumentUndo(c.document, true)
	c.document = woxcomponent.ReplaceNoteTable(c.document, block, &table)
	c.dirty, c.errorText = true, ""
	c.scheduleSave()
	c.invalidate()
}

func (c *notesWindowController) focusTableCell(block, row, column int) {
	c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol = block, row, column
	c.invalidate()
}

func (c *notesWindowController) pasteTableClipboard(block, row, column int, value string) bool {
	pasted := notesplugin.ParseClipboard(value)
	if noteClipboardHasStructure(pasted) {
		return c.pasteDocument(value)
	}
	return false
}

func (c *notesWindowController) onTableKey(block, row, column int, event woxui.KeyEvent) bool {
	if !event.Down || event.Composing || block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Table == nil {
		return false
	}
	table := *c.document.Blocks[block].Table
	switch event.Key {
	case woxui.KeyTab:
		delta := 1
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		nextRow, nextCol, ok := woxcomponent.NextNoteTableCell(table, row, column, 0, delta)
		if ok {
			c.focusTableCell(block, nextRow, nextCol)
			return true
		}
		c.focusedTableBlock = -1
		c.editorFocus.RequestFocus()
		c.invalidate()
		return true
	case woxui.KeyEnter:
		nextRow, nextCol, ok := woxcomponent.NextNoteTableCell(table, row, column, 1, 0)
		if !ok {
			c.rememberDocumentUndo(c.document, false)
			c.document = woxcomponent.InsertNoteTableRow(c.document, block, row)
			c.focusTableCell(block, row+1, column)
			c.reproject(false)
			return true
		}
		c.focusTableCell(block, nextRow, nextCol)
		return true
	case woxui.KeyArrowUp:
		if row == 0 {
			c.focusedTableBlock = -1
			c.editorFocus.RequestFocus()
			c.invalidate()
			return true
		}
		c.focusTableCell(block, row-1, column)
		return true
	case woxui.KeyArrowDown:
		if row+1 >= len(table.Rows) {
			c.focusedTableBlock = -1
			c.editorFocus.RequestFocus()
			c.invalidate()
			return true
		}
		c.focusTableCell(block, row+1, column)
		return true
	default:
		return false
	}
}

func (c *notesWindowController) insertTableRow() {
	c.tableInsertRow(c.focusedTableBlock)
}

func (c *notesWindowController) insertTableColumn() {
	c.tableInsertColumn(c.focusedTableBlock)
}

func (c *notesWindowController) deleteTableRow() {
	c.tableDeleteRow(c.focusedTableBlock)
}

func (c *notesWindowController) deleteTableColumn() {
	c.tableDeleteColumn(c.focusedTableBlock)
}

func (c *notesWindowController) deleteFocusedTable() {
	c.tableDelete(c.focusedTableBlock)
}

func (c *notesWindowController) tableInsertRow(block int) {
	row, column, ok := c.tableActionTarget(block)
	if !ok {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.InsertNoteTableRow(c.document, block, row)
	c.focusTableCell(block, row+1, column)
	c.reproject(false)
}

func (c *notesWindowController) tableInsertColumn(block int) {
	row, column, ok := c.tableActionTarget(block)
	if !ok {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.InsertNoteTableColumn(c.document, block, column)
	c.focusTableCell(block, row, column+1)
	c.reproject(false)
}

func (c *notesWindowController) tableDeleteRow(block int) {
	row, column, ok := c.tableActionTarget(block)
	if !ok {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.DeleteNoteTableRow(c.document, block, row)
	if table := c.document.Blocks[block].Table; table != nil {
		c.focusTableCell(block, min(row, len(table.Rows)-1), column)
	}
	c.reproject(false)
}

func (c *notesWindowController) tableDeleteColumn(block int) {
	row, column, ok := c.tableActionTarget(block)
	if !ok {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.DeleteNoteTableColumn(c.document, block, column)
	if table := c.document.Blocks[block].Table; table != nil {
		c.focusTableCell(block, row, min(column, noteTableColumnCount(*table)-1))
	}
	c.reproject(false)
}

// deleteEmptyTextSegment removes a blank paragraph so Backspace can close the gap between tables.
func (c *notesWindowController) deleteEmptyTextSegment(segmentStart int) bool {
	if c.record.DeletedAt > 0 {
		return false
	}
	segment := woxcomponent.NoteSegmentAtBlock(c.document, segmentStart)
	updated, ok := woxcomponent.RemoveEmptyNoteSegment(c.document, segment)
	if !ok {
		return false
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = updated
	if prev := segment.Start - 1; prev >= 0 && prev < len(c.document.Blocks) && c.document.Blocks[prev].Type == common.NoteBlockTable {
		row := 0
		if table := c.document.Blocks[prev].Table; table != nil && len(table.Rows) > 0 {
			row = len(table.Rows) - 1
		}
		c.focusTableCell(prev, row, 0)
	} else {
		c.focusedTableBlock = -1
		if c.editorFocus != nil {
			c.editorFocus.RequestFocus()
		}
	}
	c.reproject(true)
	return true
}

func (c *notesWindowController) tableDelete(block int) {
	if block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Table == nil {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.DeleteNoteTable(c.document, block)
	c.focusedTableBlock = -1
	c.reproject(false)
}

func (c *notesWindowController) tableActionTarget(block int) (int, int, bool) {
	if block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Table == nil {
		return 0, 0, false
	}
	table := *c.document.Blocks[block].Table
	row, column := len(table.Rows)-1, 0
	if columns := noteTableColumnCount(table); columns > 0 {
		column = columns - 1
	}
	if c.focusedTableBlock == block {
		row, column = c.focusedTableRow, c.focusedTableCol
	}
	return row, column, true
}

func noteTableColumnCount(table common.NoteTable) int {
	columns := 1
	for _, row := range table.Rows {
		columns = max(columns, len(row))
	}
	return columns
}

func (c *notesWindowController) updateTableActionTooltip(inside bool, label string, bounds woxui.Rect) {
	c.updateToolbarTooltip(inside, label, bounds)
}

// rememberDocumentUndo keeps bounded document snapshots and coalesces adjacent typing.
func (c *notesWindowController) rememberDocumentUndo(document common.NoteDocument, coalesce bool) {
	if coalesce && len(c.undoDocuments) > 0 {
		return
	}
	if !coalesce {
		c.lastTextEdit = time.Time{}
	}
	if len(c.undoDocuments) >= 100 {
		copy(c.undoDocuments, c.undoDocuments[1:])
		c.undoDocuments = c.undoDocuments[:99]
	}
	c.undoDocuments = append(c.undoDocuments, cloneNoteDocument(document))
	c.redoDocuments = nil
}

func (c *notesWindowController) undoDocument() bool {
	if len(c.undoDocuments) == 0 || c.record.DeletedAt > 0 {
		return true
	}
	previous := c.undoDocuments[len(c.undoDocuments)-1]
	c.undoDocuments = c.undoDocuments[:len(c.undoDocuments)-1]
	c.redoDocuments = append(c.redoDocuments, cloneNoteDocument(c.document))
	c.document = previous
	c.lastTextEdit = time.Time{}
	c.reproject(true)
	return true
}

func (c *notesWindowController) redoDocument() bool {
	if len(c.redoDocuments) == 0 || c.record.DeletedAt > 0 {
		return true
	}
	next := c.redoDocuments[len(c.redoDocuments)-1]
	c.redoDocuments = c.redoDocuments[:len(c.redoDocuments)-1]
	c.undoDocuments = append(c.undoDocuments, cloneNoteDocument(c.document))
	c.document = next
	c.lastTextEdit = time.Time{}
	c.reproject(true)
	return true
}

func (c *notesWindowController) setZoom(value float32) {
	c.zoom = max(float32(.75), min(float32(2), value))
	projected, runs, ranges := c.projectActiveText()
	c.richRuns, c.blockRanges = runs, ranges
	if projected != c.editor.Text() {
		selection := c.editor.State().Selection
		c.editor.SetText(projected, false)
		c.editor.SetSelection(selection.Anchor, selection.Focus)
	}
	_ = c.app.services.NotesSetLocal(context.Background(), c.preferenceKey("zoom"), fmt.Sprintf("%.2f", c.zoom))
	c.invalidate()
}

// readWindowPinned loads the per-note always-on-top preference. Missing values stay unpinned.
func (c *notesWindowController) readWindowPinned() bool {
	if c == nil || c.app == nil || c.app.services == nil || c.record.ID == "" {
		return false
	}
	return c.localPreference("windowPinned") == "1"
}

// applyWindowTopmost mirrors the persisted pin onto the live native window.
func (c *notesWindowController) applyWindowTopmost() {
	if c.managed == nil {
		return
	}
	_ = c.managed.Window().SetTopmost(c.windowPinned)
}

// readWindowMaximized loads the per-note maximized preference. Missing values stay restored.
func (c *notesWindowController) readWindowMaximized() bool {
	if c == nil || c.app == nil || c.app.services == nil || c.record.ID == "" {
		return false
	}
	return c.localPreference("windowMaximized") == "1"
}

// applyRestoredMaximize expands a just-restored note onto its work area when that state was persisted.
func (c *notesWindowController) applyRestoredMaximize(window *woxui.Window) {
	if !c.windowMaximized || window == nil {
		return
	}
	c.maximizeWindowOn(window)
}

// minimizeWindow sends this note to the taskbar or dock.
func (c *notesWindowController) minimizeWindow() {
	if c == nil || c.managed == nil {
		return
	}
	_ = c.managed.Window().Minimize()
}

// toggleMaximize switches between the last restored frame and the current display work area.
func (c *notesWindowController) toggleMaximize() {
	if c.windowMaximized {
		c.restoreFromMaximize()
		return
	}
	c.maximizeWindow()
}

func (c *notesWindowController) maximizeWindow() {
	if c.managed == nil {
		return
	}
	c.maximizeWindowOn(c.managed.Window())
}

func (c *notesWindowController) maximizeWindowOn(window *woxui.Window) {
	if window == nil {
		return
	}
	bounds, err := window.Bounds()
	if err != nil {
		return
	}
	if !c.windowMaximized || c.restoreFrame.Width <= 0 || c.restoreFrame.Height <= 0 {
		c.restoreFrame = bounds
	}
	target := notesMaximizeBounds(bounds)
	c.requestedSize = woxui.Size{Width: target.Width, Height: target.Height}
	if err := window.SetBounds(target); err != nil {
		return
	}
	c.windowMaximized = true
	c.manualSize = true
	c.persistMaximizePreference()
	c.persistBounds()
	c.invalidate()
}

func (c *notesWindowController) restoreFromMaximize() {
	if c.managed == nil {
		return
	}
	target := c.restoreFrame
	if target.Width <= 0 || target.Height <= 0 {
		if bounds, err := c.managed.Window().Bounds(); err == nil {
			target = woxui.Rect{X: bounds.X, Y: bounds.Y, Width: notesDefaultWidth, Height: notesDefaultHeight}
		} else {
			target = woxui.Rect{Width: notesDefaultWidth, Height: notesDefaultHeight}
		}
	}
	target = clampNotesBounds(target)
	c.requestedSize = woxui.Size{Width: target.Width, Height: target.Height}
	if err := c.managed.Window().SetBounds(target); err != nil {
		return
	}
	c.windowMaximized = false
	c.manualSize = true
	c.persistMaximizePreference()
	c.persistBounds()
	c.invalidate()
}

// syncMaximizedFromFrame clears maximize after an interactive resize leaves the work area.
func (c *notesWindowController) syncMaximizedFromFrame(size woxui.Size) {
	if !c.windowMaximized {
		return
	}
	var current woxui.Rect
	if c.managed != nil {
		if bounds, err := c.managed.Window().Bounds(); err == nil {
			current = bounds
		}
	}
	if current.Width <= 0 || current.Height <= 0 {
		current.Width, current.Height = size.Width, size.Height
	}
	target := notesMaximizeBounds(current)
	if abs32(size.Width-target.Width) <= 4 && abs32(size.Height-target.Height) <= 4 {
		return
	}
	c.windowMaximized = false
	c.restoreFrame = current
	c.persistMaximizePreference()
}

func (c *notesWindowController) persistMaximizePreference() {
	if c.app == nil || c.app.services == nil || c.record.ID == "" {
		return
	}
	_ = c.app.services.NotesSetLocal(context.Background(), c.preferenceKey("windowMaximized"), map[bool]string{true: "1", false: "0"}[c.windowMaximized])
}

// notesMaximizeBounds fills the work area that contains most of the current frame.
func notesMaximizeBounds(current woxui.Rect) woxui.Rect {
	displays, err := screen.ListDisplays()
	if err != nil || len(displays) == 0 {
		return current
	}
	return notesMaximizeBoundsToDisplays(current, displays)
}

func notesMaximizeBoundsToDisplays(current woxui.Rect, displays []screen.Display) woxui.Rect {
	if len(displays) == 0 {
		return current
	}
	best := displays[0].WorkArea
	bestArea := -1
	for _, display := range displays {
		work := display.WorkArea
		left, top := max(int(current.X), work.X), max(int(current.Y), work.Y)
		right, bottom := min(int(current.X+current.Width), work.Right()), min(int(current.Y+current.Height), work.Bottom())
		area := max(0, right-left) * max(0, bottom-top)
		if area > bestArea || (area == bestArea && display.Primary) {
			best, bestArea = work, area
		}
	}
	return woxui.Rect{X: float32(best.X), Y: float32(best.Y), Width: float32(best.Width), Height: float32(best.Height)}
}

// toggleWindowPin keeps this note's utility window above other applications.
func (c *notesWindowController) toggleWindowPin() {
	c.windowPinned = !c.windowPinned
	if c.app != nil && c.app.services != nil && c.record.ID != "" {
		_ = c.app.services.NotesSetLocal(context.Background(), c.preferenceKey("windowPinned"), map[bool]string{true: "1", false: "0"}[c.windowPinned])
	}
	c.applyWindowTopmost()
	c.invalidate()
}

func (c *notesWindowController) copyLink() {
	if c.record.ID == "" {
		return
	}
	link := "wox://plugin/" + common.NotesPluginID + "?action=open&id=" + c.record.ID
	if err := clipboard.WriteText(link); err != nil {
		c.fail(err)
	}
}

// delete soft-deletes the note and closes only its bound window.
func (c *notesWindowController) delete() {
	if c.record.ID == "" {
		return
	}
	c.runAction(func() error {
		if notesplugin.DocumentIsEmpty(c.document) {
			c.discardEmptyNote()
			return c.close()
		}
		if err := c.flush(); err != nil {
			return err
		}
		_, err := c.app.services.NotesDelete(context.Background(), c.record.ID)
		if err != nil {
			return err
		}
		return c.close()
	})
}

func (c *notesWindowController) restore() {
	if c.record.ID == "" {
		return
	}
	c.runAction(func() error {
		record, err := c.app.services.NotesRestore(context.Background(), c.record.ID)
		if err == nil {
			c.applyRecord(record)
			err = c.reloadSummaries()
		}
		return err
	})
}

// export flushes first, then writes the selected codec through the native save dialog.
func (c *notesWindowController) export(format string) {
	c.runAction(func() error {
		if err := c.flush(); err != nil {
			return err
		}
		exported, err := c.app.services.NotesExport(context.Background(), c.record.ID, format)
		if err != nil {
			return err
		}
		title := "Untitled Note"
		if len(c.document.Blocks) > 0 && strings.TrimSpace(c.document.Blocks[0].Text) != "" {
			title = c.document.Blocks[0].Text
		}
		name := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`).ReplaceAllString(title, "-")
		name = strings.Trim(strings.TrimSpace(name), ".")
		if name == "" {
			name = "Untitled Note"
		}
		path, err := c.managed.Window().SaveFile(woxui.SaveFileOptions{Title: c.app.translate("i18n:notes_export"), DefaultFileName: name + "." + exported.Extension, Extension: exported.Extension})
		if err != nil || path == "" {
			return err
		}
		if filepath.Ext(path) == "" {
			path += "." + exported.Extension
		}
		return os.WriteFile(path, []byte(exported.Content), 0o600)
	})
}
