package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/util"
	"wox/util/screen"
	"wox/util/ui"

	"github.com/google/uuid"
)

// gpuUIImpl implements common.UI using the native renderer (Direct2D on
// Windows, CoreGraphics on macOS). Launcher-related methods (ShowApp/
// HideApp/ChangeQuery/UpdateResult etc.) are handled directly via the native
// ui package. Settings/onboarding/screenshot methods are forwarded to the
// existing WebSocket-based uiImpl (which talks to the Flutter settings
// process).
type gpuUIImpl struct {
	mu sync.Mutex

	// Native renderer — platform-agnostic interface. On Windows this is
	// backed by *ui.WindowsRenderer (Direct2D/DirectWrite); on macOS by
	// *ui.MacRenderer (CoreGraphics/CoreText). Both satisfy ui.NativeRenderer.
	renderer ui.NativeRenderer
	engine   *ui.LayoutEngine
	theme    ui.Theme

	// Launcher state
	visible          bool
	queryValue       string
	results          []plugin.QueryResultUI
	selectedIdx      int
	scrollOffset     float32
	currentQuery     plugin.Query
	currentSessionId string

	// composeValue holds the in-progress IME composition text. On macOS this is
	// populated by EventIMECompose and displayed alongside queryValue until the
	// input method commits (EventTextInput). Windows never sends
	// EventIMECompose so composeValue stays empty there.
	composeValue string

	// Cursor / selection state for the query box. cursorPos is the byte offset
	// of the caret; selStart/selEnd delimit the selection (-1 means no selection).
	cursorPos     int
	selStart      int
	selEnd        int
	cursorVisible bool
	cursorTimer   *time.Timer

	// dirty is set whenever results, query text, selection, or theme change.
	// buildAndRender only produces draw commands when dirty is true, avoiding
	// unnecessary full-window redraws (which cause the selected item to flicker
	// during fast typing).
	dirty bool

	// clearResultsTimer delays clearing stale results when a new query starts.
	// If new results arrive before the timer fires, it is cancelled and the
	// old results are replaced seamlessly — no empty-list flicker. This mirrors
	// the Flutter launcher's staleVisibleResultsDuration (80ms) strategy.
	clearResultsTimer *time.Timer

	// committedWindowHeight tracks the last applied native window height so
	// the pending-result placeholder can preserve the launcher geometry during
	// fast typing (mirrors the Flutter committedWindowHeight).
	committedWindowHeight float32
	pendingWindowHeight   float32
	resizeTimer           *time.Timer

	// lastResultCount and lastWinSize track the previous frame's state so
	// buildAndRender can skip the full-window Clear when only result content
	// changed (same count, same window size) — painting over the old frame
	// without a Clear+Present eliminates the flicker during fast typing.
	lastResultCount int
	lastWinW        int
	lastWinH        int
	lastQueryText   string

	// WebSocket UI for settings/onboarding/screenshot delegation
	wsUI *uiImpl

	// Cached visibility state (mirrors uiImpl for Manager compatibility)
	isInSettingView    bool
	isInOnboardingView bool
	isRecordingHotkey  bool

	// Preview state. currentPreview holds the preview for the active result;
	// nil means no preview is shown and the result list takes the full width.
	// resultPreviewRatio mirrors Flutter's QueryLayout.ResultPreviewWidthRatio
	// (default 0.4). previewScrollOffset shifts the preview content vertically.
	currentPreview      *plugin.WoxPreview
	resultPreviewRatio  float32
	previewScrollOffset float32
	previewImgPNG       []byte
	previewImgKey       string
	previewImgLoading   bool
	previewUnwrapping   bool

	// Toolbar state. currentToolbarMsg holds the plugin toolbar message (if any).
	// hideToolbar is set by ShowContext.HideToolbar to suppress the toolbar
	// entirely (e.g. for hotkey configs that request no chrome).
	currentToolbarMsg *plugin.ToolbarMsgUI
	hideToolbar       bool

	// toolbarActionRects stores the screen-space rects of toolbar action buttons
	// from the last layout pass, used for click hit-testing.
	toolbarActionRects []float32 // flat [x0,y0,x1,y1, x0,y0,x1,y1, ...]
}

// NewGpuUI creates a native launcher UI config. The actual native window
// (Direct2D on Windows, CoreGraphics on macOS) is created lazily in Run()
// to ensure it happens on the OS main thread.
func NewGpuUI(ctx context.Context, wsUI *uiImpl) (*gpuUIImpl, error) {
	g := &gpuUIImpl{
		theme:              ui.DefaultTheme(),
		wsUI:               wsUI,
		resultPreviewRatio: defaultResultPreviewRatio,
	}
	return g, nil
}

// defaultResultPreviewRatio mirrors the Flutter launcher's default of 0.4
// (result list gets 60% of the width, preview gets 40%). A query's
// QueryLayout.ResultPreviewWidthRatio overrides this per-query.
const defaultResultPreviewRatio = 0.4

// Init creates the native renderer and prepares the launcher window. It
// returns an error if the native renderer cannot be created (e.g. platform
// without native UI support); the caller should fall back to WebSocket UI
// in that case. Must be called on the OS main thread (via mainthread.Call).
// The window starts hidden if HideOnStart is enabled in settings.
func (g *gpuUIImpl) Init(ctx context.Context) error {
	// Apply the user's currently selected theme before creating the renderer.
	// Manager.Start loads themes into the registry but does not push the active
	// theme to gpuUIImpl, so g.theme would stay at DefaultTheme (opaque) and
	// Clear would never go transparent, hiding the Mica backdrop.
	currentTheme := GetUIManager().GetCurrentTheme(ctx)
	if currentTheme.ThemeId != "" {
		g.theme = ui.ThemeFromWoxTheme(GetUIManager().resolvePlatformTheme(ctx, currentTheme))
	}
	// Apply density-scaled toolbar height so the window geometry matches
	// the selected UI density (compact/normal/comfortable).
	g.theme.ToolbarHeight = float32(DensityToolbarHeight(ctx))
	theme := g.theme

	// Create the renderer now — this must be on the OS main thread.
	// The event callback is injected here so the renderer owns it per-instance,
	// eliminating the old package-global SetEventHandler.
	renderer, err := ui.NewNativeRenderer(800, 400, theme, g.handleEvent)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to create native renderer: %s", err.Error()))
		return err
	}
	g.renderer = renderer
	g.engine = &ui.LayoutEngine{
		Theme:    theme,
		Measurer: renderer.TextMeasurer(),
	}

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	logger.Info(ctx, fmt.Sprintf("Init: HideOnStart=%v", woxSetting.HideOnStart.Get()))
	if !woxSetting.HideOnStart.Get() {
		g.renderer.Show()
		g.mu.Lock()
		g.visible = true
		g.dirty = true
		g.mu.Unlock()
		g.requestRepaint()
	} else {
		logger.Info(ctx, "Init: window starts hidden, waiting for hotkey")
	}

	return nil
}

// StartEventLoop enters the native event loop. Must be called after a
// successful Init, on the OS main thread. Blocking semantics differ by
// platform — see NativeRenderer.StartEventLoop docs.
func (g *gpuUIImpl) StartEventLoop(ctx context.Context) {
	g.renderer.StartEventLoop(func() *ui.CommandList {
		return g.buildAndRender(ctx)
	})
}

