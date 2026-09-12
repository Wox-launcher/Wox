package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
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
	"wox/common/icons"
	notesplugin "wox/plugin/system/notes"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/clipboard"
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
	// WoxTextField's default chrome is 40 logical pixels; a shorter slot overflows the overlay flex.
	notesSearchFieldHeight    = float32(40)
	notesSearchOverlayPadding = float32(12)
	notesSearchOverlayGap     = float32(8)
	notesSearchOverlayTop     = float32(44)
	// notesEditorPaddingLeft keeps checklist glyphs and the left-gutter handle
	// clear of the native window resize hit zone. 40 also lines up with the
	// Windows title slot.
	notesEditorPaddingLeft   = float32(40)
	notesEditorPaddingRight  = float32(40)
	notesEditorPaddingTop    = float32(12)
	notesEditorPaddingBottom = float32(24)
	// notesToolbarActionsWidth is the five 32-wide title-bar buttons plus gaps
	// and the 6px inset used to place that cluster. Equal macOS title insets
	// used to be 220, which left only 20px at the 460 default width.
	notesToolbarActionsWidth = float32(174)
)

var notesTitleBarIcon, _ = decodeWoxImageWithTint(fromCoreImage(icons.Get(icons.PluginNotes)), nil, 256)

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

// noteUndoEntry restores the document, caret, and whole-note selection that belonged to it.
type noteUndoEntry struct {
	document         common.NoteDocument
	selection        woxui.TextSelection
	segment          woxcomponent.NoteDocumentSegment
	tableBlock       int
	imageBlock       int
	documentSelected bool
}

type notesWindowController struct {
	app               *App
	windowID          woxui.WindowID
	managed           *woxui.ManagedWindow
	host              *woxwidget.Host
	editor            *woxwidget.TextEditingController
	editorFocus       *woxwidget.FocusNode
	requestTextFocus  bool
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
	focusedImageBlock int
	// documentSelected is a whole-note selection that crosses images and tables.
	documentSelected  bool
	markdownView      bool
	revealEditorCaret bool
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
	undoDocuments     []noteUndoEntry
	redoDocuments     []noteUndoEntry
	lastTextEdit      time.Time
	tooltipRev        atomic.Uint64
	inlineTooltip     *settingsInlineTooltipState
	windowPinned      bool
	windowMaximized   bool
	restoreFrame      woxui.Rect
	taskReorderFrom   int
	taskReorderTo     int
	taskHover         int
}

