package launcher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"wox/ui/contract"
	"wox/ui/coreclient"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	defaultWidth              = 760
	defaultMaxResult          = 10
	queryBoxHeight            = 55
	queryEditorHeight         = 38
	footerHeight              = 40
	resultRowBaseHeight       = 50
	resultRowGap              = 0
	queryResizeSettleDuration = 80 * time.Millisecond
)

// BackendFactory binds one launcher session to the embedding core backend.
type BackendFactory func(sessionID string) coreclient.Backend

func resultRowHeightForPalette(palette uiPalette) float32 {
	return resultRowBaseHeight + palette.resultItemPadding.Top + palette.resultItemPadding.Bottom
}

const (
	launcherWindowID woxui.WindowID = "wox.launcher"
	settingsWindowID woxui.WindowID = "wox.settings"
)

// App owns the launcher window, query state, and Wox core protocol client.
type App struct {
	mu                     sync.RWMutex
	previewLifecycleMu     sync.Mutex
	terminalSubscriptionMu sync.Mutex
	tooltipMu              sync.Mutex
	terminalSubscribed     string

	isDev         bool
	isPrimary     bool
	instanceName  string
	sessionID     string
	windowID      woxui.WindowID
	services      contract.Services
	clientFactory BackendFactory
	client        coreclient.Backend
	windows       *woxui.WindowManager
	instances     *appInstanceRegistry
	primary       *App
	destroyOnce   sync.Once
	unsubscribers []func()
	lifecycleCtx  context.Context
	cancel        context.CancelFunc
	destroyed     bool
	launcher      *woxui.ManagedWindow
	settingsView  *woxui.ManagedWindow
	window        *woxui.Window
	host          *woxwidget.Host
	settingsHost  *woxwidget.Host

	query                 plainQuery
	queryContext          queryContext
	queryContextKnown     bool
	editor                *woxui.TextEditor
	results               []queryResult
	resultsQueryID        string
	queryTransitionTimer  *time.Timer
	queryResizeTimer      *time.Timer
	queryResizeRevision   uint64
	pendingResults        bool
	selected              int
	hoveredResult         int
	resultScroll          scrollController
	resultScrollDetached  bool
	layout                queryLayout
	refinements           []queryRefinement
	refinementOpen        bool
	refinementScope       string
	completionHint        *queryCompletionHint
	toolbarMsg            *toolbarMessage
	toolbarRevision       uint64
	form                  *formState
	requirementForm       *requirementFormState
	triggerConflict       *triggerConflictPreviewState
	themeEditor           *themeEditorPreviewState
	chatPreview           *chatPreviewState
	webViewPreviewData    string
	webViewPreviewError   string
	chatFullscreen        bool
	actionPanel           bool
	actionSelected        int
	actionSelectionKey    string
	actionFilter          *woxui.TextEditor
	visible               bool
	show                  showAppParams
	settingsOpen          bool
	settings              settingsData
	settingsCtx           settingWindowContext
	settingTab            string
	settingRow            int
	settingNote           string
	settingSaving         bool
	settingEditKey        string
	settingEditor         *woxui.TextEditor
	settingSearchEditor   *woxui.TextEditor
	settingSearchFocused  bool
	settingSearchPanel    bool
	settingSearchSelected int
	settingSearchPlugins  []pluginSettingsPlugin
	settingSearchLoading  bool
	settingSearchLoaded   bool
	settingSearchError    string
	settingChoicePicker   *settingChoicePickerState
	choiceTooltipRevision uint64
	settingLanguages      []settingChoice
	updateChannelVersions []updateChannelVersion
	updateChannelsLoading bool
	systemFontFamilies    []string
	systemFontsLoading    bool
	systemFontsLoaded     bool
	systemFontsError      string
	plugins               []pluginSettingsPlugin
	pluginsLoading        bool
	pluginsLoaded         bool
	pluginsError          string
	pluginSelected        int
	pluginSearchEditor    *woxui.TextEditor
	pluginSearchFocused   bool
	pluginFilters         pluginFilterState
	pluginFilterOpen      bool
	pluginDetailTab       string
	pluginForm            *pluginSettingsFormState
	pluginsStore          bool
	pluginOperation       string
	pluginOperationError  string
	pluginUninstallArmed  string
	hotkeySettingsForm    *formFieldsState
	settingsHotkeyFocus   bool
	hotkeyRecording       *hotkeyRecordingState
	hotkeyAppCandidates   []ignoredHotkeyApp
	hotkeyAppsLoading     bool
	hotkeyAppsLoaded      bool
	hotkeyAppsError       string
	themes                []themeSettingsTheme
	themesMode            string
	themesLoading         bool
	themesLoaded          bool
	themesError           string
	themeSelected         int
	themeSearchEditor     *woxui.TextEditor
	themeSearchFocused    bool
	themeDetailTab        string
	themeOperation        string
	themeUninstallArmed   string
	themeWallpaperPath    string
	themeWallpaperImage   *woxui.Image
	themeWallpaperBlurred *woxui.Image
	themeWallpaperLoading bool
	themeWallpaperLoadID  uint64
	aiSettingsForm        *formFieldsState
	aiProviderCatalog     []aiProviderInfo
	aiProvidersLoading    bool
	aiProvidersLoaded     bool
	aiProvidersError      string
	tableEditor           *formTableEditorState
	modelManager          *modelManagerState
	usageStats            usageStatsData
	usagePeriod           string
	usageLoading          bool
	usageLoaded           bool
	usageError            string
	usageRevision         uint64
	aboutVersion          string
	aboutLoading          bool
	aboutLoaded           bool
	aboutError            string
	privacySample         string
	privacyError          string
	dataBackups           []backupInfo
	dataLocation          string
	dataLoading           bool
	dataLoaded            bool
	dataBusy              string
	dataError             string
	dataRestoreArmed      string
	dataPendingLocation   string
	dataClearLogsArmed    bool
	runtimeStatuses       []runtimeStatus
	runtimeLoading        bool
	runtimeLoaded         bool
	runtimeError          string
	runtimeRestarting     string
	runtimeRevision       uint64
	cloudAccount          cloudAccountStatus
	cloudSync             cloudSyncStatus
	cloudBillingPlan      cloudBillingPlan
	cloudBillingLoaded    bool
	cloudDevices          cloudDeviceList
	cloudLoading          bool
	cloudLoaded           bool
	cloudBusy             string
	cloudError            string
	cloudRevision         uint64
	cloudForm             *cloudFormState
	cloudActionMenu       string
	cloudPlugins          []pluginSettingsPlugin
	glanceItem            *glanceItem
	glanceLoading         bool
	glanceRevision        uint64
	glanceTooltipRevision uint64
	glanceTimer           *time.Timer
	glanceCatalog         []glanceCatalogItem
	glanceCatalogLoading  bool
	glanceCatalogLoaded   bool
	glanceCatalogError    string
	palette               uiPalette
	translations          map[string]string
	images                map[string]*woxui.Image
	imageRequested        map[string]string
	imageLastUsed         map[string]uint64
	imageUseSequence      uint64
	imageErrors           map[string]string
	remotePreviews        map[string]queryPreview
	previewRequests       map[string]bool
	filePreviews          map[string]filePreviewContent
	fileRequests          map[string]bool
	previewLayouts        map[string]woxwidget.TextBlockLayout
	terminalPreview       *terminalPreviewState
	aiModels              []aiModel
	aiModelsLoading       bool
	aiModelsLoaded        bool
	aiModelsError         string
	aiSkills              []chatSkill
	aiSkillsLoading       bool
	aiSkillsLoaded        bool
	aiSkillsError         string
}