// handleEvent processes native input events.
func (g *gpuUIImpl) handleEvent(ev ui.Event) {
	ctx := util.NewTraceContext()
	logger.Info(ctx, fmt.Sprintf("handleEvent: type=%d key=%d text=%q compose=%q", ev.Type, ev.Key, ev.Text, ev.ComposeText))

	switch ev.Type {
	case ui.EventKeyPress:
		switch ev.Key {
		case ui.KeyEscape:
			g.HideApp(ctx)
		case ui.KeyDown:
			g.mu.Lock()
			if g.selectedIdx < len(g.results)-1 {
				g.selectedIdx++
				g.dirty = true
				g.mu.Unlock()
				g.syncScrollOffsetWithSelection()
				g.syncCurrentPreview(ctx)
				g.requestRepaint()
			} else {
				g.mu.Unlock()
			}
		case ui.KeyUp:
			g.mu.Lock()
			if g.selectedIdx > 0 {
				g.selectedIdx--
				g.dirty = true
				g.mu.Unlock()
				g.syncScrollOffsetWithSelection()
				g.syncCurrentPreview(ctx)
				g.requestRepaint()
			} else {
				g.mu.Unlock()
			}
		case ui.KeyEnter:
			g.mu.Lock()
			if g.selectedIdx >= 0 && g.selectedIdx < len(g.results) {
				result := g.results[g.selectedIdx]
				g.mu.Unlock()
				g.executeResult(ctx, result)
				return
			}
			g.mu.Unlock()
		case ui.KeyLeft:
			g.moveCursorLeft(ev.Mods)
		case ui.KeyRight:
			g.moveCursorRight(ev.Mods)
		case ui.KeyHome:
			g.moveCursorHome(ev.Mods)
		case ui.KeyEnd:
			g.moveCursorEnd(ev.Mods)
		case ui.KeyDelete:
			g.deleteForward(ctx)
		case ui.KeyA:
			if ev.Mods&ui.ModControl != 0 || ev.Mods&ui.ModSuper != 0 {
				g.selectAll()
			}
		}

	case ui.EventTextInput:
		g.insertText(ctx, ev.Text)

	case ui.EventIMECompose:
		g.mu.Lock()
		g.composeValue = ev.ComposeText
		g.dirty = true
		g.mu.Unlock()
		g.requestRepaint()

	case ui.EventScroll:
		g.handleScroll(ctx, ev)

	case ui.EventClick:
		g.handleClick(ctx, ev)

	case ui.EventFocusLost:
		g.mu.Lock()
		wasVisible := g.visible
		g.visible = false
		g.mu.Unlock()
		if wasVisible {
			g.stopCursorBlink()
			g.releaseHiddenMemory()
			GetUIManager().PostOnHide(ctx)
		}
	}

	// Handle backspace outside the switch since it comes as KeyPress not TextInput
	if ev.Type == ui.EventKeyPress && ev.Key == ui.KeyBackspace {
		g.handleBackspace(ctx, ev.Mods)
	}
}

// blinkCursor toggles cursor visibility and requests a repaint. Driven by
// cursorTimer, which is reset on each tick while the window is visible.
func (g *gpuUIImpl) blinkCursor() {
	g.mu.Lock()
	g.cursorVisible = !g.cursorVisible
	g.dirty = true
	visible := g.visible
	g.mu.Unlock()
	if visible {
		g.requestRepaint()
		g.mu.Lock()
		if g.cursorTimer != nil {
			g.cursorTimer.Reset(blinkInterval)
		}
		g.mu.Unlock()
	}
}

// startCursorBlink begins the blink cycle, making the caret visible and
// scheduling the first toggle. Safe to call repeatedly; the timer is only
// created once.
func (g *gpuUIImpl) startCursorBlink() {
	g.mu.Lock()
	g.cursorVisible = true
	if g.cursorTimer == nil {
		t := time.AfterFunc(blinkInterval, g.blinkCursor)
		g.cursorTimer = t
	} else {
		g.cursorTimer.Reset(blinkInterval)
	}
	g.mu.Unlock()
}

// stopCursorBlink stops the blink timer and hides the caret.
func (g *gpuUIImpl) stopCursorBlink() {
	g.mu.Lock()
	if g.cursorTimer != nil {
		g.cursorTimer.Stop()
		g.cursorTimer = nil
	}
	g.cursorVisible = false
	g.mu.Unlock()
}

// blinkInterval is the caret blink rate (530ms, matching platform conventions).
const blinkInterval = 530 * time.Millisecond

// clearSelection resets the selection range to "no selection".
func (g *gpuUIImpl) clearSelection() {
	g.selStart = -1
	g.selEnd = -1
}

// selectAll selects the entire query text and moves the caret to the end.
func (g *gpuUIImpl) selectAll() {
	g.mu.Lock()
	if len(g.queryValue) > 0 {
		g.selStart = 0
		g.selEnd = len(g.queryValue)
		g.cursorPos = g.selEnd
		g.cursorVisible = true
		g.dirty = true
	}
	g.mu.Unlock()
	g.requestRepaint()
}

// moveCursorLeft moves the caret one rune left. With Shift held, the selection
// extends instead of collapsing.
func (g *gpuUIImpl) moveCursorLeft(mods ui.Modifiers) {
	g.mu.Lock()
	runes := []rune(g.queryValue)
	// Convert cursorPos (byte offset) to rune index
	byteIdx := g.cursorPos
	runeIdx := len([]rune(g.queryValue[:byteIdx]))
	if runeIdx > 0 {
		runeIdx--
		newByteIdx := len(string(runes[:runeIdx]))
		if mods&ui.ModShift != 0 {
			// Extend selection
			if g.selStart < 0 {
				g.selStart = g.cursorPos
				g.selEnd = g.cursorPos
			}
			if g.cursorPos == g.selEnd && g.cursorPos > g.selStart {
				g.selEnd = newByteIdx
			} else if g.cursorPos == g.selStart {
				g.selStart = newByteIdx
			} else {
				g.selStart = newByteIdx
				g.selEnd = g.cursorPos
			}
		} else {
			g.clearSelection()
		}
		g.cursorPos = newByteIdx
		g.cursorVisible = true
		g.dirty = true
	} else if mods&ui.ModShift == 0 {
		g.clearSelection()
		g.dirty = true
	}
	g.mu.Unlock()
	g.requestRepaint()
}

// moveCursorRight moves the caret one rune right.
func (g *gpuUIImpl) moveCursorRight(mods ui.Modifiers) {
	g.mu.Lock()
	runes := []rune(g.queryValue)
	byteIdx := g.cursorPos
	runeIdx := len([]rune(g.queryValue[:byteIdx]))
	if runeIdx < len(runes) {
		runeIdx++
		newByteIdx := len(string(runes[:runeIdx]))
		if mods&ui.ModShift != 0 {
			if g.selStart < 0 {
				g.selStart = g.cursorPos
				g.selEnd = g.cursorPos
			}
			if g.cursorPos == g.selEnd {
				g.selEnd = newByteIdx
			} else if g.cursorPos == g.selStart {
				g.selStart = newByteIdx
			} else {
				g.selStart = g.cursorPos
				g.selEnd = newByteIdx
			}
		} else {
			g.clearSelection()
		}
		g.cursorPos = newByteIdx
		g.cursorVisible = true
		g.dirty = true
	} else if mods&ui.ModShift == 0 {
		g.clearSelection()
		g.dirty = true
	}
	g.mu.Unlock()
	g.requestRepaint()
}