func newNotesWindowController(app *App, record common.NoteRecord) *notesWindowController {
	controller := &notesWindowController{
		app: app, editor: woxwidget.NewTextEditingController(""), editorFocus: woxwidget.NewFocusNode(),
		searchEditor: woxwidget.NewTextEditingController(""), searchFocus: woxwidget.NewFocusNode(),
		linkEditor: woxwidget.NewTextEditingController("https://"), linkFocus: woxwidget.NewFocusNode(),
		windowID: woxui.WindowID("wox.notes." + newID()), formatVisible: true, zoom: 1, focusedTableBlock: -1, focusedImageBlock: -1,
		taskReorderFrom: -1, taskReorderTo: -1, taskHover: -1,
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
			Role:    notesWindowRole, Icon: notesWindowIcon(), Resizable: true, Topmost: c.windowPinned, HideOnBlur: false,
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
	c.record, c.document = record, notesplugin.EnsureNoteImageEditGaps(record.Document)
	notesplugin.HydrateNoteImageDimensions(c.document)
	c.activeTextSegment = woxcomponent.NoteDocumentSegment{}
	c.focusedTableBlock = -1
	c.focusedImageBlock = -1
	c.documentSelected = false
	c.taskReorderFrom, c.taskReorderTo, c.taskHover = -1, -1, -1
	if c.markdownView {
		c.editor.SetText(notesplugin.ToMarkdown(c.document), false)
		c.richRuns, c.blockRanges = nil, nil
	} else {
		value, runs, ranges := c.projectActiveText()
		c.richRuns, c.blockRanges = runs, ranges
		c.editor.SetText(value, false)
	}
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
	if c.documentSelected {
		// The active field only holds one run; replace the whole note so typing after
		// select-all does not leave images and later paragraphs behind.
		c.documentSelected = false
		c.document = notesplugin.EnsureNoteImageEditGaps(documentFromEditor(value, common.NoteDocument{}))
	} else {
		parsed := documentFromEditor(value, woxcomponent.NoteSegmentDocument(previous, segment))
		c.document = woxcomponent.ReplaceNoteSegment(previous, segment, parsed.Blocks)
	}
	projected, runs, ranges := c.projectActiveText()
	c.richRuns, c.blockRanges = runs, ranges
	// SetText always moves the caret to the end. Keep the live editor selection
	// when the projection already matches, and restore it after a rewrite.
	if segment.Start == c.activeTextSegment.Start && projected != value {
		state := c.editor.State()
		delta := utf8.RuneCountInString(value) - utf8.RuneCountInString(projected)
		c.editor.SetText(projected, false)
		c.editor.SetSelection(max(0, state.Selection.Anchor-delta), max(0, state.Selection.Focus-delta))
	}
	c.selection = c.editor.State().Selection
	c.dirty, c.errorText = true, ""
	c.scheduleSave()
	c.invalidate()
}

func (c *notesWindowController) projectActiveText() (string, []woxcomponent.NoteTextRun, []noteBlockRange) {
	// Loaded, pasted, or edited documents can end in a table without a field for typing below it.
	if count := len(c.document.Blocks); count > 0 && c.document.Blocks[count-1].IsStructural() {
		c.document.Blocks = append(c.document.Blocks, common.NoteBlock{ID: newID(), Type: common.NoteBlockParagraph})
	}
	c.activeTextSegment = c.resolveActiveTextSegment()
	return woxcomponent.ProjectNoteSegment(c.document, c.activeTextSegment, c.editorStyle(), c.app.palette.componentTheme())
}

// resolveActiveTextSegment keeps the caret in the text run the user last entered.
func (c *notesWindowController) resolveActiveTextSegment() woxcomponent.NoteDocumentSegment {
	if c.activeTextSegment.End > c.activeTextSegment.Start && !c.activeTextSegment.Structural() {
		resolved := woxcomponent.NoteSegmentAtBlock(c.document, c.activeTextSegment.Start)
		if !resolved.Structural() {
			return resolved
		}
	}
	return c.preferredTextSegment()
}

func (c *notesWindowController) firstTextSegment() woxcomponent.NoteDocumentSegment {
	for _, segment := range woxcomponent.NoteDocumentSegments(c.document) {
		if !segment.Structural() {
			return segment
		}
	}
	return woxcomponent.NoteDocumentSegment{Start: 0, End: len(c.document.Blocks)}
}

func (c *notesWindowController) lastTextSegment() woxcomponent.NoteDocumentSegment {
	found := c.firstTextSegment()
	for _, segment := range woxcomponent.NoteDocumentSegments(c.document) {
		if !segment.Structural() {
			found = segment
		}
	}
	return found
}

// preferredTextSegment puts the caret below a leading image so the large empty editor area is editable.
func (c *notesWindowController) preferredTextSegment() woxcomponent.NoteDocumentSegment {
	first := c.firstTextSegment()
	if first.End < len(c.document.Blocks) && c.document.Blocks[first.End].Type == common.NoteBlockImage && woxcomponent.NoteSegmentIsEmpty(c.document, first) {
		return c.lastTextSegment()
	}
	return first
}

// viewModeMenuLabel names the Markdown/preview toggle with its platform shortcut.
func (c *notesWindowController) viewModeMenuLabel() string {
	key := "i18n:notes_view_markdown"
	if c.markdownView {
		key = "i18n:notes_view_preview"
	}
	return c.app.translate(key) + " (" + strings.Join(formatHotkeyLabels(primaryHotkey("e")), "+") + ")"
}

// toggleMarkdownView switches the note between the rich preview and raw Markdown source.
func (c *notesWindowController) toggleMarkdownView() {
	selection := c.editor.State().Selection
	source := c.editor.Text()
	sourceRuns := woxcomponent.NoteFieldRuns(c.richRuns)
	if c.markdownView {
		c.applyMarkdownToDocument()
		c.markdownView = false
		mapped := notesplugin.MapMarkdown(c.document)
		anchor := alignMarkdownOffset(source, mapped.Markdown, selection.Anchor)
		focus := alignMarkdownOffset(source, mapped.Markdown, selection.Focus)
		block, imageBlock, tableBlock, textOff := noteMarkdownCaretTarget(c.document, mapped, focus)
		anchorBlock, _, _, textOffAnchor := noteMarkdownCaretTarget(c.document, mapped, anchor)
		c.focusedImageBlock = imageBlock
		c.focusedTableBlock = tableBlock
		c.activeTextSegment = noteSegmentForCaretBlock(c.document, block)
		value, runs, ranges := c.projectActiveText()
		c.richRuns, c.blockRanges = runs, ranges
		c.editor.SetText(value, false)
		next := woxui.TextSelection{
			Anchor: notePreviewOffsetForBlock(c.blockRanges, anchorBlock, textOffAnchor),
			Focus:  notePreviewOffsetForBlock(c.blockRanges, block, textOff),
		}
		if visual, ok := c.noteVisualCaret(source, selection.Focus, sourceRuns, value, woxcomponent.NoteFieldRuns(runs)); ok {
			next = woxui.TextSelection{Anchor: visual, Focus: visual}
		}
		c.setEditorSelection(next, utf8.RuneCountInString(value))
	} else {
		mapped := notesplugin.MapMarkdown(c.document)
		next := notePreviewSelectionToMarkdown(c.document, c.blockRanges, selection, c.focusedImageBlock, c.focusedTableBlock, mapped)
		c.markdownView = true
		c.focusedTableBlock = -1
		c.focusedImageBlock = -1
		c.editor.SetText(mapped.Markdown, false)
		if visual, ok := c.noteVisualCaret(source, selection.Focus, sourceRuns, mapped.Markdown, nil); ok {
			next = woxui.TextSelection{Anchor: visual, Focus: visual}
		}
		c.setEditorSelection(next, utf8.RuneCountInString(mapped.Markdown))
	}
	if c.documentSelected {
		c.editor.SelectAll()
		c.selection = c.editor.State().Selection
		if c.markdownView {
			c.documentSelected = false
		}
	}
	c.revealEditorCaret = true
	if c.editorFocus != nil {
		c.editorFocus.RequestFocus()
	}
	if c.host != nil {
		c.host.RevealCaret()
	}
	c.invalidate()
}

// applyMarkdownToDocument parses the raw source editor into the durable document.
func (c *notesWindowController) applyMarkdownToDocument() {
	if !c.markdownView {
		return
	}
	c.document = notesplugin.NormalizeDocument(notesplugin.ParseMarkdown(c.editor.Text()))
	notesplugin.HydrateNoteImageDimensions(c.document)
}

// consumeEditorCaretReveal reports a one-shot request to scroll the caret on screen after a view toggle.
func (c *notesWindowController) consumeEditorCaretReveal() bool {
	reveal := c.revealEditorCaret
	c.revealEditorCaret = false
	return reveal
}

// noteMarkdownCaretVisible is the source-editor scroller interval that contains the caret.
func (c *notesWindowController) noteMarkdownCaretVisible(width, lineHeight float32, padding woxwidget.Insets) *woxwidget.ScrollRange {
	if !c.consumeEditorCaretReveal() || lineHeight <= 0 {
		return nil
	}
	innerWidth := max(float32(0), width-padding.Left-padding.Right)
	line := woxcomponent.TextFieldVisualLineIndex(c.editor.Text(), c.editor.State().Selection.Focus, c.noteMeasureWindow(), c.editorStyle(), innerWidth, nil)
	top := padding.Top + float32(line)*lineHeight
	return &woxwidget.ScrollRange{Start: top, End: top + lineHeight}
}

// buildMarkdownEditor hosts the raw Markdown source instead of the rich preview.
func (c *notesWindowController) buildMarkdownEditor(width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	padding := notesEditorPadding()
	// Source and preview share the UI font so toggling views does not change typeface.
	style := c.editorStyle()
	lineHeight := 24 * c.zoom
	fieldHeight := max(height, woxcomponent.TextFieldVisualHeight(c.editor.Text(), c.managed.Window(), style, width, lineHeight, padding, nil))
	return woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "notes.editor.scroll", AutomationID: "notes.editor.scroll", Label: c.app.translate("i18n:notes_editor"),
		Width: width, Height: height, KeepVisible: c.noteMarkdownCaretVisible(width, lineHeight, padding),
		Content: woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: "notes.editor.markdown", Label: c.app.translate("i18n:notes_editor"), Width: width, Height: fieldHeight,
			Padding: padding, Transparent: true, DisableHover: true,
			Style: style, LineHeight: lineHeight,
			TextAlignmentY: 0.5, TextColor: theme.PreviewText, Value: c.editor.Text(), Controller: c.editor,
			FocusNode: c.editorFocus, Focused: c.editorFocus.HasFocus(), Autofocus: true,
			ReadOnly: c.record.DeletedAt > 0, MaxLines: 10000, Window: c.managed.Window(), Theme: theme,
			OnChanged: c.onMarkdownChanged, OnKey: c.onKey, OnUndo: c.undoDocument, OnRedo: c.redoDocument,
			OnPaste: c.pasteFromClipboard,
		}),
		Theme: theme, ThumbColor: theme.ResultSubtitle,
	})
}