// New creates a launcher whose core services and transitional backend are supplied by the process composition root.
func New(isDev bool, services contract.Services, clientFactory BackendFactory) *App {
	windows := woxui.NewWindowManager()
	instances := newAppInstanceRegistry()
	app := newApp(isDev, services, clientFactory, windows, instances, nil, true, "", launcherWindowID)
	app.primary = app
	instances.registerPrimary(app)
	return app
}

// newApp builds isolated launcher state while sharing only process-wide window and message infrastructure.
func newApp(isDev bool, services contract.Services, clientFactory BackendFactory, windows *woxui.WindowManager, instances *appInstanceRegistry, primary *App, isPrimary bool, instanceName string, windowID woxui.WindowID) *App {
	sessionID := coreclient.NewID()
	lifecycleCtx, cancel := context.WithCancel(context.Background())
	if windowID == "" {
		windowID = woxui.WindowID("wox.instance." + sessionID)
	}
	app := &App{
		isDev:           isDev,
		isPrimary:       isPrimary,
		instanceName:    instanceName,
		sessionID:       sessionID,
		windowID:        windowID,
		services:        services,
		clientFactory:   clientFactory,
		windows:         windows,
		instances:       instances,
		primary:         primary,
		lifecycleCtx:    lifecycleCtx,
		cancel:          cancel,
		query:           newInputQuery(""),
		editor:          woxui.NewTextEditor(""),
		selected:        -1,
		hoveredResult:   -1,
		settingTab:      "general",
		usagePeriod:     "30d",
		palette:         defaultPalette(),
		translations:    map[string]string{},
		images:          map[string]*woxui.Image{},
		imageRequested:  map[string]string{},
		imageLastUsed:   map[string]uint64{},
		imageErrors:     map[string]string{},
		remotePreviews:  map[string]queryPreview{},
		previewRequests: map[string]bool{},
		filePreviews:    map[string]filePreviewContent{},
		fileRequests:    map[string]bool{},
		previewLayouts:  map[string]woxwidget.TextBlockLayout{},
		show: showAppParams{
			WindowWidth:    defaultWidth,
			MaxResultCount: defaultMaxResult,
			StartPage:      "mru",
		},
	}
	app.unsubscribers = append(app.unsubscribers, app.windows.SubscribeMessages(app.windowID, settingsChangedTopic, app.onSharedSettingsChanged))
	return app
}

// Start connects to core and creates the hidden native window on the UI runtime thread.
func (a *App) Start() error {
	return a.start()
}

