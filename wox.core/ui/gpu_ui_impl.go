package ui

import (
	"context"
	"fmt"
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
	renderer   *ui.WindowsRenderer
	engine     *ui.LayoutEngine
	theme      ui.Theme

	// Launcher state
	visible     bool
	queryValue  string
	results     []plugin.QueryResultUI
	selectedIdx int
	scrollOffset float32
	currentQuery plugin.Query
	currentSessionId string

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
	// Apply the current theme (may have been set by ChangeTheme before Run)
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
		g.visible = true
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
			}
			g.mu.Unlock()
		case ui.KeyUp:
			g.mu.Lock()
			if g.selectedIdx > 0 {
				g.selectedIdx--
			}
			g.mu.Unlock()
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
		g.mu.Unlock()
		// Trigger a new query
		g.triggerQuery(ctx)

	case ui.EventFocusLost:
		g.mu.Lock()
		wasVisible := g.visible
		g.visible = false
		g.mu.Unlock()
		if wasVisible {
			GetUIManager().PostOnHide(ctx)
		}
	}

	// Handle backspace outside the switch since it comes as KeyPress not TextInput
	if ev.Type == ui.EventKeyPress && ev.Key == ui.KeyBackspace {
		g.mu.Lock()
		if len(g.queryValue) > 0 {
			runes := []rune(g.queryValue)
			g.queryValue = string(runes[:len(runes)-1])
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

	// Convert query results to list items with rasterized icons
	items := make([]ui.ListItem, len(g.results))
	for i, r := range g.results {
		item := ui.ListItem{
			Title:    r.Title,
			Subtitle: r.SubTitle,
		}
		// Rasterize the result icon to PNG for Direct2D rendering
		if !r.Icon.IsEmpty() {
			item.IconPNG = ui.RasterizeWoxImageWithSize(r.Icon, 36)
		}
		items[i] = item
	}

	root := ui.VBox{
		Padding: 16,
		Gap:     12,
		Children: []ui.Widget{
			ui.TextBox{
				ID:           "query",
				Placeholder:  "Type to search...",
				FontSize:     16,
				FontColor:    ui.ColorTextPrimary,
				BgColor:      ui.RGBA(1, 1, 1, 0.06),
				CornerRadius: 8,
				CursorColor:  ui.ColorCursor,
				Value:        g.queryValue,
				Focused:      true,
			},
			ui.ListBox{
				ID:            "results",
				ItemHeight:    48,
				Items:         items,
				ScrollOffset:  g.scrollOffset,
				Selected:      g.selectedIdx,
				SelectedColor: &ui.ColorSelected,
			},
		},
	}

	result := g.engine.Layout(root, 800, 400)
	return &result.Commands
}

// triggerQuery starts a new plugin query with the current query text.
func (g *gpuUIImpl) triggerQuery(ctx context.Context) {
	go func() {
		queryCtx := util.NewTraceContext()
		queryId := uuid.NewString()
		plainQuery := common.PlainQuery{
			QueryId:   queryId,
			QueryType: plugin.QueryTypeInput,
			QueryText: g.queryValue,
		}
		q, ownerPlugin, err := plugin.GetPluginManager().NewQuery(queryCtx, plainQuery)
		if err != nil {
			logger.Error(queryCtx, fmt.Sprintf("gpuUI query error: %s", err.Error()))
			return
		}

		g.mu.Lock()
		g.currentQuery = q
		g.currentSessionId = util.GetContextSessionId(queryCtx)
		g.results = nil
		g.selectedIdx = 0
		g.scrollOffset = 0
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
				g.results = allResults
				g.mu.Unlock()
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
				g.results = allResults
				g.mu.Unlock()
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
				g.results = allResults
				g.mu.Unlock()
				logger.Info(queryCtx, fmt.Sprintf("gpuUI query done: %d results", len(allResults)))
				return
			case <-time.After(time.Minute):
				logger.Info(queryCtx, fmt.Sprintf("gpuUI query timeout: %s", g.queryValue))
				return
			}
		}
	}()
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
	g.mu.Unlock()
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
	g.renderer.Hide()
	GetUIManager().PostOnHide(ctx)
}

func (g *gpuUIImpl) ShowApp(ctx context.Context, showContext common.ShowContext) {
	GetUIManager().RefreshActiveWindowSnapshot(ctx)

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
	g.mu.Unlock()
	g.renderer.Show()
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
	g.mu.Unlock()
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
	g.mu.Unlock()
}