// onMarkdownChanged keeps the durable document in sync while the source editor is open.
func (c *notesWindowController) onMarkdownChanged(value string) {
	c.rememberDocumentUndo(c.document, time.Since(c.lastTextEdit) < 750*time.Millisecond)
	c.lastTextEdit = time.Now()
	c.document = notesplugin.NormalizeDocument(notesplugin.ParseMarkdown(value))
	c.dirty, c.errorText = true, ""
	c.scheduleSave()
	c.invalidate()
}

// focusImage selects one attachment so its action bar can appear.
func (c *notesWindowController) focusImage(block int) {
	c.documentSelected = false
	c.focusedImageBlock = block
	c.focusedTableBlock = -1
	c.invalidate()
}

// focusTextBesideImage puts the caret in the paragraph before or after the selected image.
func (c *notesWindowController) focusTextBesideImage(block int, after bool) {
	id := ""
	if block >= 0 && block < len(c.document.Blocks) {
		id = c.document.Blocks[block].ID
	}
	c.document = notesplugin.EnsureNoteImageEditGaps(c.document)
	if id != "" {
		for index, item := range c.document.Blocks {
			if item.ID == id {
				block = index
				break
			}
		}
	}
	target := block
	if after {
		target = min(block+1, len(c.document.Blocks)-1)
	} else if block > 0 {
		target = block - 1
	} else {
		target = 0
	}
	c.bindActiveText(target, !after)
	// The shared FocusNode still belongs to the previous segment until the next build.
	// Let the target field request focus after it has attached that node.
	c.requestTextFocus = true
}

// focusNoteText moves editing into a text run after the user taps it.
func (c *notesWindowController) focusNoteText(segmentStart int) {
	if c.focusedImageBlock < 0 && c.focusedTableBlock < 0 && c.activeTextSegment.Start == segmentStart && (c.editorFocus == nil || c.editorFocus.HasFocus()) {
		return
	}
	c.bindActiveText(segmentStart, true)
}

// bindActiveText projects a text run without requesting focus through the previous segment's attachment.
func (c *notesWindowController) bindActiveText(segmentStart int, caretAtEnd bool) {
	c.documentSelected = false
	c.focusedImageBlock = -1
	c.focusedTableBlock = -1
	c.activeTextSegment = woxcomponent.NoteSegmentAtBlock(c.document, segmentStart)
	if c.activeTextSegment.Structural() {
		c.activeTextSegment = c.preferredTextSegment()
	}
	value, runs, ranges := woxcomponent.ProjectNoteSegment(c.document, c.activeTextSegment, c.editorStyle(), c.app.palette.componentTheme())
	c.richRuns, c.blockRanges = runs, ranges
	c.editor.SetText(value, false)
	if caretAtEnd {
		end := utf8.RuneCountInString(value)
		c.editor.SetSelection(end, end)
	} else {
		c.editor.SetSelection(0, 0)
	}
	c.selection = c.editor.State().Selection
	c.invalidate()
}