// start initializes one independent launcher session against the shared window runtime.
func (a *App) start() error {
	if a.services == nil {
		return errors.New("core lifecycle services are required")
	}
	if a.clientFactory == nil {
		return errors.New("core backend factory is required")
	}
	a.client = a.clientFactory(a.sessionID)
	connectContext, cancelConnect := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelConnect()
	if err := a.client.Connect(connectContext); err != nil {
		return err
	}
	if err := a.reloadTheme(); err != nil {
		log.Printf("load Wox theme, using fallback palette: %v", err)
	}
	if err := a.reloadSettings(); err != nil {
		log.Printf("load Wox settings, using fallback launcher behavior: %v", err)
	}
	if err := a.reloadTranslations(); err != nil {
		log.Printf("load Wox translations, using source labels: %v", err)
	}

	host := woxwidget.NewHost(a.buildLauncher)
	launcher, _, err := a.windows.Open(a.windowID, woxui.WindowOptions{
		Title:     "Wox",
		Size:      woxui.Size{Width: float32(a.show.WindowWidth), Height: queryBoxHeight + a.palette.appPadding.Top + a.palette.appPadding.Bottom + footerHeight},
		OnFrame:   host.Frame,
		OnPointer: host.Pointer,
		OnKey: func(event woxui.KeyEvent) bool {
			if host.Key(event) {
				return true
			}
			return a.onKey(event)
		},
		OnTextInput: func(event woxui.TextInputEvent) {
			if !host.TextInput(event) {
				a.onTextInput(event)
			}
		},
		OnFocus: a.onFocus,
		OnClosed: func() {
			host.Dispose()
			a.onLauncherWindowClosed()
		},
	})
	if err != nil {
		_ = a.client.Close()
		return err
	}
	a.launcher = launcher
	a.window = launcher.Window()
	a.host = host
	host.Attach(a.window)
	if err := a.window.SetAppearance(themeColorIsDark(a.palette.background)); err != nil {
		return fmt.Errorf("apply Wox UI appearance: %w", err)
	}
	if err := a.window.SetFontFamily(a.settings.AppFontFamily); err != nil {
		return fmt.Errorf("apply Wox UI font: %w", err)
	}
	if err := a.window.SetTextInputState(woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: 130, Y: 29, Width: 1, Height: 24}}); err != nil {
		return err
	}

	lifecycleContext, cancelLifecycle := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelLifecycle()
	if a.isPrimary {
		if err := a.services.Ready(lifecycleContext, a.sessionID); err != nil {
			return fmt.Errorf("notify Wox core that Go UI is ready: %w", err)
		}
	} else if err := a.services.RegisterInstance(lifecycleContext, a); err != nil {
		return fmt.Errorf("register secondary Wox UI instance: %w", err)
	}
	return nil
}

// Close releases the protocol connection after the final native window closes.
func (a *App) Close() error {
	if !a.isPrimary {
		a.mu.RLock()
		launcher := a.launcher
		a.mu.RUnlock()
		if launcher != nil {
			return launcher.Close()
		}
		a.destroySecondary()
		return nil
	}

	a.mu.Lock()
	a.destroyed = true
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	var closeErr error
	if a.windows != nil {
		closeErr = a.windows.CloseAll()
	}
	a.unsubscribeAll()
	if a.client == nil {
		return closeErr
	}
	clientErr := a.client.Close()
	if closeErr != nil {
		return closeErr
	}
	return clientErr
}

func (a *App) showWindow(params showAppParams) error {
	a.mu.Lock()
	if params.WindowWidth <= 0 {
		params.WindowWidth = defaultWidth
	}
	if params.MaxResultCount <= 0 {
		params.MaxResultCount = defaultMaxResult
	}
	a.show = params
	if params.SelectAll {
		a.editor.SelectAll()
	}
	a.actionPanel = false
	a.actionSelected = 0
	a.actionSelectionKey = ""
	a.actionFilter = nil
	a.form = nil
	a.visible = true
	queryEmpty := a.query.QueryText == ""
	launcher := a.launcher
	a.mu.Unlock()
	if launcher == nil {
		return errors.New("launcher window is not initialized")
	}
	a.reconcileSelectedPreview()
	a.restoreQueryTextInput()

	if err := a.window.SetHideOnBlur(params.HideOnBlur); err != nil {
		return err
	}
	if err := a.applyWindowBoundsAtShowPosition(); err != nil {
		return err
	}
	if _, err := launcher.Show(); err != nil {
		return err
	}
	if err := a.notifyShown(); err != nil {
		return err
	}
	if queryEmpty && params.StartPage == "mru" {
		return a.requestMRU()
	}
	go a.refreshGlance("windowShown", "", nil)
	return nil
}

func (a *App) hideWindow(notify bool) error {
	a.mu.Lock()
	if !a.visible {
		a.mu.Unlock()
		return nil
	}
	a.actionPanel = false
	a.actionSelected = 0
	a.actionSelectionKey = ""
	a.actionFilter = nil
	a.form = nil
	a.visible = false
	a.stopGlanceLocked(false)
	launcher := a.launcher
	a.mu.Unlock()
	if launcher == nil {
		return errors.New("launcher window is not initialized")
	}
	a.reconcileSelectedPreview()
	a.mu.Lock()
	a.requirementForm = nil
	a.triggerConflict = nil
	a.themeEditor = nil
	a.mu.Unlock()
	a.resetChatPreview()
	if err := launcher.Hide(); err != nil {
		return err
	}
	if notify {
		return a.notifyHidden()
	}
	return nil
}

func (a *App) onFocus(event woxui.FocusEvent) {
	if event.Active {
		return
	}
	a.mu.Lock()
	if !a.visible {
		a.mu.Unlock()
		return
	}
	hideOnBlur := a.show.HideOnBlur
	launcher := a.launcher
	if hideOnBlur {
		a.visible = false
		a.stopGlanceLocked(false)
	}
	a.mu.Unlock()
	if hideOnBlur {
		a.reconcileSelectedPreview()
		if launcher != nil {
			_ = launcher.Hide()
		}
		a.resetChatPreview()
	}
	go func() {
		if hideOnBlur {
			if err := a.notifyHidden(); err != nil {
				log.Printf("notify Wox core after blur hide: %v", err)
			}
			return
		}
		if err := a.notifyFocusLost(); err != nil {
			log.Printf("notify Wox core after focus loss: %v", err)
		}
	}()
}

