package launcher

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"wox/common"
	"wox/common/icons"
	notesplugin "wox/plugin/system/notes"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// notesWindowRole keeps minimized notes on the taskbar and dock. Utility windows
// are hidden from the shell (WS_EX_TOOLWINDOW / skip-taskbar), so they cannot be restored.
const notesWindowRole = woxui.WindowRoleApplication

const (
	notesDefaultWidth  = float32(460)
	notesDefaultHeight = float32(320)
	notesMinimumWidth  = float32(460)
	notesMinimumHeight = float32(240)
	notesMaximumHeight = float32(640)
	// notesEditorPaddingLeft keeps checklist glyphs and the left-gutter handle
	// clear of the native window resize hit zone. 40 also lines up with the
	// Windows title slot.
	notesEditorPaddingLeft   = float32(40)
	notesEditorPaddingRight  = float32(40)
	notesEditorPaddingTop    = float32(12)
	notesEditorPaddingBottom = float32(24)
	// notesEditorLineHeight is the body line box for the 14-unit note font.
	// 20 keeps wrapped lines from sitting in a loose 24-unit box; paragraph breaks add their own gap.
	notesEditorLineHeight = float32(20)
)

var (
	notesTitleBarIconOnce sync.Once
	notesTitleBarIcon     *woxui.Image
)

// notesWindowController binds editing, persistence, and presentation to one native window.
type notesWindowController struct {
	*notesEditingSession
	*notesAutosave
	owner           *notesController
	windowID        woxui.WindowID
	managed         *woxui.ManagedWindow
	host            *woxwidget.Host
	searchEditor    *woxwidget.TextEditingController
	searchFocus     *woxwidget.FocusNode
	linkEditor      *woxwidget.TextEditingController
	linkFocus       *woxwidget.FocusNode
	summaries       []common.NoteSummary
	searchIndex     int
	searchOpen      bool
	findOpen        bool
	findReveal      bool
	findEditor      *woxwidget.TextEditingController
	findFocus       *woxwidget.FocusNode
	findIndex       int
	moreOpen        bool
	formatMore      bool
	linkOpen        bool
	formatVisible   bool
	saving          bool
	lastFrame       woxui.Size
	requestedSize   woxui.Size
	manualSize      bool
	tooltipRev      atomic.Uint64
	inlineTooltip   *settingsInlineTooltipState
	windowPinned    bool
	windowMaximized bool
	restoreFrame    woxui.Rect
}

// notesWindowIcon is the notes plugin glyph used both in the title bar and on the taskbar.
// It is decoded on first use: the 256 px taskbar raster costs 256 KB and most sessions never
// open the notes window, so decoding it at package init kept that memory resident for nothing.
func notesWindowIcon() *woxui.Image {
	notesTitleBarIconOnce.Do(func() {
		notesTitleBarIcon, _ = decodeWoxImageWithTint(fromCoreImage(icons.Get(icons.PluginNotes)), nil, 256)
	})
	return notesTitleBarIcon
}

// notesNativeMinSize is the live resize floor passed to the native window.
func notesNativeMinSize() woxui.Size {
	return woxui.Size{Width: notesMinimumWidth, Height: notesMinimumHeight}
}

func newNotesWindowController(owner *notesController, record common.NoteRecord) *notesWindowController {
	controller := &notesWindowController{
		owner:               owner,
		notesEditingSession: newNotesEditingSession(),
		searchEditor:        woxwidget.NewTextEditingController(""),
		findEditor:          woxwidget.NewTextEditingController(""), findFocus: woxwidget.NewFocusNode(),
		searchFocus:   woxwidget.NewFocusNode(),
		linkEditor:    woxwidget.NewTextEditingController("https://"),
		linkFocus:     woxwidget.NewFocusNode(),
		windowID:      woxui.WindowID("wox.notes." + newID()),
		formatVisible: true,
	}
	controller.notesAutosave = &notesAutosave{
		dispatch: owner.runOnUI, stopped: owner.isDestroyed,
		save: controller.flush, invalidate: controller.invalidate,
	}
	controller.controlTheme = func() woxcomponent.ControlTheme { return owner.palette().componentTheme().Controls }
	controller.onChanged = controller.scheduleSave
	controller.onInvalidate = controller.invalidate
	controller.mapVisualCaret = controller.noteVisualCaret
	controller.revealCaret = func() {
		if controller.host != nil {
			controller.host.RevealCaret()
		}
	}
	controller.applyRecord(record)
	return controller
}