// scaleImage changes display width while preserving the image's aspect ratio.
func (c *notesWindowController) scaleImage(block, delta int) {
	if block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Image == nil {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	updated := woxcomponent.CloneNoteDocument(c.document)
	updated.Blocks[block].Image.Scale = notesplugin.AdjustNoteImageScale(updated.Blocks[block].Image.Scale, delta)
	c.document = updated
	c.focusedImageBlock = block
	c.dirty, c.errorText = true, ""
	c.scheduleSave()
	c.invalidate()
}

// deleteImage removes the focused attachment block from the document.
func (c *notesWindowController) deleteImage(block int) {
	if block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Type != common.NoteBlockImage {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.DeleteNoteImage(c.document, block)
	c.focusedImageBlock = -1
	c.reproject(false)
}

// resolveNoteImage loads a note attachment or remote picture at preview resolution.
func (c *notesWindowController) resolveNoteImage(image common.NoteImage) *woxui.Image {
	if c.app == nil {
		return nil
	}
	width := c.lastFrame.Width
	if width <= 0 {
		width = notesDefaultWidth
	}
	height := woxcomponent.NoteEditorImageMaxHeight * max(c.zoom, 1)
	size := previewImageRequestSize(width, height)
	if remote := notesplugin.NoteImageRemoteURL(image); remote != "" {
		return c.app.imageForSize(woxImage{ImageType: "url", ImageData: remote}, size)
	}
	path := notesplugin.ResolveNoteImagePath(image)
	if path == "" {
		return nil
	}
	return c.app.imageForSize(woxImage{ImageType: "absolute", ImageData: path}, size)
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
	c.applyMarkdownToDocument()
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
	c.applyMarkdownToDocument()
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
	var editor woxwidget.Widget
	if c.markdownView {
		editor = c.buildMarkdownEditor(frame.Size.Width, editorHeight, theme)
	} else {
		editor = woxcomponent.WoxNoteEditor(woxcomponent.NoteEditorProps{
			ID: "notes.editor", Label: a.translate("i18n:notes_editor"), Document: c.document,
			Width: frame.Size.Width, Height: editorHeight, Padding: notesEditorPadding(),
			Style: c.editorStyle(), LineHeight: 24 * c.zoom, Zoom: c.zoom, TextColor: theme.PreviewText, Theme: theme,
			Window: c.managed.Window(), ReadOnly: c.record.DeletedAt > 0, Autofocus: true, Controller: c.editor,
			FocusNode: c.editorFocus, Focused: (c.editorFocus.HasFocus() || c.requestTextFocus) && c.focusedTableBlock < 0 && c.focusedImageBlock < 0, Selection: c.selection,
			RevealCaret:        c.consumeEditorCaretReveal(),
			ActiveSegmentStart: c.activeTextSegment.Start, OnTextFocus: c.focusNoteText,
			OnChanged: c.onSegmentChanged, OnSelectionChanged: c.onEditorSelectionChanged,
			OnSelectAll: c.selectEntireDocument, OnCopy: c.copyDocumentSelection, OnCut: c.cutDocumentSelection,
			DocumentSelected: c.documentSelected,
			OnTapOffset:      c.handleBlockTap, OnTapBelowText: c.appendParagraphBelowText, CursorAtOffset: c.editorCursorAt, OnKey: c.onKey,
			OnUndo: c.undoDocument, OnRedo: c.redoDocument, OnPaste: c.pasteFromClipboard,
			TransformPaste: func(value string) string {
				converted := notesplugin.ToMarkdown(notesplugin.ParseMarkdown(value))
				if strings.TrimSpace(converted) == "" && strings.TrimSpace(value) != "" {
					return value
				}
				return converted
			},
			OnTableChange: c.replaceTable, OnTableFocus: c.focusTableCell, OnTableKey: c.onTableKey, OnTablePaste: c.pasteTableClipboard,
			OnDeleteEmptySegment: c.deleteEmptyTextSegment,
			OnTableInsertRow:     c.tableInsertRow, OnTableInsertColumn: c.tableInsertColumn, OnTableDeleteRow: c.tableDeleteRow,
			OnTableDeleteColumn: c.tableDeleteColumn, OnTableDelete: c.tableDelete, OnTableActionHover: c.updateTableActionTooltip,
			TableActionLabels: woxcomponent.NoteTableActionLabels{
				InsertRow: c.app.translate("i18n:notes_table_insert_row"), InsertColumn: c.app.translate("i18n:notes_table_insert_column"),
				DeleteRow: c.app.translate("i18n:notes_table_delete_row"), DeleteColumn: c.app.translate("i18n:notes_table_delete_column"),
				DeleteTable: c.app.translate("i18n:notes_table_delete"),
			},
			FocusedTableBlock: c.focusedTableBlock, FocusedTableRow: c.focusedTableRow, FocusedTableCol: c.focusedTableCol,
			ResolveImage: c.resolveNoteImage, MissingImageLabel: a.translate("i18n:notes_image_missing"),
			FocusedImageBlock: c.focusedImageBlock, OnImageFocus: c.focusImage, OnImageLeave: c.focusTextBesideImage, OnImageScale: c.scaleImage, OnImageDelete: c.deleteImage,
			OnImageActionHover: c.updateTableActionTooltip,
			ImageActionLabels: woxcomponent.NoteImageActionLabels{
				Smaller: c.app.translate("i18n:notes_image_smaller"), Larger: c.app.translate("i18n:notes_image_larger"),
				Delete: c.app.translate("i18n:notes_image_delete"),
			},
			ReorderTaskLabel: a.translate("i18n:notes_reorder_task"), ReorderingTask: c.taskReorderFrom, HoveredTask: c.taskHover, ReorderDest: c.taskReorderTo,
			OnHoverTask: c.hoverTask, OnReorderTaskStart: c.startTaskReorder, OnReorderTaskDrag: c.dragTaskReorder, OnReorderTaskEnd: c.endTaskReorder,
		})
		c.requestTextFocus = false
	}
	var overlay woxwidget.Widget
	if c.linkOpen {
		overlay = c.buildLinkOverlay(frame.Size, theme)
	} else if c.searchOpen {
		overlay = c.buildSearchOverlay(frame.Size, theme)
	} else if c.moreOpen {
		overlay = c.buildMoreOverlay(frame.Size, theme)
	}
	overlay = c.composeLinuxInlineTooltip(frame.Size, theme, overlay)
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
	if custom := notesplugin.CustomNoteTitle(c.document); custom != "" {
		title = custom
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
	titleLeft, titleRight, titleAlignment := notesTitleSlot(runtime.GOOS, contentRight)
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
		{Left: titleLeft, Right: titleRight, StretchWidth: true, Child: woxwidget.Align{Height: launcherview.NotesToolbarHeight, Horizontal: titleAlignment, Vertical: .5, Child: woxwidget.TextBlock{
			Value: title, MaxLines: 1, ShrinkWrap: true, AlignmentY: 0.5,
			Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ToolbarText,
		}}},
		{Right: contentRight + 6, AnchorRight: true, Top: 4, Child: right},
	}
	if runtime.GOOS != "darwin" {
		children = append(children, woxwidget.StackChild{Left: 12, Child: woxwidget.Align{Width: 20, Height: launcherview.NotesToolbarHeight, Vertical: .5, Child: woxwidget.Image{Source: notesTitleBarIcon, Width: 20, Height: 20}}})
	}
	children = append(children, woxwidget.StackChild{Child: woxcomponent.WindowCloseChrome(woxcomponent.WindowCloseChromeProps{
		ID: "notes.toolbar.close", Width: width, Platform: runtime.GOOS, Theme: theme, Active: active, Maximized: c.windowMaximized,
		OnMinimize: c.minimizeWindow, OnMaximize: c.toggleMaximize, OnClose: c.requestClose,
	})})
	content := woxwidget.Stack{Width: width, Height: launcherview.NotesToolbarHeight, Children: children}
	return woxwidget.Container{Width: width, Height: launcherview.NotesToolbarHeight, Child: content}
}

// notesEditorPadding insets the rich and Markdown editors from the window frame.
func notesEditorPadding() woxwidget.Insets {
	return woxwidget.Insets{Left: notesEditorPaddingLeft, Top: notesEditorPaddingTop, Right: notesEditorPaddingRight, Bottom: notesEditorPaddingBottom}
}

// notesTitleSlot places the window title like other Wox chrome: centered on
// macOS, left-aligned after the app icon on Windows and Linux.
func notesTitleSlot(goos string, contentRight float32) (left, right, alignment float32) {
	if goos == "darwin" {
		return notesToolbarActionsWidth, notesToolbarActionsWidth, 0.5
	}
	return 40, contentRight + notesToolbarActionsWidth, 0
}

// updateToolbarTooltip keeps Notes chrome hints in the same hover path as settings.
// Linux cannot position a native overlay window, so the owner window paints the
// same in-window tooltip settings already uses. Other platforms keep the native overlay.
func (c *notesWindowController) updateToolbarTooltip(inside bool, text string, anchor woxui.Rect) {
	if util.IsLinux() {
		c.app.scheduleLinuxInlineTooltip(linuxInlineTooltipTarget{
			revision: &c.tooltipRev, state: &c.inlineTooltip,
			open: true, invalidate: c.invalidate, job: "show Notes inline tooltip",
		}, inside, text, anchor, "top")
		return
	}
	c.app.setNativeHoverTooltip(&c.tooltipRev, "go-ui-notes-titlebar", "update Notes chrome tooltip", inside, text, anchor, "top", func() *woxui.Window {
		if c.managed == nil {
			return nil
		}
		return c.managed.Window()
	})
}

// composeLinuxInlineTooltip stacks the in-window Linux tooltip above search, link, or more overlays.
func (c *notesWindowController) composeLinuxInlineTooltip(size woxui.Size, theme woxcomponent.Theme, overlay woxwidget.Widget) woxwidget.Widget {
	if c.inlineTooltip == nil {
		return overlay
	}
	tooltip, left, top := launcherview.SettingsInlineTooltipOverlay(launcherview.SettingsInlineTooltipProps{
		Width: size.Width, Height: size.Height, Anchor: c.inlineTooltip.Anchor, Message: c.inlineTooltip.Text, Side: c.inlineTooltip.Side, Theme: theme,
	})
	if tooltip == nil {
		return overlay
	}
	layer := woxwidget.StackChild{Left: left, Top: top, Child: tooltip}
	if overlay == nil {
		return woxwidget.Stack{Width: size.Width, Height: size.Height, Children: []woxwidget.StackChild{layer}}
	}
	return woxwidget.Stack{Width: size.Width, Height: size.Height, Children: []woxwidget.StackChild{
		{Child: overlay},
		layer,
	}}
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
	stats := c.noteCharacterCountLabel()
	row := []woxwidget.Widget{
		woxwidget.Semantics{
			Key: "notes.format.stats", AutomationID: "notes.format.stats", Role: woxui.AccessibilityRoleText, Label: stats,
			Child: woxwidget.Text{Value: stats, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle},
		},
	}
	if !c.markdownView {
		row = append(row, woxwidget.Expanded{Child: woxwidget.Align{
			Height: launcherview.NotesFormatBarHeight, Horizontal: 1, Vertical: .5,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 2, Children: items},
		}})
	}
	return woxwidget.Container{Width: width, Height: launcherview.NotesFormatBarHeight, BorderColor: theme.PreviewSplit, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 12, Right: 12},
		Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: row}}
}