func (a *App) lifecycleContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

func (a *App) notifyShown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return a.services.Shown(ctx, a.sessionID)
}

func (a *App) notifyHidden() error {
	ctx, cancel := a.lifecycleContext()
	defer cancel()
	return a.services.Hidden(ctx, a.sessionID)
}

func (a *App) notifyFocusLost() error {
	ctx, cancel := a.lifecycleContext()
	defer cancel()
	return a.services.FocusLost(ctx, a.sessionID)
}

func (a *App) notifySettingViewChanged(inSettingView bool) error {
	ctx, cancel := a.lifecycleContext()
	defer cancel()
	return a.services.SettingViewChanged(ctx, a.sessionID, inSettingView)
}

func (a *App) setQuery(query plainQuery) {
	if query.QueryID == "" {
		query.QueryID = coreclient.NewID()
	}
	if query.QueryType == "" {
		query.QueryType = "input"
	}
	a.mu.Lock()
	a.query = query
	a.queryContext = queryContext{}
	a.queryContextKnown = false
	a.editor.SetText(query.QueryText, false)
	a.resetQueryTransitionLocked()
	a.results = nil
	a.resultsQueryID = ""
	a.selected = -1
	a.hoveredResult = -1
	a.resultScroll.reset()
	a.resultScrollDetached = false
	a.layout = queryLayout{}
	a.stopGlanceLocked(true)
	a.refinements = nil
	a.refinementOpen = false
	a.refinementScope = ""
	a.completionHint = nil
	a.actionPanel = false
	a.actionSelected = 0
	a.actionSelectionKey = ""
	a.actionFilter = nil
	a.form = nil
	a.mu.Unlock()
	a.reconcileSelectedPreview()
	a.mu.Lock()
	a.requirementForm = nil
	a.triggerConflict = nil
	a.themeEditor = nil
	a.mu.Unlock()
	a.resetChatPreview()
	a.restoreQueryTextInput()
	_ = a.window.Invalidate()
}

func (a *App) sendCurrentQuery() error {
	a.mu.RLock()
	query := a.query
	startPage := a.show.StartPage
	skipCompletionHint := !a.settings.EnableQueryCompletionHint
	a.mu.RUnlock()
	if err := a.startTypedQuery(query, skipCompletionHint); err != nil {
		return err
	}
	if query.QueryText == "" && startPage == "mru" {
		return a.requestMRU()
	}
	return nil
}

func (a *App) requestMRU() error {
	a.mu.Lock()
	a.query = newInputQuery("")
	a.queryContext = queryContext{IsGlobalQuery: true}
	a.queryContextKnown = true
	a.editor.SetText("", false)
	queryID := a.query.QueryID
	a.resetQueryTransitionLocked()
	a.results = nil
	a.resultsQueryID = ""
	a.selected = -1
	a.hoveredResult = -1
	a.resultScroll.reset()
	a.resultScrollDetached = false
	a.layout = queryLayout{}
	a.stopGlanceLocked(true)
	a.refinements = nil
	a.refinementOpen = false
	a.refinementScope = ""
	a.completionHint = nil
	a.actionPanel = false
	a.actionSelected = 0
	a.actionSelectionKey = ""
	a.actionFilter = nil
	a.form = nil
	a.mu.Unlock()
	a.reconcileSelectedPreview()
	a.mu.Lock()
	a.requirementForm = nil
	a.triggerConflict = nil
	a.themeEditor = nil
	a.mu.Unlock()
	a.resetChatPreview()
	go a.loadTypedMRU(queryID)
	return nil
}

func (a *App) applyResults(queryID string, results []queryResult, layout *queryLayout, refinements *[]queryRefinement, context *queryContext, queryStartTimestamp int64) {
	a.mu.Lock()
	if a.destroyed || queryID == "" || queryID != a.query.QueryID {
		a.mu.Unlock()
		return
	}
	if a.isDev && a.settings.ShowPerformanceTail && a.settings.ShowPerformanceTailUIReceived && queryStartTimestamp > 0 {
		appendUIReceivedTails(results, max(int64(0), time.Now().UnixMilli()-queryStartTimestamp))
	}
	selectedID := ""
	if a.selected >= 0 && a.selected < len(a.results) {
		selectedID = a.results[a.selected].ID
	}
	for index := range results {
		if results[index].QueryID == "" {
			results[index].QueryID = queryID
		}
	}
	a.resetQueryTransitionLocked()
	a.results = results
	a.resultsQueryID = queryID
	a.hoveredResult = -1
	if layout != nil {
		enterChatMode := layout.ChatMode && !a.layout.ChatMode
		a.layout = *layout
		if enterChatMode {
			a.chatFullscreen = true
		} else if !layout.ChatMode {
			a.chatFullscreen = false
		}
	}
	if refinements != nil {
		a.applyRefinementsLocked(*refinements)
	}
	if context != nil {
		a.queryContext = *context
		a.queryContextKnown = true
	}
	glanceEligible := a.glanceEligibleLocked()
	refreshGlance := glanceEligible && a.glanceItem == nil && !a.glanceLoading
	if glanceEligible && a.glanceItem != nil && !a.glanceLoading && a.glanceTimer == nil {
		a.scheduleGlanceRefreshLocked(a.settings.PrimaryGlance)
	} else if !glanceEligible {
		a.stopGlanceLocked(true)
	}
	a.selected = selectableIndex(results, selectedID)
	if selectedID == "" || a.selected < 0 || a.results[a.selected].ID != selectedID {
		a.resultScrollDetached = false
	}
	closedActionPanel := false
	if a.actionPanel && len(unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)) == 0 {
		closedActionPanel = a.resetActionPanelLocked()
	} else if a.actionPanel {
		a.normalizeActionSelectionLocked()
	}
	a.mu.Unlock()
	a.reconcileSelectedPreview()
	if closedActionPanel {
		a.restoreQueryTextInput()
	}
	if refreshGlance {
		go a.refreshGlance("manualRefresh", "", nil)
	}
	a.scheduleQueryWindowBounds(queryID)
	_ = a.window.Invalidate()
}