// moveCursorHome moves the caret to the beginning of the text.
func (g *gpuUIImpl) moveCursorHome(mods ui.Modifiers) {
	g.mu.Lock()
	if mods&ui.ModShift != 0 {
		if g.selStart < 0 {
			g.selStart = 0
			g.selEnd = g.cursorPos
		} else {
			g.selStart = 0
		}
	} else {
		g.clearSelection()
	}
	g.cursorPos = 0
	g.cursorVisible = true
	g.dirty = true
	g.mu.Unlock()
	g.requestRepaint()
}

// moveCursorEnd moves the caret to the end of the text.
func (g *gpuUIImpl) moveCursorEnd(mods ui.Modifiers) {
	g.mu.Lock()
	end := len(g.queryValue)
	if mods&ui.ModShift != 0 {
		if g.selStart < 0 {
			g.selStart = g.cursorPos
			g.selEnd = end
		} else {
			g.selEnd = end
		}
	} else {
		g.clearSelection()
	}
	g.cursorPos = end
	g.cursorVisible = true
	g.dirty = true
	g.mu.Unlock()
	g.requestRepaint()
}

// insertText handles EventTextInput: inserts committed text at the cursor
// position, replacing any active selection, then triggers a new query.
func (g *gpuUIImpl) insertText(ctx context.Context, text string) {
	g.mu.Lock()
	g.composeValue = ""

	// If there's a selection, delete it first
	if g.selStart >= 0 && g.selEnd > g.selStart {
		g.queryValue = g.queryValue[:g.selStart] + text + g.queryValue[g.selEnd:]
		g.cursorPos = g.selStart + len(text)
	} else {
		// Insert at cursor position
		g.queryValue = g.queryValue[:g.cursorPos] + text + g.queryValue[g.cursorPos:]
		g.cursorPos += len(text)
	}
	g.clearSelection()
	g.cursorVisible = true
	g.dirty = true
	g.mu.Unlock()
	g.triggerQuery(ctx)
}

// handleBackspace deletes the character before the cursor (or the selected
// range if a selection is active).
func (g *gpuUIImpl) handleBackspace(ctx context.Context, mods ui.Modifiers) {
	g.mu.Lock()
	if g.selStart >= 0 && g.selEnd > g.selStart {
		// Delete the selection
		g.queryValue = g.queryValue[:g.selStart] + g.queryValue[g.selEnd:]
		g.cursorPos = g.selStart
		g.clearSelection()
	} else if g.cursorPos > 0 {
		// Delete one rune before the cursor
		runes := []rune(g.queryValue[:g.cursorPos])
		if len(runes) > 0 {
			newCursor := len(string(runes[:len(runes)-1]))
			g.queryValue = g.queryValue[:newCursor] + g.queryValue[g.cursorPos:]
			g.cursorPos = newCursor
		}
	} else if len(g.queryValue) > 0 {
		// Fallback: delete last character (old behavior when cursorPos wasn't tracked)
		runes := []rune(g.queryValue)
		g.queryValue = string(runes[:len(runes)-1])
	} else {
		g.mu.Unlock()
		return
	}
	g.cursorVisible = true
	g.dirty = true
	g.mu.Unlock()
	g.triggerQuery(ctx)
}

// deleteForward deletes the character after the cursor (Delete key).
func (g *gpuUIImpl) deleteForward(ctx context.Context) {
	g.mu.Lock()
	if g.selStart >= 0 && g.selEnd > g.selStart {
		g.queryValue = g.queryValue[:g.selStart] + g.queryValue[g.selEnd:]
		g.cursorPos = g.selStart
		g.clearSelection()
	} else if g.cursorPos < len(g.queryValue) {
		g.queryValue = g.queryValue[:g.cursorPos] + g.queryValue[g.cursorPos+1:]
	}
	g.cursorVisible = true
	g.dirty = true
	g.mu.Unlock()
	g.triggerQuery(ctx)
}

// buildToolbarActions returns the action buttons to render on the toolbar
// right side. Actions come from the selected result's action list (only those
// with hotkeys). When no result is selected or there are no actions with
// hotkeys, returns nil.
func (g *gpuUIImpl) buildToolbarActions(ctx context.Context) []ui.ToolbarAction {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.buildToolbarActionsLocked()
}

// buildAndRender generates the draw command list from current state.
func (g *gpuUIImpl) buildAndRender(ctx context.Context) *ui.CommandList {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.visible {
		logger.Info(ctx, "buildAndRender: not visible, skip")
		return nil
	}
	// Only render when something actually changed. Without this gate every
	// message-loop wakeup would do a full Clear + redraw, making the selected
	// result background flicker during fast typing.
	if !g.dirty {
		if runtime.GOOS != "darwin" {
			logger.Info(ctx, "buildAndRender: not dirty, skip")
			return nil
		}
		// Cocoa may call drawRect to rebuild an exposed or newly ordered
		// backing store after an earlier hidden resize consumed the dirty bit.
		// Return a full frame on macOS so a transparent NSPanel never presents
		// with only its vibrancy layer visible.
	} else {
		g.dirty = false
	}

	// Use the real window dimensions so layout matches the native surface.
	// Falls back to the initial 800x400 before the first WM_SIZE arrives.
	winW, winH := 800, 400
	if g.renderer != nil {
		if w, h := g.renderer.GetSize(); w > 0 && h > 0 {
			winW, winH = w, h
		}
	}
	t := g.theme
	resultCount := len(g.results)

	// Convert query results to list items. Icons are pre-rasterized by
	// rasterizeIconsInBackground on a goroutine, so this loop only reads
	// already-cached PNG bytes — no decoding on the main thread.
	items := make([]ui.ListItem, len(g.results))
	for i, r := range g.results {
		items[i] = ui.ListItem{
			Title:    r.Title,
			Subtitle: r.SubTitle,
			IconPNG:  r.IconPNG,
			IconKey:  r.IconKey,
		}
	}

	// Build the result area. When the active result has a preview, the result
	// list and preview panel share the width horizontally (results on the left,
	// preview on the right) according to resultPreviewRatio. Without a preview
	// the list takes the full width.
	listBox := ui.ListBox{
		ID:            "results",
		ItemHeight:    t.ListItemHeight,
		Items:         items,
		ScrollOffset:  g.scrollOffset,
		Selected:      g.selectedIdx,
		SelectedColor: &t.SelectedBg,
	}

	var resultArea ui.Widget
	if g.currentPreview != nil && !g.currentPreview.IsEmpty() {
		// The HBox lives inside the root VBox whose inner width excludes the
		// window padding on both sides, so compute the split from the inner
		// width, not the full window width.
		innerW := float32(winW) - t.WindowPadding*2
		resultW := innerW * (1 - g.resultPreviewRatio)
		previewW := innerW * g.resultPreviewRatio
		listBox.Width = resultW
		previewPanel := ui.PreviewPanel{
			ID:           "preview",
			PreviewType:  g.currentPreview.PreviewType,
			PreviewData:  g.currentPreview.PreviewData,
			PreviewTags:  toUIPreviewTags(g.currentPreview.PreviewTags),
			ScrollOffset: g.previewScrollOffset,
			BgColor:      &t.PreviewBg,
			SplitColor:   t.PreviewSplitLineColor,
			FontColor:    t.PreviewFontColor,
			FontSize:     t.FontSize,
			FontFamily:   t.FontFamily,
			ImagePNG:     g.previewImgPNG,
			ImageKey:     g.previewImgKey,
			Width:        previewW,
		}
		resultArea = ui.HBox{
			Gap:     0,
			Padding: 0,
			Children: []ui.Widget{
				listBox,
				previewPanel,
			},
		}
	} else {
		resultArea = listBox
	}

	// During IME composition the textbox shows queryValue + composeValue so the
	// user sees the in-progress composition inline. A full underline/highlight
	// style is not yet supported by layout.go; this simple concatenation keeps
	// the composition visible (and is cleared when the IME commits).
	displayValue := g.queryValue + g.composeValue

	// Build the toolbar widget. When not visible it contributes zero height.
	toolbar := ui.Toolbar{
		ID:           "toolbar",
		Height:       t.ToolbarHeight,
		Visible:      g.toolbarVisible(),
		BgColor:      t.ToolbarBg,
		FontColor:    t.ToolbarFontColor,
		PaddingLeft:  t.ToolbarPaddingLeft,
		PaddingRight: t.ToolbarPaddingRight,
		TopBorder:    len(g.results) > 0,
	}
	// Populate left area from the current toolbar message (if any).
	if g.currentToolbarMsg != nil {
		toolbar.LeftText = g.currentToolbarMsg.Title
		if !g.currentToolbarMsg.Icon.IsEmpty() {
			iconPNG, iconKey := ui.RasterizeWoxImageWithSizeAndKey(g.currentToolbarMsg.Icon, 16)
			toolbar.LeftIcon = iconPNG
			toolbar.LeftIconKey = iconKey
		}
		if g.currentToolbarMsg.Progress != nil {
			p := *g.currentToolbarMsg.Progress
			toolbar.Progress = &p
		}
		toolbar.Indeterminate = g.currentToolbarMsg.Indeterminate
	}
	// Populate right area with action buttons from the selected result.
	// Use the lock-free variant because buildAndRender already holds g.mu.
	toolbar.Actions = g.buildToolbarActionsLocked()

	root := ui.VBox{
		Padding: t.WindowPadding,
		Gap:     12,
		Children: []ui.Widget{
			ui.TextBox{
				ID:             "query",
				Placeholder:    "Type to search...",
				FontSize:       t.FontSize,
				FontColor:      t.QueryBoxFontColor,
				BgColor:        t.QueryBoxBg,
				CornerRadius:   t.QueryBoxRadius,
				CursorColor:    t.QueryBoxCursorColor,
				Value:          displayValue,
				Focused:        true,
				CursorPos:      g.cursorPos,
				SelectionStart: g.selStart,
				SelectionEnd:   g.selEnd,
				SelectionColor: ui.Color{R: 0.3, G: 0.5, B: 0.9, A: 0.3},
				BlinkVisible:   g.cursorVisible,
			},
			resultArea,
			toolbar,
		},
	}

	result := g.engine.Layout(root, float32(winW), float32(winH))
	g.lastResultCount = resultCount
	g.lastWinW = winW
	g.lastWinH = winH
	g.lastQueryText = g.queryValue

	// Set the toolbar drag region so the user can move the window by dragging
	// the toolbar area. The drag band spans the full toolbar height at the
	// bottom of the window.
	if g.toolbarVisible() {
		toolbarY := float32(winH) - g.theme.ToolbarHeight - g.theme.WindowPadding
		g.renderer.SetDragRegion(toolbarY, float32(winH))
	} else {
		g.renderer.SetDragRegion(0, 0)
	}

	return &result.Commands
}