// noteCharacterCountLabel is the live document length shown on the format bar.
func (c *notesWindowController) noteCharacterCountLabel() string {
	return fmt.Sprintf(c.app.translate("i18n:notes_character_count"), notesplugin.DocumentCharacterCount(c.document))
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
	return woxwidget.Container{Width: width, Height: launcherview.NotesStatusHeight, Padding: woxwidget.Insets{Left: 12, Right: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}}
}

func (c *notesWindowController) buildSearchOverlay(size woxui.Size, theme woxcomponent.Theme) woxwidget.Widget {
	width := max(float32(260), size.Width-24)
	// Keep the panel below the title-bar inset so the overlay flex stays inside the window.
	height := min(float32(300), max(float32(0), size.Height-notesSearchOverlayTop))
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
		Content: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: notesSearchListGap, Children: rows},
		Theme:   theme, ThumbColor: theme.ResultSubtitle,
	})
	panel := woxwidget.Container{Width: width, Height: height, Radius: 10, Floating: true, Color: theme.ActionBackground, BorderColor: theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.UniformInsets(notesSearchOverlayPadding), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: notesSearchOverlayGap, Children: []woxwidget.Widget{
		search, results,
	}}}
	return c.overlayScrim(size, panel, 12, notesSearchOverlayTop, func() { c.searchOpen = false; c.invalidate() })
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
		return c.app.translate("i18n:notes_favorites")
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
	c.applyMarkdownToDocument()
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
		c.searchOpen = false
		c.invalidate()
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
	width := float32(240)
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
		c.menuRow("view", c.viewModeMenuLabel(), width, theme, c.toggleMarkdownView),
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
	panel := woxwidget.Container{Width: width, Height: panelHeight, Radius: 9, Floating: true, Color: theme.ActionBackground, BorderColor: theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.UniformInsets(6), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
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
	panel := woxwidget.Container{Width: width, Height: 100, Radius: 10, Floating: true, Color: theme.ActionBackground, BorderColor: theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.UniformInsets(10), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{field, buttons}}}
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
		if c.focusedImageBlock >= 0 {
			c.focusTextBesideImage(c.focusedImageBlock, true)
			return true
		}
		c.requestClose()
		return true
	}
	if c.documentSelected && (event.Key == woxui.KeyBackspace || event.Key == woxui.KeyDelete) && event.Modifiers == 0 {
		return c.clearDocumentSelection()
	}
	if c.focusedImageBlock >= 0 && event.Modifiers == 0 {
		switch event.Key {
		case woxui.KeyBackspace, woxui.KeyDelete:
			c.deleteImage(c.focusedImageBlock)
			return true
		case woxui.KeyArrowLeft, woxui.KeyArrowUp:
			c.focusTextBesideImage(c.focusedImageBlock, false)
			return true
		case woxui.KeyArrowRight, woxui.KeyArrowDown, woxui.KeyEnter:
			c.focusTextBesideImage(c.focusedImageBlock, true)
			return true
		}
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
	if event.Modifiers&woxui.KeyModifierAlt != 0 && event.Modifiers&^woxui.KeyModifierAlt == 0 {
		switch event.Key {
		case woxui.KeyArrowUp:
			return c.nudgeTaskBlock(-1)
		case woxui.KeyArrowDown:
			return c.nudgeTaskBlock(1)
		}
	}
	if !event.Modifiers.HasPrimary() {
		return false
	}
	switch event.Key {
	case woxui.Key("a"):
		return c.selectEntireDocument()
	case woxui.Key("c"):
		return c.copyDocumentSelection()
	case woxui.Key("x"):
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			c.toggleInline("strike")
			return true
		}
		return c.cutDocumentSelection()
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
	case woxui.Key("e"):
		c.toggleMarkdownView()
	case woxui.Key("k"):
		c.openLink()
	case woxui.Key("v"):
		return c.pasteFromClipboard("")
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

// onEditorSelectionChanged stores the live caret and drops a whole-note selection when it shrinks.
func (c *notesWindowController) onEditorSelectionChanged(selection woxui.TextSelection) {
	if selection == c.selection {
		return
	}
	c.selection = selection
	c.focusedTableBlock = -1
	c.focusedImageBlock = -1
	if c.documentSelected && !c.editorHasFullSelection() {
		c.documentSelected = false
	}
	c.invalidate()
}

// selectEntireDocument selects every block so Ctrl+A is not stopped by an image or table.
func (c *notesWindowController) selectEntireDocument() bool {
	if c.markdownView || !noteDocumentHasStructuralBlocks(c.document) {
		return false
	}
	c.documentSelected = true
	c.focusedImageBlock = -1
	c.focusedTableBlock = -1
	if c.editorFocus != nil {
		c.editorFocus.RequestFocus()
	}
	c.editor.SelectAll()
	c.selection = c.editor.State().Selection
	c.invalidate()
	return true
}

// copyDocumentSelection writes the full note Markdown when select-all crossed structural blocks.
func (c *notesWindowController) copyDocumentSelection() bool {
	if !c.documentSelected {
		return false
	}
	if err := clipboard.WriteText(notesplugin.ToMarkdown(c.document)); err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("note copy: %v", err))
	}
	return true
}