// scheduleQueryWindowBounds coalesces streaming query snapshots into one resize after input settles.
func (a *App) scheduleQueryWindowBounds(queryID string) {
	a.mu.Lock()
	if a.destroyed || queryID == "" || queryID != a.query.QueryID {
		a.mu.Unlock()
		return
	}
	a.queryResizeRevision++
	revision := a.queryResizeRevision
	if a.queryResizeTimer != nil {
		a.queryResizeTimer.Stop()
	}
	a.queryResizeTimer = time.AfterFunc(queryResizeSettleDuration, func() {
		_ = woxui.Call(func() {
			a.mu.Lock()
			if a.destroyed || revision != a.queryResizeRevision || queryID != a.query.QueryID {
				a.mu.Unlock()
				return
			}
			a.queryResizeTimer = nil
			a.mu.Unlock()
			if err := a.applyWindowBounds(); err != nil {
				log.Printf("resize launcher for query results: %v", err)
			}
		})
	})
	a.mu.Unlock()
}

func (a *App) applyWindowBounds() error {
	return a.applyWindowBoundsWithPlacement(false)
}

func (a *App) applyWindowBoundsAtShowPosition() error {
	return a.applyWindowBoundsWithPlacement(true)
}

func (a *App) applyWindowBoundsWithPlacement(useShowPosition bool) error {
	a.mu.RLock()
	params := a.show
	results := append([]queryResult(nil), a.results...)
	resultCount := len(results)
	layout := a.layout
	refinementVisible := len(a.refinements) > 0 && a.refinementOpen && !params.HideQueryBox
	actionPanel := a.actionPanel
	palette := a.palette
	formHeight := formPanelHeight(a.form)
	actionCount := 0
	requirementPreview := false
	previewVisible := false
	toolbarMessageVisible := a.toolbarMsg != nil
	chatFullscreen := a.chatFullscreen
	if actionPanel && a.actionFilter != nil {
		actionCount = len(filteredActionIndices(unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg), a.actionFilter.State().Text, a.translations, a.settings.UsePinYin))
	}
	if a.selected >= 0 && a.selected < len(a.results) {
		requirementPreview = a.results[a.selected].Preview.PreviewType == "query_requirement_settings"
		previewVisible = a.results[a.selected].Preview.PreviewData != ""
	}
	a.mu.RUnlock()
	width := params.WindowWidth
	if width <= 0 {
		width = defaultWidth
	}
	maxResults := params.MaxResultCount
	if maxResults <= 0 {
		maxResults = defaultMaxResult
	}
	visibleResults := min(resultCount, maxResults)
	resultRowHeight := int(resultRowHeightForPalette(palette))
	resultVerticalPadding := int(palette.resultContainerPadding.Top + palette.resultContainerPadding.Bottom)
	queryAreaHeight := int(queryBoxHeight + palette.appPadding.Top + palette.appPadding.Bottom)
	toolbarVisible := !params.HideToolbar && !chatFullscreen && (resultCount > 0 || toolbarMessageVisible)
	height := 0
	if !params.HideQueryBox {
		height += queryAreaHeight
	}
	if refinementVisible {
		height += refinementBarHeight
	}
	if visibleResults > 0 {
		if layout.GridLayout != nil {
			height += min(gridResultsHeight(results, float32(width), layout.GridLayout), maxResults*resultRowHeight)
		} else {
			height += resultVerticalPadding + visibleResults*resultRowHeight + max(0, visibleResults-1)*resultRowGap
		}
	}
	if toolbarVisible {
		height += footerHeight
	}
	maximumResultWindowHeight := resultVerticalPadding + maxResults*resultRowHeight + max(0, maxResults-1)*resultRowGap
	if !params.HideQueryBox {
		maximumResultWindowHeight += queryAreaHeight
	}
	if refinementVisible {
		maximumResultWindowHeight += refinementBarHeight
	}
	if toolbarVisible {
		maximumResultWindowHeight += footerHeight
	}
	if previewVisible {
		height = max(height, maximumResultWindowHeight)
	}
	if requirementPreview {
		minimumHeight := 360
		if !params.HideQueryBox {
			minimumHeight += queryAreaHeight
		}
		if refinementVisible {
			minimumHeight += refinementBarHeight
		}
		if toolbarVisible {
			minimumHeight += footerHeight
		}
		height = max(height, minimumHeight)
	}
	if actionPanel {
		actionHeight := int(actionPanelBaseHeightForPalette(palette)) + max(1, min(actionCount, maxVisibleActions))*actionRowHeight
		if !params.HideQueryBox {
			actionHeight += queryAreaHeight
		}
		if refinementVisible {
			actionHeight += refinementBarHeight
		}
		if toolbarVisible {
			actionHeight += footerHeight
		}
		// Opening the action panel restores Flutter's full configured result height while still allowing larger panels to fit.
		height = max(height, maximumResultWindowHeight, actionHeight)
	}
	if formHeight > 0 {
		formWindowHeight := formHeight + 20
		if !params.HideQueryBox {
			formWindowHeight += queryAreaHeight
		}
		if refinementVisible {
			formWindowHeight += refinementBarHeight
		}
		if toolbarVisible {
			formWindowHeight += footerHeight
		}
		height = max(height, formWindowHeight)
	}
	if height <= 0 {
		height = resultRowHeight
	}
	current, err := a.window.Bounds()
	if err != nil {
		return err
	}
	x, y := launcherWindowOrigin(params, current, float32(height), useShowPosition)
	target := woxui.Rect{
		X:      x,
		Y:      y,
		Width:  float32(width),
		Height: float32(height),
	}
	if current == target {
		return nil
	}
	return a.window.SetBounds(target)
}