// maxResultCount returns the configured maximum number of visible results,
// defaulting to 8 (the same default as WoxSetting.MaxResultCount).
const defaultMaxResultCount = 8
const shrinkWindowDelay = 120 * time.Millisecond

// resizeWindowHeight sets the native window size to match the current result
// count. Called after results arrive so the launcher grows/shrinks with the
// list. The resize is posted to the message queue (WM_APP_RESIZE), which
// atomically chains SetWindowPos → WM_SIZE (bitmap rebuild) → onRender, so
// no separate requestRepaint is needed.
func (g *gpuUIImpl) resizeWindowHeight() {
	if g.renderer == nil {
		return
	}

	// Read theme and results under the lock to avoid data races with
	// concurrent goroutines updating g.results / g.theme.
	g.mu.Lock()
	t := g.theme
	itemCount := len(g.results)
	hasPreview := g.currentPreview != nil && !g.currentPreview.IsEmpty()
	lastCommittedHeight := g.committedWindowHeight
	// Toolbar height — include it in the total when visible.
	toolbarH := float32(0)
	if !g.hideToolbar && (itemCount > 0 || g.currentToolbarMsg != nil) {
		toolbarH = t.ToolbarHeight
	}
	g.mu.Unlock()

	pad := t.WindowPadding * 2
	queryBoxH := t.QueryBoxHeight
	if itemCount > defaultMaxResultCount {
		itemCount = defaultMaxResultCount
	}
	resultH := float32(itemCount) * t.ListItemHeight
	maxResultH := float32(defaultMaxResultCount) * t.ListItemHeight
	if resultH > maxResultH {
		resultH = maxResultH
	}
	gap := float32(12)
	total := queryBoxH + gap + resultH + toolbarH + pad
	// When a preview is visible, give the launcher a minimum height so the
	// preview area has enough room to be useful even when there are few results.
	// This matches the Flutter launcher behavior where preview-only queries
	// still reserve a usable surface.
	if hasPreview {
		minPreviewH := queryBoxH + gap + maxResultH + pad
		if total < minPreviewH {
			total = minPreviewH
		}
	}
	minH := queryBoxH + pad
	if total < minH {
		total = minH
	}
	targetH := total

	// Skip if the target height hasn't changed since the last commit.
	if lastCommittedHeight > 0 && lastCommittedHeight == targetH {
		g.requestRepaint()
		return
	}

	if lastCommittedHeight > 0 && targetH < lastCommittedHeight {
		g.mu.Lock()
		if g.resizeTimer != nil {
			g.resizeTimer.Stop()
		}
		currentQueryId := g.currentQuery.Id
		g.pendingWindowHeight = targetH
		g.resizeTimer = time.AfterFunc(shrinkWindowDelay, func() {
			g.mu.Lock()
			shouldResize := g.currentQuery.Id == currentQueryId && g.pendingWindowHeight == targetH
			if shouldResize {
				g.resizeTimer = nil
			}
			g.mu.Unlock()
			if shouldResize {
				g.applyWindowHeight(targetH)
			}
		})
		g.mu.Unlock()
		g.requestRepaint()
		return
	}

	g.mu.Lock()
	if g.resizeTimer != nil {
		g.resizeTimer.Stop()
		g.resizeTimer = nil
	}
	g.pendingWindowHeight = 0
	g.mu.Unlock()
	g.applyWindowHeight(targetH)
}

// applyWindowHeight commits a native height resize and marks the next frame dirty.
func (g *gpuUIImpl) applyWindowHeight(targetH float32) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(util.NewTraceContext())
	width := woxSetting.AppWidth.Get()
	if width <= 0 {
		width = 750
	}
	g.renderer.SetSize(width, int(targetH))
	g.mu.Lock()
	g.committedWindowHeight = targetH
	g.pendingWindowHeight = 0
	g.dirty = true
	g.mu.Unlock()
}

// requestRepaint posts a repaint message to the native message loop so the
// main thread re-renders after results or query text change. Safe to call
// from any goroutine — PostMessage is thread-safe.
func (g *gpuUIImpl) requestRepaint() {
	if g.renderer != nil {
		g.renderer.RequestRepaint()
	}
}

