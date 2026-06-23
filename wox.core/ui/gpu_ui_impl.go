package ui

import (
	"context"
	"fmt"
	"runtime/debug"
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

// gpuUIImpl implements common.UI using the native Direct2D renderer.
// Launcher-related methods (ShowApp/HideApp/ChangeQuery/UpdateResult etc.)
// are handled directly via the native ui package.
// Settings/onboarding/screenshot methods are forwarded to the existing
// WebSocket-based uiImpl (which talks to the Flutter settings process).
type gpuUIImpl struct {
	mu sync.Mutex

	// Native renderer (Direct2D on Windows)
	renderer *ui.WindowsRenderer
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
}

// NewGpuUI creates a native launcher UI config. The actual Direct2D window
// is created lazily in Run() to ensure it happens on the OS main thread.
func NewGpuUI(ctx context.Context, wsUI *uiImpl) (*gpuUIImpl, error) {
	g := &gpuUIImpl{
		theme: ui.DefaultTheme(),
		wsUI:  wsUI,
	}
	return g, nil
}

// Run starts the native message loop. Blocks until the window is closed.
// Must be called on the OS main thread (via mainthread.Call).
// The window starts hidden if HideOnStart is enabled in settings.
func (g *gpuUIImpl) Run(ctx context.Context) {
	// Apply the user's currently selected theme before creating the renderer.
	// Manager.Start loads themes into the registry but does not push the active
	// theme to gpuUIImpl, so g.theme would stay at DefaultTheme (opaque) and
	// Clear would never go transparent, hiding the Mica backdrop.
	currentTheme := GetUIManager().GetCurrentTheme(ctx)
	if currentTheme.ThemeId != "" {
		g.theme = ui.ThemeFromWoxTheme(GetUIManager().resolvePlatformTheme(ctx, currentTheme))
	}
	theme := g.theme

	// Create the renderer now — this must be on the OS main thread.
	renderer, err := ui.NewWindowsRenderer(800, 400, theme)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to create native renderer: %s", err.Error()))
		return
	}
	g.renderer = renderer
	g.engine = &ui.LayoutEngine{
		Theme:    theme,
		Measurer: renderer.TextMeasurer(),
	}

	// Set up event handler
	ui.SetEventHandler(g.handleEvent)

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if !woxSetting.HideOnStart.Get() {
		g.renderer.Show()
		g.mu.Lock()
		g.visible = true
		g.dirty = true
		g.mu.Unlock()
		g.requestRepaint()
	}

	g.renderer.RunMessageLoop(func() *ui.CommandList {
		return g.buildAndRender(ctx)
	})
}

// handleEvent processes native input events.
func (g *gpuUIImpl) handleEvent(ev ui.Event) {
	ctx := util.NewTraceContext()

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
			}
			g.mu.Unlock()
			g.requestRepaint()
		case ui.KeyUp:
			g.mu.Lock()
			if g.selectedIdx > 0 {
				g.selectedIdx--
				g.dirty = true
			}
			g.mu.Unlock()
			g.requestRepaint()
		case ui.KeyEnter:
			g.mu.Lock()
			if g.selectedIdx >= 0 && g.selectedIdx < len(g.results) {
				result := g.results[g.selectedIdx]
				g.mu.Unlock()
				g.executeResult(ctx, result)
				return
			}
			g.mu.Unlock()
		}

	case ui.EventTextInput:
		g.mu.Lock()
		g.queryValue += ev.Text
		g.dirty = true
		g.mu.Unlock()
		// Trigger a new query
		g.triggerQuery(ctx)

	case ui.EventFocusLost:
		g.mu.Lock()
		wasVisible := g.visible
		g.visible = false
		g.mu.Unlock()
		if wasVisible {
			g.releaseHiddenMemory()
			GetUIManager().PostOnHide(ctx)
		}
	}

	// Handle backspace outside the switch since it comes as KeyPress not TextInput
	if ev.Type == ui.EventKeyPress && ev.Key == ui.KeyBackspace {
		g.mu.Lock()
		if len(g.queryValue) > 0 {
			runes := []rune(g.queryValue)
			g.queryValue = string(runes[:len(runes)-1])
			g.dirty = true
			g.mu.Unlock()
			g.triggerQuery(ctx)
		} else {
			g.mu.Unlock()
		}
	}
}

// buildAndRender generates the draw command list from current state.
func (g *gpuUIImpl) buildAndRender(ctx context.Context) *ui.CommandList {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.visible {
		return nil
	}
	// Only render when something actually changed. Without this gate every
	// message-loop wakeup would do a full Clear + redraw, making the selected
	// result background flicker during fast typing.
	if !g.dirty {
		return nil
	}
	g.dirty = false

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

	root := ui.VBox{
		Padding: t.WindowPadding,
		Gap:     12,
		Children: []ui.Widget{
			ui.TextBox{
				ID:           "query",
				Placeholder:  "Type to search...",
				FontSize:     t.FontSize,
				FontColor:    t.QueryBoxFontColor,
				BgColor:      t.QueryBoxBg,
				CornerRadius: t.QueryBoxRadius,
				CursorColor:  t.QueryBoxCursorColor,
				Value:        g.queryValue,
				Focused:      true,
			},
			ui.ListBox{
				ID:            "results",
				ItemHeight:    t.ListItemHeight,
				Items:         items,
				ScrollOffset:  g.scrollOffset,
				Selected:      g.selectedIdx,
				SelectedColor: &t.SelectedBg,
			},
		},
	}

	result := g.engine.Layout(root, float32(winW), float32(winH))
	g.lastResultCount = resultCount
	g.lastWinW = winW
	g.lastWinH = winH
	g.lastQueryText = g.queryValue
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
	lastCommittedHeight := g.committedWindowHeight
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
	total := queryBoxH + gap + resultH + pad
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
		for i := range results {
			r := &results[i]
			if r.Icon.IsEmpty() {
				continue
			}
			// RasterizeWoxImageWithSizeAndKey caches by hash+size, so
			// repeated calls for the same icon are cheap (map lookup).
			png, key := ui.RasterizeWoxImageWithSizeAndKey(r.Icon, 36)
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
		g.dirty = true
		g.mu.Unlock()

		// Required: lifecycle handling must run before Query
		plugin.GetPluginManager().HandleQueryLifecycle(queryCtx, q, ownerPlugin)

		resultChan, fallbackReadyChan, doneChan := plugin.GetPluginManager().Query(queryCtx, q)

		var allResults []plugin.QueryResultUI
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
				g.results = allResults
				g.dirty = true
				g.mu.Unlock()
				g.rasterizeIconsInBackground(allResults)
				g.resizeWindowHeight()
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
	g.mu.Unlock()
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
	g.wsUI.ShowToolbarMsg(ctx, msg)
}

func (g *gpuUIImpl) ClearToolbarMsg(ctx context.Context, toolbarMsgId string) {
	g.wsUI.ClearToolbarMsg(ctx, toolbarMsgId)
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
}