// launcherWindowOrigin keeps user-moved windows in place while preserving a bottom query box during height changes.
func launcherWindowOrigin(params showAppParams, current woxui.Rect, targetHeight float32, useShowPosition bool) (float32, float32) {
	if useShowPosition {
		return float32(params.Position.X), float32(params.Position.Y)
	}
	x, y := current.X, current.Y
	if params.QueryBoxAtBottom {
		y += current.Height - targetHeight
	}
	return x, y
}

func (a *App) onKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	if !a.hotkeyRecordingUsesSettingsWindow() && a.onHotkeyRecordingKey(event) {
		return true
	}
	if !a.formTableUsesSettingsWindow() && a.onFormTableKey(event) {
		return true
	}
	if a.onFormKey(event) {
		return true
	}
	if a.onRequirementFormKey(event) {
		return true
	}
	if a.onActionKey(event) {
		return true
	}
	if a.onTriggerConflictPreviewKey(event) {
		return true
	}
	if a.onThemeEditorPreviewKey(event) {
		return true
	}
	if a.onChatPreviewKey(event) {
		return true
	}
	if a.onTerminalPreviewKey(event) {
		return true
	}
	if a.onToolbarKey(event) {
		return true
	}
	if event.Key == woxui.KeyTab {
		if event.Modifiers == woxui.KeyModifierShift {
			a.autoCompleteQueryFromSelectedResult()
			return true
		}
		if event.Modifiers == 0 {
			a.acceptQueryCompletionHint()
			return true
		}
	}
	if event.Key == woxui.Key("f") && event.Modifiers.HasPrimary() && a.toggleRefinementBar() {
		return true
	}
	a.mu.Lock()
	textHandled, textChanged := a.editor.HandleKey(event)
	if textChanged {
		a.applyQueryTextChangeLocked(a.editor.State().Text)
	}
	a.mu.Unlock()
	if textHandled {
		_ = a.window.Invalidate()
		if textChanged {
			a.reconcileSelectedPreview()
			if err := a.sendCurrentQuery(); err != nil {
				log.Printf("send query after editing command: %v", err)
			}
		}
		return true
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		a.moveSelection(-a.resultNavigationColumns())
		return true
	case woxui.KeyArrowDown:
		a.moveSelection(a.resultNavigationColumns())
		return true
	case woxui.KeyArrowLeft:
		if a.resultNavigationColumns() > 1 {
			a.moveSelection(-1)
			return true
		}
	case woxui.KeyArrowRight:
		if a.resultNavigationColumns() > 1 {
			a.moveSelection(1)
			return true
		}
		return false
	case woxui.KeyEnter:
		a.activateSelected()
		return true
	case woxui.KeyEscape:
		go func() {
			if err := a.hideWindow(true); err != nil {
				log.Printf("hide launcher: %v", err)
			}
		}()
		return true
	default:
		return false
	}
	return false
}

func (a *App) resultNavigationColumns() int {
	a.mu.RLock()
	layout := a.layout.GridLayout
	a.mu.RUnlock()
	if layout == nil {
		return 1
	}
	return normalizedGridLayout(layout).Columns
}