// rasterizeIconsInBackground pre-rasterizes result icons to PNG on a
// background goroutine so the main render thread never blocks on icon
// decoding. buildAndRender only reads the already-rasterized bytes.
func (g *gpuUIImpl) rasterizeIconsInBackground(results []plugin.QueryResultUI) {
	if len(results) == 0 {
		return
	}
	go func() {
		// Rasterize icons at the physical pixel size matching the current
		// DPI scale. If the display is 150% (DPI=144), icons are rasterized
		// to 54px instead of 36px so the native layer doesn't upscale a
		// small bitmap (which causes blurriness). Falls back to 36 on
		// platforms where GetDPI is unavailable.
		iconPx := 36
		if g.renderer != nil {
			dpi := g.renderer.GetDPI()
			if dpi > 0 {
				iconPx = int(36 * dpi / 96.0)
				if iconPx < 36 {
					iconPx = 36
				}
			}
		}
		for i := range results {
			r := &results[i]
			if r.Icon.IsEmpty() {
				continue
			}
			// RasterizeWoxImageWithSizeAndKey caches by hash+size, so
			// repeated calls for the same icon are cheap (map lookup).
			png, key := ui.RasterizeWoxImageWithSizeAndKey(r.Icon, iconPx)
			g.mu.Lock()
			if i < len(g.results) && g.results[i].Title == r.Title {
				g.results[i].IconPNG = png
				g.results[i].IconKey = key
				g.dirty = true
			}
			g.mu.Unlock()
		}
		g.requestRepaint()
	}()
}

// triggerQuery starts a new plugin query with the current query text.
func (g *gpuUIImpl) triggerQuery(ctx context.Context) {
	g.mu.Lock()
	queryText := g.queryValue
	g.mu.Unlock()

	go func(queryText string) {
		queryCtx := util.NewTraceContext()
		queryId := uuid.NewString()
		plainQuery := common.PlainQuery{
			QueryId:   queryId,
			QueryType: plugin.QueryTypeInput,
			QueryText: queryText,
		}
		q, ownerPlugin, err := plugin.GetPluginManager().NewQuery(queryCtx, plainQuery)
		if err != nil {
			logger.Error(queryCtx, fmt.Sprintf("gpuUI query error: %s", err.Error()))
			return
		}

		g.mu.Lock()
		g.currentQuery = q
		g.currentSessionId = util.GetContextSessionId(queryCtx)
		// Don't clear results immediately. Keep the old result list visible
		// and set a delay timer: if new results arrive within 80ms the timer
		// is cancelled and results are replaced seamlessly (no empty-list
		// flicker). If the query is still pending after 80ms, clear then.
		// This mirrors the Flutter launcher's staleVisibleResultsDuration.
		if g.clearResultsTimer != nil {
			g.clearResultsTimer.Stop()
		}
		currentQueryId := q.Id
		var clearTimer *time.Timer
		clearTimer = time.AfterFunc(80*time.Millisecond, func() {
			shouldRepaint := false
			g.mu.Lock()
			if g.currentQuery.Id == currentQueryId && g.clearResultsTimer == clearTimer {
				g.results = nil
				g.selectedIdx = 0
				g.scrollOffset = 0
				g.currentPreview = nil
				g.previewScrollOffset = 0
				g.previewImgPNG = nil
				g.previewImgKey = ""
				g.previewImgLoading = false
				g.previewUnwrapping = false
				g.dirty = true
				g.clearResultsTimer = nil
				shouldRepaint = true
			}
			g.mu.Unlock()
			if shouldRepaint {
				g.requestRepaint()
			}
		})
		g.clearResultsTimer = clearTimer
		g.selectedIdx = 0
		g.scrollOffset = 0
		g.currentPreview = nil
		g.previewScrollOffset = 0
		g.previewImgPNG = nil
		g.previewImgKey = ""
		g.previewImgLoading = false
		g.previewUnwrapping = false
		g.dirty = true
		g.mu.Unlock()

		// Required: lifecycle handling must run before Query
		plugin.GetPluginManager().HandleQueryLifecycle(queryCtx, q, ownerPlugin)

		resultChan, fallbackReadyChan, doneChan := plugin.GetPluginManager().Query(queryCtx, q)

		var allResults []plugin.QueryResultUI
		// firstResponse tracks whether we've seen the first QueryResponse so we
		// can apply its QueryLayout.ResultPreviewWidthRatio once per query.
		firstResponse := true
		for {
			select {
			case response := <-resultChan:
				allResults = append(allResults, response.Results...)
				g.mu.Lock()
				if g.currentQuery.Id != currentQueryId {
					g.mu.Unlock()
					return
				}
				if g.clearResultsTimer != nil {
					g.clearResultsTimer.Stop()
					g.clearResultsTimer = nil
				}
				// Apply the per-query result/preview width ratio from the first
				// response that carries a Layout. Subsequent responses keep the
				// ratio already applied; a null ratio falls back to the default.
				if firstResponse {
					if response.Layout.ResultPreviewWidthRatio != nil {
						r := float32(*response.Layout.ResultPreviewWidthRatio)
						if r > 0 {
							g.resultPreviewRatio = r
						}
					} else {
						g.resultPreviewRatio = defaultResultPreviewRatio
					}
					firstResponse = false
				}
				g.results = allResults
				g.dirty = true
				g.mu.Unlock()
				g.rasterizeIconsInBackground(allResults)
				g.resizeWindowHeight()
				g.syncCurrentPreview(queryCtx)
			case <-fallbackReadyChan:
				// drain any pending results
				for {
					select {
					case response := <-resultChan:
						allResults = append(allResults, response.Results...)
					default:
						goto fallbackDone
					}
				}
			fallbackDone:
				g.mu.Lock()
				if g.currentQuery.Id != currentQueryId {
					g.mu.Unlock()
					return
				}
				if g.clearResultsTimer != nil {
					g.clearResultsTimer.Stop()
					g.clearResultsTimer = nil
				}
				g.results = allResults
				g.dirty = true
				g.mu.Unlock()
				g.rasterizeIconsInBackground(allResults)
				g.resizeWindowHeight()
				g.syncCurrentPreview(queryCtx)
			case <-doneChan:
				// drain any final results
				for {
					select {
					case response := <-resultChan:
						allResults = append(allResults, response.Results...)
					default:
						goto queryDone
					}
				}
			queryDone:
				g.mu.Lock()
				if g.currentQuery.Id != currentQueryId {
					g.mu.Unlock()
					return
				}
				if g.clearResultsTimer != nil {
					g.clearResultsTimer.Stop()
					g.clearResultsTimer = nil
				}
				g.results = allResults
				g.dirty = true
				g.mu.Unlock()
				g.rasterizeIconsInBackground(allResults)
				g.resizeWindowHeight()
				g.syncCurrentPreview(queryCtx)
				logger.Info(queryCtx, fmt.Sprintf("gpuUI query done: %d results", len(allResults)))
				return
			case <-time.After(time.Minute):
				logger.Info(queryCtx, fmt.Sprintf("gpuUI query timeout: %s", queryText))
				return
			}
		}
	}(queryText)
}

// executeResult triggers the selected result's default action.
func (g *gpuUIImpl) executeResult(ctx context.Context, result plugin.QueryResultUI) {
	g.mu.Lock()
	queryId := g.currentQuery.Id
	sessionId := g.currentSessionId
	g.mu.Unlock()

	if result.Actions != nil && len(result.Actions) > 0 {
		actionId := result.Actions[0].Id
		err := plugin.GetPluginManager().ExecuteAction(ctx, sessionId, queryId, result.Id, actionId)
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("execute action error: %s", err.Error()))
		}
	}
	g.HideApp(ctx)
}