// cutDocumentSelection copies the full note and replaces it with an empty document.
func (c *notesWindowController) cutDocumentSelection() bool {
	if !c.documentSelected {
		return false
	}
	c.copyDocumentSelection()
	return c.clearDocumentSelection()
}

// clearDocumentSelection deletes a whole-note selection, including images and tables.
func (c *notesWindowController) clearDocumentSelection() bool {
	if !c.documentSelected || c.record.DeletedAt > 0 {
		return false
	}
	return c.replaceSelectedDocument(notesplugin.EmptyDocument())
}

// replaceSelectedDocument swaps the current note for pasted or cleared content.
func (c *notesWindowController) replaceSelectedDocument(next common.NoteDocument) bool {
	c.rememberDocumentUndo(c.document, false)
	c.documentSelected = false
	c.focusedImageBlock = -1
	c.focusedTableBlock = -1
	c.document = notesplugin.EnsureNoteImageEditGaps(notesplugin.NormalizeDocument(next))
	notesplugin.HydrateNoteImageDimensions(c.document)
	c.activeTextSegment = woxcomponent.NoteDocumentSegment{}
	if c.markdownView {
		c.editor.SetText(notesplugin.ToMarkdown(c.document), false)
		c.selection = c.editor.State().Selection
		c.dirty, c.errorText = true, ""
		c.scheduleSave()
		c.invalidate()
		return true
	}
	c.reproject(true)
	return true
}

func (c *notesWindowController) editorHasFullSelection() bool {
	state := c.editor.State()
	limit := utf8.RuneCountInString(state.Text)
	if limit == 0 {
		return true
	}
	return !state.Selection.Collapsed() && state.Selection.Start() == 0 && state.Selection.End() == limit
}