// activeNotesController returns the last active note window, then falls back to another live note window.
func (a *notesController) activeNotesController() *notesWindowController {
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

func (a *notesController) activateNoteWindow(controller *notesWindowController) {
	if controller == nil {
		return
	}
	a.activeNote = controller
	_ = a.services.NotesSetLocal(context.Background(), "currentNoteId", controller.record.ID)
}

func (a *notesController) removeNoteWindow(controller *notesWindowController) {
	if controller == nil || a.noteWindows[controller.record.ID] != controller {
		return
	}
	delete(a.noteWindows, controller.record.ID)
	if a.activeNote == controller {
		a.activeNote = nil
	}
}

// openNewNoteWindow opens an empty draft and reuses one that the user has not typed into yet.
func (a *notesController) openNewNoteWindow() error {
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
func (a *notesController) openNoteRequest(request common.NotesWindowRequest) error {
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
func (a *notesController) openNoteRecord(record common.NoteRecord, request common.NotesWindowRequest) error {
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
	if c == nil || c.owner == nil {
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
		managed, _, openErr = c.owner.windows.Open(c.windowID, woxui.WindowOptions{
			Title: c.owner.language.translate("i18n:notes_title"), Size: woxui.Size{Width: notesDefaultWidth, Height: notesDefaultHeight},
			MinSize:             notesNativeMinSize(),
			TitleBarControls:    woxui.TitleBarControls{Height: woxcomponent.TitleBarHeight, Close: true, Minimize: true, Maximize: true},
			OnMaximizeRequested: c.toggleMaximize,
			Role:                notesWindowRole, Icon: notesWindowIcon(), Resizable: true, Topmost: c.windowPinned, HideOnBlur: false,
			OnFrame: func(list *woxui.DisplayList, frame woxui.FrameInfo) { c.frame(host, list, frame) }, OnPointer: host.Pointer,
			OnFocus: func(event woxui.FocusEvent) {
				host.SetWindowFocused(event.Active)
				if event.Active {
					c.owner.activateNoteWindow(c)
				}
			},
			OnKey: func(event woxui.KeyEvent) bool {
				if c.onFindKey(event) {
					return true
				}
				return c.owner.dispatchWindowKey(event, host, common.NotesPluginID, c.requestClose, c.onKey)
			},
			OnTextInput:      func(event woxui.TextInputEvent) { host.TextInput(event) },
			OnCloseRequested: c.requestClose,
			OnClosed: func() {
				c.cancelScheduledSave()
				c.discardEmptyNote()
				if !notesplugin.DocumentIsEmpty(c.document) {
					_ = c.flush()
				}
				host.Dispose()
				c.managed, c.host = nil, nil
				c.owner.removeNoteWindow(c)
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
	if err := window.SetAppearance(c.owner.palette().isDark()); err != nil {
		return nil, err
	}
	if err := window.SetFontFamily(c.owner.fontFamily()); err != nil {
		return nil, err
	}
	if err := window.SetTopmost(c.windowPinned); err != nil {
		return nil, err
	}
	c.windowMaximized = c.readWindowMaximized()
	if !c.restoreBounds(window) {
		_ = window.CenterOnMouseScreen(woxui.Size{Width: notesDefaultWidth, Height: notesDefaultHeight})
		if bounds, err := window.Bounds(); err == nil {
			offset := float32((len(c.owner.noteWindows) - 1) % 8 * 24)
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
	c.owner.activateNoteWindow(c)
	c.editorFocus.RequestFocus()
	if request.ExportFormat != "" && request.ExportFormat != "choose" {
		c.export(request.ExportFormat)
	}
	return managed.Window().Invalidate()
}

// applyRecord projects a durable record into this window's editor state.
func (c *notesWindowController) applyRecord(record common.NoteRecord) {
	c.cancelScheduledSave()
	c.notesEditingSession.load(record)
	c.windowPinned = c.readWindowPinned()
	c.applyWindowTopmost()
	c.resizeForDocument()
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

// requestClose avoids re-entering native destruction from a platform close-request callback.
func (c *notesWindowController) requestClose() {
	util.Go(c.owner.lifecycleCtx, "close Notes window", func() {
		if err := c.owner.runOnUI("close Notes window", func() {
			if err := c.close(); err != nil {
				c.fail(err)
			}
		}); err != nil {
			util.GetLogger().Error(context.Background(), fmt.Sprintf("close Notes window: %v", err))
		}
	})
}

func (c *notesWindowController) invalidate() {
	if c.managed != nil {
		_ = c.managed.Window().Invalidate()
	}
}