// ── common.UI interface ──────────────────────────────────────────────────

func (g *gpuUIImpl) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	g.mu.Lock()
	g.queryValue = query.QueryText
	g.selectedIdx = 0
	g.scrollOffset = 0
	g.cursorPos = len(query.QueryText)
	g.clearSelection()
	g.dirty = true
	g.mu.Unlock()
	g.requestRepaint()
}

func (g *gpuUIImpl) RefreshQuery(ctx context.Context, preserveSelectedIndex bool) {
	// Re-trigger query with current text
	g.triggerQuery(ctx)
}

func (g *gpuUIImpl) RefreshGlance(ctx context.Context, pluginId string, ids []string) {
	// TODO: glance items
}

func (g *gpuUIImpl) UpdateDiagnosticStatus(ctx context.Context, enabled bool) {
	// TODO: diagnostic status indicator
}

func (g *gpuUIImpl) HideApp(ctx context.Context) {
	g.mu.Lock()
	g.visible = false
	g.mu.Unlock()
	g.stopCursorBlink()
	if g.renderer != nil {
		g.renderer.Hide()
	}
	g.releaseHiddenMemory()
	GetUIManager().PostOnHide(ctx)
}

// releaseHiddenMemory drops launcher-only caches and asks Go to return idle memory.
func (g *gpuUIImpl) releaseHiddenMemory() {
	if g.renderer != nil {
		g.renderer.ReleaseMemory()
	}
	ui.ClearIconCache()
	debug.FreeOSMemory()
}

func (g *gpuUIImpl) ShowApp(ctx context.Context, showContext common.ShowContext) {
	GetUIManager().RefreshActiveWindowSnapshot(ctx)

	if g.renderer == nil {
		logger.Error(ctx, "ShowApp: renderer is nil!")
		return
	}

	// Apply HideToolbar from the show context
	g.mu.Lock()
	g.hideToolbar = showContext.HideToolbar
	g.mu.Unlock()

	// Reset window height to match current result count before showing.
	g.resizeWindowHeight()

	// Position window: use explicit position from context, otherwise center
	// on the screen where the mouse is.
	winW, winH := 800, 400
	if showContext.WindowPosition != nil {
		g.renderer.SetPosition(showContext.WindowPosition.X, showContext.WindowPosition.Y)
	} else {
		screenSize := screen.GetMouseScreen()
		x := screenSize.X + (screenSize.Width-winW)/2
		y := screenSize.Y + (screenSize.Height-winH)/3 // upper third
		g.renderer.SetPosition(x, y)
	}

	// Reset query state on show
	g.mu.Lock()
	g.visible = true
	g.dirty = true
	// Handle select-all on show (mirrors Flutter's selectAll param)
	if showContext.SelectAll && len(g.queryValue) > 0 {
		g.selStart = 0
		g.selEnd = len(g.queryValue)
		g.cursorPos = g.selEnd
	} else {
		// Place cursor at end of text
		g.cursorPos = len(g.queryValue)
		g.clearSelection()
	}
	g.mu.Unlock()
	g.startCursorBlink()
	g.renderer.Show()
	g.requestRepaint()
	GetUIManager().PostOnShow(ctx)
}

func (g *gpuUIImpl) ToggleApp(ctx context.Context, showContext common.ShowContext) {
	if g.IsVisible(ctx) {
		g.HideApp(ctx)
	} else {
		g.ShowApp(ctx, showContext)
	}
}

func (g *gpuUIImpl) RecordHotkey(ctx context.Context, hotkey string) {
	// TODO: native hotkey recording
}

func (g *gpuUIImpl) GetServerPort(ctx context.Context) int {
	return GetUIManager().serverPort
}

func (g *gpuUIImpl) ChangeTheme(ctx context.Context, theme common.Theme) {
	logger.Info(ctx, fmt.Sprintf("change theme: %s", theme.ThemeName))
	if theme.IsAutoAppearance {
		GetUIManager().ChangeTheme(ctx, theme)
		return
	}
	effectiveTheme := GetUIManager().resolvePlatformTheme(ctx, theme)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	woxSetting.ThemeId.Set(effectiveTheme.ThemeId)

	// Convert Wox theme JSON → ui.Theme and update the layout engine
	g.mu.Lock()
	g.theme = ui.ThemeFromWoxTheme(effectiveTheme)
	g.theme.ToolbarHeight = float32(DensityToolbarHeight(ctx))
	if g.engine != nil {
		g.engine.Theme = g.theme
	}
	g.dirty = true
	if g.renderer != nil {
		bg := g.theme.WindowBg
		lum := 0.2126*bg.R + 0.7152*bg.G + 0.0722*bg.B
		g.renderer.SetDarkMode(lum < 0.5)
	}
	g.mu.Unlock()
	g.requestRepaint()
}

func (g *gpuUIImpl) InstallTheme(ctx context.Context, theme common.Theme) {
	GetStoreManager().Install(ctx, theme)
}

func (g *gpuUIImpl) UninstallTheme(ctx context.Context, theme common.Theme) {
	GetStoreManager().Uninstall(ctx, theme)
	GetUIManager().ChangeToDefaultTheme(ctx)
}

func (g *gpuUIImpl) GetAllThemes(ctx context.Context) []common.Theme {
	return GetUIManager().GetAllThemes(ctx)
}

func (g *gpuUIImpl) RestoreTheme(ctx context.Context) {
	GetUIManager().RestoreTheme(ctx)
}

func (g *gpuUIImpl) Notify(ctx context.Context, msg common.NotifyMsg) {
	// Delegate to WS UI (which may show toolbar msg or system notification)
	g.wsUI.Notify(ctx, msg)
}

func (g *gpuUIImpl) UpdateAttentionUnreadCount(ctx context.Context, unreadCount int) {
	// TODO: tray icon attention
}

func (g *gpuUIImpl) ShowToolbarMsg(ctx context.Context, msg interface{}) {
	toolbarMsg, ok := msg.(plugin.ToolbarMsgUI)
	if !ok {
		// Fallback to WS UI for unexpected types
		g.wsUI.ShowToolbarMsg(ctx, msg)
		return
	}
	g.mu.Lock()
	g.currentToolbarMsg = &toolbarMsg
	g.dirty = true
	g.mu.Unlock()
	g.resizeWindowHeight()
	g.requestRepaint()
}

func (g *gpuUIImpl) ClearToolbarMsg(ctx context.Context, toolbarMsgId string) {
	g.mu.Lock()
	if g.currentToolbarMsg != nil && g.currentToolbarMsg.Id == toolbarMsgId {
		g.currentToolbarMsg = nil
		g.dirty = true
	}
	g.mu.Unlock()
	g.resizeWindowHeight()
	g.requestRepaint()
}

func (g *gpuUIImpl) IsVisible(ctx context.Context) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.visible
}

func (g *gpuUIImpl) IsInSettingView() bool {
	return g.isInSettingView
}

func (g *gpuUIImpl) IsInManagementView() bool {
	return g.isInSettingView || g.isInOnboardingView
}

// ── Settings/onboarding delegation to WS UI ─────────────────────────────

func (g *gpuUIImpl) OpenSettingWindow(ctx context.Context, windowContext common.SettingWindowContext) {
	g.wsUI.OpenSettingWindow(ctx, windowContext)
}

func (g *gpuUIImpl) FocusSettingWindow(ctx context.Context) {
	g.wsUI.FocusSettingWindow(ctx)
}