func noteDocumentHasStructuralBlocks(document common.NoteDocument) bool {
	for _, block := range document.Blocks {
		if block.IsStructural() {
			return true
		}
	}
	return false
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
	// Document block numbers are not slice indexes after an image or table splits the editor.
	c.editor.SetCaret(woxcomponent.NoteRangeForBlock(c.blockRanges, block).TextStart)
	c.selection = c.editor.State().Selection
	c.invalidate()
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
	oldTextStart := woxcomponent.NoteRangeForBlock(c.blockRanges, block).TextStart
	c.rememberDocumentUndo(c.document, false)
	c.document = document
	c.reproject(false)
	shift := woxcomponent.NoteRangeForBlock(c.blockRanges, block).TextStart - oldTextStart
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

// appendParagraphBelowText lets blank-space clicks leave a formatted block without changing Enter's continuation rules.
func (c *notesWindowController) appendParagraphBelowText() bool {
	if c.record.DeletedAt > 0 || len(c.document.Blocks) == 0 {
		return false
	}
	if last := c.document.Blocks[len(c.document.Blocks)-1]; last.Type == common.NoteBlockParagraph && last.Text == "" {
		return false
	}
	c.rememberDocumentUndo(c.document, false)
	c.document.Blocks = append(c.document.Blocks, common.NoteBlock{ID: newID(), Type: common.NoteBlockParagraph})
	c.reproject(false)
	c.editor.SetCaret(utf8.RuneCountInString(c.editor.Text()))
	c.selection = c.editor.State().Selection
	return true
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

// hoverTask reveals the left-gutter handle for the checklist item under the pointer.
func (c *notesWindowController) hoverTask(block int) {
	if c.taskHover == block {
		return
	}
	c.taskHover = block
	c.invalidate()
}

// startTaskReorder begins a checklist drag without rewriting the document until drop.
func (c *notesWindowController) startTaskReorder(block int) {
	if c.record.DeletedAt > 0 || block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Type != common.NoteBlockTask {
		return
	}
	c.taskReorderFrom, c.taskReorderTo = block, block
}

// dragTaskReorder records the insert-before index under the pointer so the editor can preview it.
func (c *notesWindowController) dragTaskReorder(block int, contentY float32) {
	if c.taskReorderFrom < 0 {
		c.startTaskReorder(block)
	}
	if c.taskReorderFrom < 0 {
		return
	}
	dest := c.taskReorderDest(contentY)
	if dest == c.taskReorderTo {
		return
	}
	c.taskReorderTo = dest
	c.invalidate()
}

// taskReorderDest maps the pointer's document Y onto an insert-before index.
func (c *notesWindowController) taskReorderDest(contentY float32) int {
	lineHeight := float32(24) * c.zoom
	if lineHeight <= 0 {
		lineHeight = 24
	}
	innerWidth := c.lastFrame.Width - 32
	if innerWidth <= 0 {
		innerWidth = 400
	}
	var window *woxui.Window
	if c.managed != nil {
		window = c.managed.Window()
	}
	return woxcomponent.NoteTaskLiveDest(c.document, c.blockRanges, c.taskReorderFrom, contentY, lineHeight, innerWidth, c.editor.Text(), c.richRuns, c.editorStyle(), window)
}

// endTaskReorder commits the pending checklist move.
func (c *notesWindowController) endTaskReorder() {
	from, dest := c.taskReorderFrom, c.taskReorderTo
	c.taskReorderFrom, c.taskReorderTo = -1, -1
	if from < 0 {
		return
	}
	c.applyTaskMove(from, dest)
}

// nudgeTaskBlock moves the caret's checklist item one slot with Alt+Arrow.
func (c *notesWindowController) nudgeTaskBlock(delta int) bool {
	index := woxcomponent.NoteTaskAtCaret(c.document, c.blockRanges, c.selection)
	if index < 0 || c.record.DeletedAt > 0 {
		return false
	}
	dest := woxcomponent.NoteTaskNudgeDest(c.document, index, delta)
	return c.applyTaskMove(index, dest)
}

// applyTaskMove writes one checklist reorder through the shared undo and save path.
func (c *notesWindowController) applyTaskMove(from, dest int) bool {
	updated, next := woxcomponent.MoveNoteTaskGroup(c.document, from, dest)
	if next == from && noteDocumentsEqual(updated, c.document) {
		c.invalidate()
		return false
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = updated
	c.reproject(false)
	c.focusNoteBlock(next)
	return true
}

// focusNoteBlock puts the caret on the moved task so the handle stays on that item.
func (c *notesWindowController) focusNoteBlock(index int) {
	for _, blockRange := range c.blockRanges {
		if blockRange.Block == index {
			c.editor.SetCaret(blockRange.TextStart)
			c.selection = c.editor.State().Selection
			return
		}
	}
}

func noteDocumentsEqual(left, right common.NoteDocument) bool {
	if len(left.Blocks) != len(right.Blocks) {
		return false
	}
	for index := range left.Blocks {
		if left.Blocks[index].ID != right.Blocks[index].ID {
			return false
		}
	}
	return true
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

// pasteFromClipboard inserts files, a clipboard bitmap, or structured text at the caret.
func (c *notesWindowController) pasteFromClipboard(value string) bool {
	if c.record.DeletedAt > 0 {
		return true
	}
	if strings.TrimSpace(value) == "" {
		if text, err := clipboard.ReadText(); err == nil {
			value = text
		}
	}
	img, imgErr := clipboard.ReadImage()
	if imgErr != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("note paste: read image: %v", imgErr))
	}
	if paths, err := clipboard.ReadFilePaths(); err == nil && len(paths) > 0 {
		if pasted := notesplugin.DocumentFromClipboardFiles(paths); noteDocumentHasImage(pasted) || img == nil {
			if len(pasted.Blocks) > 0 {
				return c.insertPastedDocument(pasted)
			}
		}
	}
	if img != nil && !noteClipboardTextOutranksImage(value, img) {
		return c.pasteImportedImage(img, "clipboard.png")
	}
	if strings.TrimSpace(value) != "" {
		return c.pasteDocument(value)
	}
	return false
}

// noteClipboardTextOutranksImage keeps Word/browser prose instead of their DIB preview.
// A lone URL or filename next to a real bitmap is the usual "copy image" side-channel.
func noteClipboardTextOutranksImage(value string, img image.Image) bool {
	value = strings.TrimSpace(value)
	if value == "" || img == nil {
		return false
	}
	if strings.ContainsAny(value, " \t\n\r") {
		return true
	}
	bounds := img.Bounds()
	return bounds.Dx() < 64 || bounds.Dy() < 64
}

func noteDocumentHasImage(document common.NoteDocument) bool {
	for _, block := range document.Blocks {
		if block.Type == common.NoteBlockImage {
			return true
		}
	}
	return false
}

// pasteClipboardFiles imports copied image files and turns other paths into links.
func (c *notesWindowController) pasteClipboardFiles(paths []string) bool {
	pasted := notesplugin.DocumentFromClipboardFiles(paths)
	if len(pasted.Blocks) == 0 {
		return false
	}
	return c.insertPastedDocument(pasted)
}

// pasteImportedImage stores a clipboard bitmap as a note attachment and inserts it.
func (c *notesWindowController) pasteImportedImage(img image.Image, fileName string) bool {
	pasted, err := notesplugin.DocumentFromClipboardImage(img, fileName)
	if err != nil {
		c.errorText = c.app.translate("i18n:notes_paste_failed")
		util.GetLogger().Error(context.Background(), fmt.Sprintf("paste note image: %v", err))
		c.invalidate()
		return true
	}
	return c.insertPastedDocument(pasted)
}

func (c *notesWindowController) pasteDocument(value string) bool {
	pasted := notesplugin.ParseClipboard(value)
	if !noteClipboardHasStructure(pasted) {
		return false
	}
	return c.insertPastedDocument(pasted)
}

// insertPastedDocument places structured clipboard blocks at the caret or table focus.
func (c *notesWindowController) insertPastedDocument(pasted common.NoteDocument) bool {
	if len(pasted.Blocks) == 0 {
		return false
	}
	if c.documentSelected {
		return c.replaceSelectedDocument(pasted)
	}
	c.rememberDocumentUndo(c.document, false)
	index := 0
	if c.focusedTableBlock >= 0 {
		index = c.focusedTableBlock
	} else if len(c.blockRanges) > 0 {
		index = noteBlockAt(c.blockRanges, c.selection.Focus)
	}
	insertAt := index + 1
	if index < 0 || index >= len(c.document.Blocks) {
		insertAt = len(c.document.Blocks)
		c.document.Blocks = append(c.document.Blocks, pasted.Blocks...)
	} else if c.document.Blocks[index].Type == common.NoteBlockParagraph && strings.TrimSpace(c.document.Blocks[index].Text) == "" {
		insertAt = index
		c.document.Blocks = slices.Replace(c.document.Blocks, index, index+1, pasted.Blocks...)
	} else {
		c.document.Blocks = slices.Insert(c.document.Blocks, index+1, pasted.Blocks...)
	}
	c.focusedImageBlock = -1
	if pasted.Blocks[0].Type == common.NoteBlockTable {
		c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol = index+1, 0, 0
		if index < len(c.document.Blocks) && c.document.Blocks[index].Type == common.NoteBlockTable && strings.TrimSpace(c.document.Blocks[index].Text) == "" {
			c.focusedTableBlock = index
		}
	} else {
		for offset, block := range pasted.Blocks {
			if block.Type != common.NoteBlockImage {
				continue
			}
			c.focusedTableBlock = -1
			c.focusedImageBlock = insertAt + offset
			break
		}
	}
	if c.markdownView {
		c.editor.SetText(notesplugin.ToMarkdown(c.document), false)
		c.selection = c.editor.State().Selection
		c.dirty, c.errorText = true, ""
		c.scheduleSave()
		c.invalidate()
		return true
	}
	c.reproject(false)
	return true
}

func noteClipboardHasStructure(document common.NoteDocument) bool {
	if len(document.Blocks) != 1 {
		return true
	}
	return document.Blocks[0].Type == common.NoteBlockTable || document.Blocks[0].Type == common.NoteBlockImage
}

func (c *notesWindowController) replaceTable(block int, table common.NoteTable) {
	c.rememberDocumentUndo(c.document, true)
	c.document = woxcomponent.ReplaceNoteTable(c.document, block, &table)
	c.dirty, c.errorText = true, ""
	c.scheduleSave()
	c.invalidate()
}

func (c *notesWindowController) focusTableCell(block, row, column int) {
	c.documentSelected = false
	c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol = block, row, column
	c.focusedImageBlock = -1
	c.invalidate()
}

func (c *notesWindowController) pasteTableClipboard(block, row, column int, value string) bool {
	return c.pasteFromClipboard(value)
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

// noteSegmentTouchesImage reports empty caret gaps that must stay so typing can land beside an image.
func noteSegmentTouchesImage(document common.NoteDocument, segment woxcomponent.NoteDocumentSegment) bool {
	if segment.Start > 0 && segment.Start <= len(document.Blocks) && document.Blocks[segment.Start-1].Type == common.NoteBlockImage {
		return true
	}
	return segment.End < len(document.Blocks) && document.Blocks[segment.End].Type == common.NoteBlockImage
}

// deleteEmptyTextSegment removes a blank paragraph so Backspace can close the gap between tables.
func (c *notesWindowController) deleteEmptyTextSegment(segmentStart int) bool {
	if c.record.DeletedAt > 0 {
		return false
	}
	segment := woxcomponent.NoteSegmentAtBlock(c.document, segmentStart)
	if noteSegmentTouchesImage(c.document, segment) {
		return false
	}
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
	c.undoDocuments = append(c.undoDocuments, noteUndoEntry{
		document:         cloneNoteDocument(document),
		selection:        c.selection,
		segment:          c.activeTextSegment,
		tableBlock:       c.focusedTableBlock,
		imageBlock:       c.focusedImageBlock,
		documentSelected: c.documentSelected,
	})
	c.redoDocuments = nil
}

// liveUndoEntry captures the current document and caret for the opposite history stack.
func (c *notesWindowController) liveUndoEntry() noteUndoEntry {
	return noteUndoEntry{
		document:         cloneNoteDocument(c.document),
		selection:        c.editor.State().Selection,
		segment:          c.activeTextSegment,
		tableBlock:       c.focusedTableBlock,
		imageBlock:       c.focusedImageBlock,
		documentSelected: c.documentSelected,
	}
}

func (c *notesWindowController) undoDocument() bool {
	if len(c.undoDocuments) == 0 || c.record.DeletedAt > 0 {
		return true
	}
	previous := c.undoDocuments[len(c.undoDocuments)-1]
	c.undoDocuments = c.undoDocuments[:len(c.undoDocuments)-1]
	c.redoDocuments = append(c.redoDocuments, c.liveUndoEntry())
	c.applyUndoEntry(previous)
	return true
}

func (c *notesWindowController) redoDocument() bool {
	if len(c.redoDocuments) == 0 || c.record.DeletedAt > 0 {
		return true
	}
	next := c.redoDocuments[len(c.redoDocuments)-1]
	c.redoDocuments = c.redoDocuments[:len(c.redoDocuments)-1]
	c.undoDocuments = append(c.undoDocuments, c.liveUndoEntry())
	c.applyUndoEntry(next)
	return true
}

// applyUndoEntry restores a snapshot and the caret that belonged to that version.
func (c *notesWindowController) applyUndoEntry(entry noteUndoEntry) {
	c.document = entry.document
	c.activeTextSegment = entry.segment
	c.focusedTableBlock = entry.tableBlock
	c.focusedImageBlock = entry.imageBlock
	c.documentSelected = entry.documentSelected
	c.lastTextEdit = time.Time{}
	c.syncEditorAfterDocumentChange(entry.selection)
	if c.documentSelected {
		c.focusedImageBlock = -1
		c.focusedTableBlock = -1
		c.editor.SelectAll()
		c.selection = c.editor.State().Selection
	}
}

// syncEditorAfterDocumentChange refreshes the visible editor after undo or redo.
func (c *notesWindowController) syncEditorAfterDocumentChange(selection woxui.TextSelection) {
	if c.markdownView {
		value := notesplugin.ToMarkdown(c.document)
		c.editor.SetText(value, false)
		c.richRuns, c.blockRanges = nil, nil
		c.setEditorSelection(selection, utf8.RuneCountInString(value))
		c.dirty, c.errorText = true, ""
		c.scheduleSave()
		c.invalidate()
		return
	}
	value, runs, ranges := c.projectActiveText()
	c.richRuns, c.blockRanges = runs, ranges
	c.editor.SetText(value, false)
	c.setEditorSelection(selection, utf8.RuneCountInString(value))
	c.selection = c.editor.State().Selection
	c.dirty, c.errorText = true, ""
	c.scheduleSave()
	c.invalidate()
}

// setEditorSelection clamps a restored caret onto the current editor text.
func (c *notesWindowController) setEditorSelection(selection woxui.TextSelection, limit int) {
	c.editor.SetSelection(min(max(0, selection.Anchor), limit), min(max(0, selection.Focus), limit))
	c.selection = c.editor.State().Selection
}

func (c *notesWindowController) setZoom(value float32) {
	c.zoom = max(float32(.75), min(float32(2), value))
	if !c.markdownView {
		projected, runs, ranges := c.projectActiveText()
		c.richRuns, c.blockRanges = runs, ranges
		if projected != c.editor.Text() {
			selection := c.editor.State().Selection
			c.editor.SetText(projected, false)
			c.editor.SetSelection(selection.Anchor, selection.Focus)
		}
	}
	_ = c.app.services.NotesSetLocal(context.Background(), c.preferenceKey("zoom"), fmt.Sprintf("%.2f", c.zoom))
	c.invalidate()
}

// readWindowPinned loads the per-note always-on-top preference. New notes default to pinned.
func (c *notesWindowController) readWindowPinned() bool {
	if c == nil || c.app == nil || c.app.services == nil || c.record.ID == "" {
		return false
	}
	value := c.localPreference("windowPinned")
	return value == "" || value == "1"
}

// applyWindowTopmost mirrors the persisted pin onto the live native window.
// On macOS, pin also joins every Space; unpin returns the note to the current
// Space and normal app stacking.
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

// toggleWindowPin keeps this note above other applications and on every Space.
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
		if custom := notesplugin.CustomNoteTitle(c.document); custom != "" {
			title = custom
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