func (a *App) onTextInput(event woxui.TextInputEvent) {
	if a.onActionTextInput(event) {
		return
	}
	if !a.formTableUsesSettingsWindow() && a.onFormTableTextInput(event) {
		return
	}
	if a.onFormTextInput(event) {
		return
	}
	if a.onRequirementFormTextInput(event) {
		return
	}
	if a.onTriggerConflictPreviewTextInput(event) {
		return
	}
	if a.onThemeEditorPreviewTextInput(event) {
		return
	}
	if a.onChatPreviewTextInput(event) {
		return
	}
	if a.onTerminalPreviewTextInput(event) {
		return
	}
	a.mu.Lock()
	committed := a.editor.HandleTextInput(event)
	if committed {
		a.applyQueryTextChangeLocked(a.editor.State().Text)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
	if committed {
		a.reconcileSelectedPreview()
		if err := a.sendCurrentQuery(); err != nil {
			log.Printf("send committed query: %v", err)
		}
	}
}

func (a *App) moveSelection(delta int) {
	a.mu.Lock()
	if len(a.results) == 0 {
		a.mu.Unlock()
		return
	}
	index := a.selected
	changed := false
	for {
		index += delta
		if index < 0 || index >= len(a.results) {
			break
		}
		if !a.results[index].IsGroup {
			a.selected = index
			a.resultScrollDetached = false
			changed = true
			a.actionPanel = false
			a.actionSelected = 0
			a.actionSelectionKey = ""
			a.actionFilter = nil
			a.chatFullscreen = false
			break
		}
	}
	a.mu.Unlock()
	if changed {
		a.reconcileSelectedPreview()
		a.restoreQueryTextInput()
	}
	_ = a.window.Invalidate()
}

func (a *App) selectResult(index int) {
	a.mu.Lock()
	closedPanel := false
	closedForm := false
	valid := false
	if index >= 0 && index < len(a.results) && !a.results[index].IsGroup {
		valid = true
		changed := a.selected != index
		a.selected = index
		closedPanel = a.resetActionPanelLocked()
		closedForm = a.form != nil
		a.form = nil
		if changed {
			a.resultScrollDetached = false
			a.chatFullscreen = false
		}
	}
	a.mu.Unlock()
	if valid {
		a.reconcileSelectedPreview()
		a.restoreQueryTextInput()
	}
	if closedPanel || closedForm {
		_ = a.applyWindowBounds()
	}
	_ = a.window.Invalidate()
}

func (a *App) hoverResult(index int, inside bool) {
	a.mu.Lock()
	if inside {
		if index >= 0 && index < len(a.results) && !a.results[index].IsGroup {
			a.hoveredResult = index
		}
	} else if a.hoveredResult == index {
		a.hoveredResult = -1
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) activateSelected() {
	a.mu.RLock()
	selected := a.selected
	a.mu.RUnlock()
	a.activateResult(selected)
}

func (a *App) activateResult(index int) {
	a.mu.RLock()
	if len(a.results) > 0 && (index < 0 || index >= len(a.results) || a.results[index].IsGroup) {
		a.mu.RUnlock()
		return
	}
	entries := unifiedActionPanelEntries(a.results, index, a.toolbarMsg)
	if len(entries) == 0 {
		a.mu.RUnlock()
		return
	}
	entry := entries[0]
	for _, candidate := range entries {
		if candidate.IsDefault {
			entry = candidate
			break
		}
	}
	a.mu.RUnlock()
	a.activateActionPanelEntry(entry)
}

func selectableIndex(results []queryResult, selectedID string) int {
	for index, result := range results {
		if selectedID != "" && result.ID == selectedID && !result.IsGroup {
			return index
		}
	}
	for index, result := range results {
		if !result.IsGroup {
			return index
		}
	}
	return -1
}

type plainQuery struct {
	QueryID          string            `json:"QueryId"`
	QueryType        string            `json:"QueryType"`
	QueryText        string            `json:"QueryText"`
	QuerySelection   selection         `json:"QuerySelection"`
	QueryRefinements map[string]string `json:"QueryRefinements"`
	ContextData      map[string]string `json:"ContextData"`
}

type selection struct {
	Type      string   `json:"Type"`
	Text      string   `json:"Text"`
	FilePaths []string `json:"FilePaths"`
}

func newInputQuery(text string) plainQuery {
	return plainQuery{
		QueryID:          coreclient.NewID(),
		QueryType:        "input",
		QueryText:        text,
		QuerySelection:   selection{FilePaths: []string{}},
		QueryRefinements: map[string]string{},
		ContextData:      map[string]string{},
	}
}

type showAppParams struct {
	SelectAll        bool     `json:"SelectAll"`
	Position         position `json:"Position"`
	WindowWidth      int      `json:"WindowWidth"`
	MaxResultCount   int      `json:"MaxResultCount"`
	LaunchMode       string   `json:"LaunchMode"`
	StartPage        string   `json:"StartPage"`
	HideQueryBox     bool     `json:"HideQueryBox"`
	HideToolbar      bool     `json:"HideToolbar"`
	QueryBoxAtBottom bool     `json:"QueryBoxAtBottom"`
	HideOnBlur       bool     `json:"HideOnBlur"`
	ShowSource       string   `json:"ShowSource"`
}

type position struct {
	Type string `json:"Type"`
	X    int    `json:"X"`
	Y    int    `json:"Y"`
}

type queryResponse struct {
	QueryID             string              `json:"QueryId"`
	Results             []queryResult       `json:"Results"`
	Refinements         []queryRefinement   `json:"Refinements"`
	Layout              queryLayout         `json:"Layout"`
	Context             queryContext        `json:"Context"`
	IsFinal             bool                `json:"IsFinal"`
	QueryStartTimestamp int64               `json:"QueryStartTimestamp"`
	ActionIconRefs      map[string]woxImage `json:"ActionIconRefs"`
}

// resolveActionIconRefs restores response-local action icons before shared widgets see the result batch.
func resolveActionIconRefs(results []queryResult, refs map[string]woxImage) {
	for resultIndex := range results {
		for actionIndex := range results[resultIndex].Actions {
			icon := &results[resultIndex].Actions[actionIndex].Icon
			if icon.ImageType != "iconref" {
				continue
			}
			if resolved, ok := refs[icon.ImageData]; ok {
				*icon = resolved
			}
		}
	}
}

type queryContext struct {
	IsGlobalQuery bool   `json:"IsGlobalQuery"`
	PluginID      string `json:"PluginId"`
}

type queryLayout struct {
	Icon                    woxImage    `json:"Icon"`
	ResultPreviewWidthRatio *float64    `json:"ResultPreviewWidthRatio"`
	GridLayout              *gridLayout `json:"GridLayout"`
	ChatMode                bool        `json:"ChatMode"`
}

type gridLayout struct {
	Columns     int      `json:"Columns"`
	ShowTitle   bool     `json:"ShowTitle"`
	ItemPadding int      `json:"ItemPadding"`
	ItemMargin  int      `json:"ItemMargin"`
	AspectRatio float64  `json:"AspectRatio"`
	Commands    []string `json:"Commands"`
}

type queryRefinement struct {
	ID           string                  `json:"Id"`
	Title        string                  `json:"Title"`
	Type         string                  `json:"Type"`
	Options      []queryRefinementOption `json:"Options"`
	DefaultValue []string                `json:"DefaultValue"`
	Hotkey       string                  `json:"Hotkey"`
	Persist      bool                    `json:"Persist"`
}

type queryRefinementOption struct {
	Value    string   `json:"Value"`
	Title    string   `json:"Title"`
	Icon     woxImage `json:"Icon"`
	Keywords []string `json:"Keywords"`
	Count    *int     `json:"Count"`
}

type queryResult struct {
	QueryID  string         `json:"QueryId"`
	ID       string         `json:"Id"`
	Title    string         `json:"Title"`
	SubTitle string         `json:"SubTitle"`
	Icon     woxImage       `json:"Icon"`
	Preview  queryPreview   `json:"Preview"`
	Tails    []resultTail   `json:"Tails"`
	Actions  []resultAction `json:"Actions"`
	IsGroup  bool           `json:"IsGroup"`
}

type resultTail struct {
	Type         string            `json:"Type"`
	Text         string            `json:"Text"`
	TextCategory string            `json:"TextCategory"`
	Image        woxImage          `json:"Image"`
	ImageWidth   *float64          `json:"ImageWidth"`
	ImageHeight  *float64          `json:"ImageHeight"`
	Tooltip      string            `json:"Tooltip"`
	ContextData  map[string]string `json:"ContextData"`
}

const uiReceivedTailTooltip = "onReceivedQueryResults elapsed since Go UI query request"

// appendUIReceivedTails replaces the UI-owned timing tail while preserving core and plugin tails.
func appendUIReceivedTails(results []queryResult, elapsed int64) {
	for index := range results {
		if results[index].IsGroup {
			continue
		}
		tails := results[index].Tails[:0]
		for _, tail := range results[index].Tails {
			if tail.Tooltip != uiReceivedTailTooltip {
				tails = append(tails, tail)
			}
		}
		results[index].Tails = append(tails, resultTail{Type: "text", Text: fmt.Sprintf("%dms", elapsed), TextCategory: "default", Tooltip: uiReceivedTailTooltip})
	}
}

type queryPreview struct {
	PreviewType        string            `json:"PreviewType"`
	PreviewData        string            `json:"PreviewData"`
	PreviewOverlayData string            `json:"PreviewOverlayData"`
	PreviewTags        []previewTag      `json:"PreviewTags"`
	PreviewProperties  map[string]string `json:"PreviewProperties"`
	ScrollPosition     string            `json:"ScrollPosition"`
}

type previewTag struct {
	Label   string `json:"Label"`
	Tooltip string `json:"Tooltip"`
}

type resultAction struct {
	ID                     string           `json:"Id"`
	Type                   string           `json:"Type"`
	Name                   string           `json:"Name"`
	Icon                   woxImage         `json:"Icon"`
	IsDefault              bool             `json:"IsDefault"`
	PreventHideAfterAction bool             `json:"PreventHideAfterAction"`
	Hotkey                 string           `json:"Hotkey"`
	Form                   []formDefinition `json:"Form"`
}

type formDefinition struct {
	Type          string              `json:"Type"`
	Value         formDefinitionValue `json:"Value"`
	SearchAliases []string            `json:"SearchAliases"`
}

type formDefinitionValue struct {
	Key               string            `json:"Key"`
	Label             string            `json:"Label"`
	Title             string            `json:"Title"`
	Suffix            string            `json:"Suffix"`
	DefaultValue      string            `json:"DefaultValue"`
	Tooltip           string            `json:"Tooltip"`
	Content           string            `json:"Content"`
	MaxLines          int               `json:"MaxLines"`
	IsMulti           bool              `json:"IsMulti"`
	Options           []formOption      `json:"Options"`
	Validators        []formValidator   `json:"Validators"`
	Columns           []formTableColumn `json:"Columns"`
	SortColumnKey     string            `json:"SortColumnKey"`
	SortOrder         string            `json:"SortOrder"`
	MaxHeight         int               `json:"MaxHeight"`
	InlineTable       bool              `json:"InlineTable"`
	UpdateDialogWidth int               `json:"UpdateDialogWidth"`
}

type formTableColumn struct {
	Key                string          `json:"Key"`
	Label              string          `json:"Label"`
	Tooltip            string          `json:"Tooltip"`
	Width              int             `json:"Width"`
	Type               string          `json:"Type"`
	Validators         []formValidator `json:"Validators"`
	SelectOptions      []formOption    `json:"SelectOptions"`
	TextMaxLines       int             `json:"TextMaxLines"`
	HideInTable        bool            `json:"HideInTable"`
	HideInUpdate       bool            `json:"HideInUpdate"`
	AllowedHotkeyKinds []string        `json:"AllowedHotkeyKinds"`
}

type formOption struct {
	Label            string `json:"Label"`
	Value            string `json:"Value"`
	ID               string `json:"ID"`
	DisplayName      string `json:"DisplayName"`
	Description      string `json:"Description"`
	Languages        string `json:"Languages"`
	Recommended      bool   `json:"Recommended"`
	Available        bool   `json:"Available"`
	Status           string `json:"Status"`
	DownloadProgress int    `json:"DownloadProgress"`
	SizeMB           int    `json:"SizeMB"`
	Error            string `json:"Error"`
}

type formValidator struct {
	Type  string             `json:"Type"`
	Value formValidatorValue `json:"Value"`
}

type formValidatorValue struct {
	IsInteger bool `json:"IsInteger"`
	IsFloat   bool `json:"IsFloat"`
}