func (g *gpuUIImpl) OpenOnboardingWindow(ctx context.Context) {
	g.wsUI.OpenOnboardingWindow(ctx)
}

func (g *gpuUIImpl) ReloadSettingPlugins(ctx context.Context) {
	g.wsUI.ReloadSettingPlugins(ctx)
}

func (g *gpuUIImpl) ReloadSetting(ctx context.Context) {
	g.wsUI.ReloadSetting(ctx)
}

func (g *gpuUIImpl) ReloadSettingThemes(ctx context.Context) {
	g.wsUI.ReloadSettingThemes(ctx)
}

func (g *gpuUIImpl) CloudSyncProgressChanged(ctx context.Context, progress any) {
	g.wsUI.CloudSyncProgressChanged(ctx, progress)
}

func (g *gpuUIImpl) RefreshAccountStatus(ctx context.Context) {
	g.wsUI.RefreshAccountStatus(ctx)
}

// ── Result update methods ───────────────────────────────────────────────

func (g *gpuUIImpl) UpdateResult(ctx context.Context, result interface{}) bool {
	// TODO: implement result update without full re-query
	return false
}

func (g *gpuUIImpl) PushResults(ctx context.Context, payload interface{}) bool {
	// TODO: implement incremental result push
	return true
}

// ── Native-only methods (stubs for now) ─────────────────────────────────

func (g *gpuUIImpl) PickFiles(ctx context.Context, params common.PickFilesParams) []string {
	return g.wsUI.PickFiles(ctx, params)
}

func (g *gpuUIImpl) CaptureScreenshot(ctx context.Context, request common.CaptureScreenshotRequest) (common.CaptureScreenshotResult, error) {
	return g.wsUI.CaptureScreenshot(ctx, request)
}

func (g *gpuUIImpl) WriteClipboardImageFile(ctx context.Context, filePath string) error {
	return g.wsUI.WriteClipboardImageFile(ctx, filePath)
}

func (g *gpuUIImpl) GetActiveWindowSnapshot(ctx context.Context) common.ActiveWindowSnapshot {
	return GetUIManager().GetActiveWindowSnapshot(ctx)
}

func (g *gpuUIImpl) FocusToChatInput(ctx context.Context) {
	// TODO: AI chat view
}

func (g *gpuUIImpl) SendChatResponse(ctx context.Context, chatData common.AIChatData) {
	// TODO: AI chat view
}

func (g *gpuUIImpl) ReloadChatResources(ctx context.Context, resouceName string) {
	// TODO: AI chat resources
}

// SetResults replaces the full result list (called by query manager).
func (g *gpuUIImpl) SetResults(ctx context.Context, results []plugin.QueryResultUI) {
	g.mu.Lock()
	g.results = results
	if g.selectedIdx >= len(results) {
		g.selectedIdx = 0
	}
	g.scrollOffset = 0
	g.dirty = true
	g.mu.Unlock()
	g.rasterizeIconsInBackground(results)
	g.resizeWindowHeight()
	g.syncCurrentPreview(ctx)
}

// ── Preview support ─────────────────────────────────────────────────────

// toUIPreviewTags converts plugin preview tags to the ui package's PreviewTag.
// Allocation-free when there are no tags.
func toUIPreviewTags(tags []plugin.WoxPreviewTag) []ui.PreviewTag {
	if len(tags) == 0 {
		return nil
	}
	out := make([]ui.PreviewTag, len(tags))
	for i, t := range tags {
		out[i] = ui.PreviewTag{Label: t.Label, Tooltip: t.Tooltip}
	}
	return out
}

// syncCurrentPreview updates currentPreview from the active result and kicks
// off any async work (image rasterization / remote unwrap) the new preview
// needs. Safe to call when no results are selected (clears the preview).
func (g *gpuUIImpl) syncCurrentPreview(ctx context.Context) {
	g.mu.Lock()
	if g.selectedIdx < 0 || g.selectedIdx >= len(g.results) {
		g.currentPreview = nil
		g.previewImgPNG = nil
		g.previewImgKey = ""
		g.previewImgLoading = false
		g.previewUnwrapping = false
		g.previewScrollOffset = 0
		g.dirty = true
		g.mu.Unlock()
		return
	}

	preview := g.results[g.selectedIdx].Preview
	if preview.IsEmpty() {
		g.currentPreview = nil
		g.previewImgPNG = nil
		g.previewImgKey = ""
		g.previewImgLoading = false
		g.previewUnwrapping = false
		g.previewScrollOffset = 0
		g.dirty = true
		g.mu.Unlock()
		return
	}

	// Capture a copy so async goroutines don't race with result replacement.
	previewCopy := preview
	g.currentPreview = &previewCopy
	g.previewScrollOffset = 0
	g.previewImgPNG = nil
	g.previewImgKey = ""
	g.previewImgLoading = false
	g.previewUnwrapping = false
	previewType := previewCopy.PreviewType
	g.dirty = true
	g.mu.Unlock()

	switch previewType {
	case plugin.WoxPreviewTypeImage:
		g.rasterizePreviewImageInBackground(previewCopy)
	case plugin.WoxPreviewTypeRemote:
		g.unwrapRemotePreview(ctx, previewCopy)
	}
}

// rasterizePreviewImageInBackground converts the preview's WoxImage (stored in
// PreviewData as "type:data") to PNG on a goroutine so the render thread never
// blocks on image decoding. The result is written back to previewImgPNG/Key
// and a repaint is requested.
func (g *gpuUIImpl) rasterizePreviewImageInBackground(preview plugin.WoxPreview) {
	g.mu.Lock()
	g.previewImgLoading = true
	g.mu.Unlock()

	go func() {
		img := common.ParseWoxImageString(preview.PreviewData)
		if img.IsEmpty() {
			g.mu.Lock()
			g.previewImgLoading = false
			g.mu.Unlock()
			return
		}
		// Preview images can be large; use a generous size cap so they fit the
		// panel without decoding at full photo resolution.
		png, key := ui.RasterizeWoxImageWithSizeAndKey(img, 400)
		g.mu.Lock()
		// Only commit if the current preview is still the same one we rasterized.
		if g.currentPreview != nil && g.currentPreview.PreviewType == plugin.WoxPreviewTypeImage &&
			g.currentPreview.PreviewData == preview.PreviewData {
			g.previewImgPNG = png
			g.previewImgKey = key
			g.previewImgLoading = false
			g.dirty = true
		}
		g.mu.Unlock()
		g.requestRepaint()
	}()
}

// unwrapRemotePreview fetches the real preview via HTTP when core wrapped a
// large preview as WoxPreviewTypeRemote. The PreviewData is a relative URL
// like "/preview?sessionId=...&queryId=...&id=...". On success the returned
// WoxPreview replaces currentPreview and any image/text handling runs as if
// the preview had arrived inline.
func (g *gpuUIImpl) unwrapRemotePreview(ctx context.Context, remote plugin.WoxPreview) {
	g.mu.Lock()
	g.previewUnwrapping = true
	g.mu.Unlock()

	go func() {
		port := GetUIManager().serverPort
		url := "http://127.0.0.1:" + strconv.Itoa(port) + remote.PreviewData

		resp, err := http.Get(url)
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("unwrap remote preview: %s", err.Error()))
			g.mu.Lock()
			g.previewUnwrapping = false
			g.mu.Unlock()
			return
		}
		defer resp.Body.Close()

		var real plugin.WoxPreview
		if err := json.NewDecoder(resp.Body).Decode(&real); err != nil {
			logger.Error(ctx, fmt.Sprintf("decode remote preview: %s", err.Error()))
			g.mu.Lock()
			g.previewUnwrapping = false
			g.mu.Unlock()
			return
		}

		g.mu.Lock()
		// Only apply if the user hasn't moved to a different result meanwhile.
		if g.currentPreview != nil && g.currentPreview.PreviewType == plugin.WoxPreviewTypeRemote &&
			g.currentPreview.PreviewData == remote.PreviewData {
			g.currentPreview = &real
			g.previewUnwrapping = false
			g.dirty = true
			previewType := real.PreviewType
			g.mu.Unlock()
			switch previewType {
			case plugin.WoxPreviewTypeImage:
				g.rasterizePreviewImageInBackground(real)
			}
			g.requestRepaint()
			return
		}
		g.previewUnwrapping = false
		g.mu.Unlock()
	}()
}

// handleScroll routes mouse-wheel scrolling to the preview panel when the
// cursor is over the preview area, otherwise to the result list. This mirrors
// the Flutter launcher where each pane scrolls independently.
func (g *gpuUIImpl) handleScroll(ctx context.Context, ev ui.Event) {
	g.mu.Lock()
	hasPreview := g.currentPreview != nil && !g.currentPreview.IsEmpty()
	// Preview occupies the right portion of the window (after the window padding).
	innerW := float32(ev.Width)
	previewStartX := innerW * (1 - g.resultPreviewRatio)
	if !hasPreview {
		previewStartX = innerW + 1 // never match
	}

	if hasPreview && ev.X >= previewStartX {
		// Scroll preview. DeltaY < 0 means scroll up (wheel up).
		delta := -ev.DeltaY * 40
		g.previewScrollOffset += delta
		if g.previewScrollOffset < 0 {
			g.previewScrollOffset = 0
		}
		g.dirty = true
		g.mu.Unlock()
		g.requestRepaint()
		return
	}

	// Scroll result list.
	delta := -ev.DeltaY * 40
	g.scrollOffset += delta
	if g.scrollOffset < 0 {
		g.scrollOffset = 0
	}
	// Clamp to the valid scroll range so the thumb never overshoots.
	maxScroll := g.maxScrollOffset()
	if maxScroll > 0 && g.scrollOffset > maxScroll {
		g.scrollOffset = maxScroll
	}
	g.dirty = true
	g.mu.Unlock()
	g.requestRepaint()
}

// viewportHeight returns the vertical space available for the result list,
// excluding the query box, gaps, padding, and toolbar (when visible).
func (g *gpuUIImpl) viewportHeight() float32 {
	t := g.theme
	pad := t.WindowPadding * 2
	queryBoxH := t.QueryBoxHeight
	gap := float32(12)
	toolbarH := float32(0)
	if g.toolbarVisible() {
		toolbarH = t.ToolbarHeight
	}
	h := g.committedWindowHeight - queryBoxH - gap - toolbarH - pad
	if h < 0 {
		h = 0
	}
	return h
}

// maxScrollOffset returns the maximum valid scroll offset (contentH - viewportH).
func (g *gpuUIImpl) maxScrollOffset() float32 {
	if len(g.results) == 0 {
		return 0
	}
	itemH := g.theme.ListItemHeight
	contentH := float32(len(g.results)) * itemH
	viewportH := g.viewportHeight()
	maxScroll := contentH - viewportH
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

// handleClick processes mouse clicks. Currently only toolbar action buttons
// are clickable; the rest of the window is passive (no list-item click handling
// yet). toolbarActionRects is populated by buildToolbarActions during the
// last layout pass.
func (g *gpuUIImpl) handleClick(ctx context.Context, ev ui.Event) {
	g.mu.Lock()
	rects := g.toolbarActionRects
	actions := g.buildToolbarActionsLocked()
	g.mu.Unlock()

	if len(rects) == 0 || len(actions) == 0 {
		return
	}

	// Each rect is [x0, y0, x1, y1] in DIP.
	for i := 0; i+3 < len(rects) && i/4 < len(actions); i += 4 {
		if ev.X >= rects[i] && ev.X <= rects[i+2] &&
			ev.Y >= rects[i+1] && ev.Y <= rects[i+3] {
			action := actions[i/4]
			if action.Action != nil {
				action.Action()
			}
			return
		}
	}
}

// buildToolbarActionsLocked is a lock-free version of buildToolbarActions for
// use when g.mu is already held. Returns the same action slice.
func (g *gpuUIImpl) buildToolbarActionsLocked() []ui.ToolbarAction {
	if g.selectedIdx < 0 || g.selectedIdx >= len(g.results) {
		return nil
	}
	result := g.results[g.selectedIdx]
	if result.Actions == nil {
		return nil
	}

	var actions []ui.ToolbarAction
	for _, a := range result.Actions {
		if a.Hotkey == "" {
			continue
		}
		action := a
		actions = append(actions, ui.ToolbarAction{
			Label:  a.Name,
			Hotkey: a.Hotkey,
			Action: func() {
				actionCtx := util.NewTraceContext()
				g.mu.Lock()
				queryId := g.currentQuery.Id
				sessionId := g.currentSessionId
				g.mu.Unlock()
				_ = plugin.GetPluginManager().ExecuteAction(actionCtx, sessionId, queryId, result.Id, action.Id)
				g.HideApp(actionCtx)
			},
		})
	}
	return actions
}

// toolbarVisible returns true when the toolbar should be rendered. The
// toolbar shows when there are results or a plugin toolbar message, unless
// the caller explicitly requested HideToolbar. Mirrors the Flutter launcher's
// isToolbarVisible logic.
func (g *gpuUIImpl) toolbarVisible() bool {
	if g.hideToolbar {
		return false
	}
	if g.currentToolbarMsg != nil {
		return true
	}
	return len(g.results) > 0
}

// toolbarHeight returns the height to allocate for the toolbar (0 when
// not visible).
func (g *gpuUIImpl) toolbarHeight() float32 {
	if !g.toolbarVisible() {
		return 0
	}
	return g.theme.ToolbarHeight
}

// syncScrollOffsetWithSelection adjusts scrollOffset so the selected item
// stays within the visible viewport after arrow-key navigation. Mirrors the
// Flutter launcher's syncScrollPositionWithActiveIndex. Must be called with
// g.mu held.
func (g *gpuUIImpl) syncScrollOffsetWithSelection() {
	if g.selectedIdx < 0 || len(g.results) == 0 {
		return
	}
	itemH := g.theme.ListItemHeight
	viewportH := g.viewportHeight()
	visibleCount := int(viewportH / itemH)
	if visibleCount <= 0 {
		visibleCount = 1
	}

	firstVisible := int(g.scrollOffset / itemH)
	lastVisible := firstVisible + visibleCount

	if g.selectedIdx < firstVisible {
		// Selected is above the viewport — scroll up to align it at the top.
		g.scrollOffset = float32(g.selectedIdx) * itemH
	} else if g.selectedIdx >= lastVisible {
		// Selected is below the viewport — scroll down to bring it into view.
		g.scrollOffset = float32(g.selectedIdx-visibleCount+1) * itemH
	}

	// Clamp to valid range.
	maxScroll := g.maxScrollOffset()
	if g.scrollOffset < 0 {
		g.scrollOffset = 0
	}
	if g.scrollOffset > maxScroll {
		g.scrollOffset = maxScroll
	}
	g.dirty = true
}
