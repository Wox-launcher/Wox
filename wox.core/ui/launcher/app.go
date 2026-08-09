package launcher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wox/common"
	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/clipboard"
)

const (
	defaultWidth     = 760
	defaultMaxResult = 10
	resultRowGap     = 0
)

const (
	launcherWindowID   woxui.WindowID = "wox.launcher"
	settingsWindowID   woxui.WindowID = "wox.settings"
	onboardingWindowID woxui.WindowID = "wox.onboarding"
)

// App owns one launcher window and its typed core service boundary.
type App struct {
	// These narrow locks protect the few resources intentionally accessed outside the UI thread.
	translationsMu         sync.RWMutex
	terminalSubscriptionMu sync.Mutex
	unsubscribersMu        sync.Mutex
	tooltipMu              sync.Mutex
	terminalSubscribed     string
	terminalDesired        atomic.Value

	isDev          bool
	isPrimary      bool
	instanceName   string
	sessionID      string
	windowID       woxui.WindowID
	services       contract.Services
	uiCall         func(func()) error
	windows        *woxui.WindowManager
	instances      *appInstanceRegistry
	primary        *App
	destroyOnce    sync.Once
	unsubscribers  []func()
	lifecycleCtx   context.Context
	cancel         context.CancelFunc
	destroyed      atomic.Bool
	launcher       *woxui.ManagedWindow
	settingsView   *woxui.ManagedWindow
	onboardingView *woxui.ManagedWindow
	window         *woxui.Window
	host           *woxwidget.Host
	settingsHost   *woxwidget.Host
	onboardingHost *woxwidget.Host

	query             plainQuery
	queryContext      queryContext
	queryContextKnown bool
	queryHistories    []plainQuery
	queryHistoryIndex int
	canRecallHistory  bool
	editor            *woxui.TextEditor
	// selectionAnchor holds the rune offset captured at query drag-selection start so extend updates only the focus.
	selectionAnchor            int
	results                    []queryResult
	resultRevision             uint64
	resultsSectionRevision     uint64
	resultsQueryID             string
	queryComplete              bool
	queryTransitionTimer       *time.Timer
	queryLoading               bool
	queryLoadingTimer          *time.Timer
	previewTooltipRevision     atomic.Uint64
	selected                   int
	hoveredResult              int
	pendingSelection           *pendingResultSelection
	resultScroll               scrollController
	resultScrollDetached       bool
	layout                     queryLayout
	refinements                []queryRefinement
	refinementsSectionRevision uint64
	refinementOpen             bool
	refinementScope            string
	completionHint             *queryCompletionHint
	toolbarMsg                 *toolbarMessage
	toolbarRevision            uint64
	form                       *formState
	requirementForm            *requirementFormState
	launcherTableEditor        *formTableEditorState
	triggerConflict            *triggerConflictPreviewState
	chatPreview                *chatPreviewState
	webViewPreviewData         string
	webViewPreviewError        string
	webViewNavigation          woxui.WebViewNavigationState
	// Native Office preview state separates selected, delayed, and reported handler generations.
	nativeFilePreviewPath        string
	nativeFilePreviewPendingPath string
	nativeFilePreviewManualPath  string
	nativeFilePreviewError       string
	chatFullscreen               bool
	terminalFullscreen           bool
	actionPanel                  bool
	actionSelected               int
	actionSelectionKey           string
	actionsSectionRevision       uint64
	actionSectionState           actionSectionRevisionState
	actionFilter                 *woxui.TextEditor
	visible                      bool
	show                         showAppParams
	settingsOpen                 bool
	onboardingOpen               bool
	onboardingStep               int
	onboardingChoice             string
	onboardingChoiceAnchor       woxui.Rect
	onboardingPermission         contract.MacOSPermissionStatus
	onboardingLoading            bool
	onboardingError              string
	settingsCtx                  settingWindowContext
	settingTab                   string
	settingRow                   int
	settingSaving                bool
	settingFlash                 string
	settingFlashTimer            *time.Timer
	settingsInlineTooltip        *settingsInlineTooltipState
	cloudPlanTooltip             *cloudPlanTooltipState
	settingsDemo                 *settingsDemoState
	settingsDemoRevision         atomic.Uint64
	choiceTooltipRevision        atomic.Uint64
	settingsTableEditor          *formTableEditorState
	glanceItem                   *glanceItem
	glanceLoading                bool
	glanceRevision               uint64
	glanceTooltipRevision        atomic.Uint64
	glanceTimer                  *time.Timer
	// Settings controllers (zero App back-dependency; populated by newApp).
	generalSettings      *generalSettingsController
	appearanceSettings   *appearanceSettingsController
	networkSettings      *networkSettingsController
	dataSettings         *dataSettingsController
	cloudSettings        *cloudSettingsController
	runtimeSettings      *runtimeSettingsController
	themeSettings        *themeSettingsController
	pluginSettings       *pluginSettingsController
	aiSettings           *aiSettingsController
	usageSettings        *usageSettingsController
	updateSettings       *updateSettingsController
	privacySettings      *privacySettingsController
	aboutSettings        *aboutSettingsController
	hotkeySettings       *hotkeySettingsController
	settingsSearch       *settingsSearchController
	sharedEdit           *sharedEditState
	palette              uiPalette
	densityMetrics       launcherDensityMetrics
	translations         map[string]string
	translationsRevision atomic.Uint64
	// imageMu protects the image cache because image decoding completes on background goroutines.
	imageMu sync.RWMutex
	// appIcon is decoded during app construction so native title bars never start without their icon.
	appIcon                                   *woxui.Image
	images                                    map[string]*woxui.Image
	imagesRevision                            atomic.Uint64
	imageRequested                            map[string]string
	imageVariants                             map[string]string
	imageVariantKeys                          map[string]string
	imageLastUsed                             map[string]uint64
	imageUseSequence                          uint64
	imageErrors                               map[string]string
	remotePreviews                            map[string]queryPreview
	previewRequests                           map[string]bool
	filePreviews                              map[string]filePreviewContent
	fileRequests                              map[string]bool
	nativeFilePreviewGeneration               uint64
	nativeFilePreviewTimer                    *time.Timer
	nativeFilePreviewBoundsTimer              *time.Timer
	nativeFilePreviewBounds                   woxui.Rect
	nativeFilePreviewBoundsPath               string
	nativeFilePreviewBoundsGeneration         uint64
	nativeFilePreviewReportedBounds           woxui.Rect
	nativeFilePreviewReportedBoundsPath       string
	nativeFilePreviewReportedBoundsGeneration uint64
	nativeFilePreviewHasReportedBounds        bool
	mdDocs                                    map[string]woxcomponent.MarkdownDocument
	previewLayouts                            map[string]woxwidget.TextBlockLayout
	dictationAudio                            *dictationPreviewAudioState
	terminalPreview                           *terminalPreviewState

	// queryFocusNotifiedInActiveWindow prevents the initial native activation
	// from duplicating the query-focus notification already sent while laying out.
	queryFocusNotifiedInActiveWindow bool
}

// New creates a launcher whose typed core services are supplied by the process composition root.
func New(isDev bool, services contract.Services) *App {
	windows := woxui.NewWindowManager()
	instances := newAppInstanceRegistry()
	app := newApp(isDev, services, windows, instances, nil, true, "", launcherWindowID)
	app.primary = app
	instances.registerPrimary(app)
	return app
}

// newApp builds isolated launcher state while sharing only process-wide window and message infrastructure.
func newApp(isDev bool, services contract.Services, windows *woxui.WindowManager, instances *appInstanceRegistry, primary *App, isPrimary bool, instanceName string, windowID woxui.WindowID) *App {
	sessionID := newID()
	lifecycleCtx, cancel := context.WithCancel(context.Background())
	if windowID == "" {
		windowID = woxui.WindowID("wox.instance." + sessionID)
	}
	appIcon, _ := decodeWoxImageWithTint(appIconImageSource, nil, 256)
	app := &App{
		isDev:            isDev,
		isPrimary:        isPrimary,
		instanceName:     instanceName,
		sessionID:        sessionID,
		windowID:         windowID,
		services:         services,
		uiCall:           woxui.Call,
		windows:          windows,
		instances:        instances,
		primary:          primary,
		lifecycleCtx:     lifecycleCtx,
		cancel:           cancel,
		appIcon:          appIcon,
		query:            newInputQuery(""),
		editor:           woxui.NewTextEditor(""),
		selected:         -1,
		hoveredResult:    -1,
		settingTab:       "general",
		palette:          defaultPalette(),
		densityMetrics:   launcherDensityMetricsFor(""),
		translations:     map[string]string{},
		images:           map[string]*woxui.Image{},
		imageRequested:   map[string]string{},
		imageVariants:    map[string]string{},
		imageVariantKeys: map[string]string{},
		imageLastUsed:    map[string]uint64{},
		imageErrors:      map[string]string{},
		remotePreviews:   map[string]queryPreview{},
		previewRequests:  map[string]bool{},
		filePreviews:     map[string]filePreviewContent{},
		fileRequests:     map[string]bool{},
		mdDocs:           map[string]woxcomponent.MarkdownDocument{},
		previewLayouts:   map[string]woxwidget.TextBlockLayout{},
		show: showAppParams{
			WindowWidth:    defaultWidth,
			MaxResultCount: defaultMaxResult,
			StartPage:      "mru",
		},
	}
	app.terminalDesired.Store("")
	app.unsubscribersMu.Lock()
	app.unsubscribers = append(app.unsubscribers, app.windows.SubscribeMessages(app.windowID, settingsChangedTopic, app.onSharedSettingsChanged))
	app.unsubscribersMu.Unlock()
	deps := CommonDeps{
		Invalidate: app.invalidateSettingsWindow,
		Translate:  app.translate,
		IsDev:      isDev,
		Palette:    func() uiPalette { return app.palette },
		RunOnUI:    app.runOnUI,
	}
	app.sharedEdit = newSharedEditState()
	app.generalSettings = newGeneralSettingsController(deps, app.sharedEdit)
	app.appearanceSettings = newAppearanceSettingsController(deps)
	app.networkSettings = newNetworkSettingsController(deps)
	app.dataSettings = newDataSettingsController(deps)
	// Wire cross-domain helpers the controller needs without giving it a back-reference to *App.
	// reloadSettings refreshes all settings after a restore; pickDirectory opens the native directory picker.
	app.dataSettings.BindCrossDomain(
		app.reloadSettings,
		func() (string, error) {
			window := app.settingsNativeWindow()
			if window == nil {
				return "", fmt.Errorf("settings window not ready")
			}
			return window.PickFile(woxui.FileDialogOptions{Directory: true})
		},
	)
	app.cloudSettings = newCloudSettingsController(deps)
	app.runtimeSettings = newRuntimeSettingsController(deps)
	app.themeSettings = newThemeSettingsController(deps)
	app.pluginSettings = newPluginSettingsController(deps)
	app.aiSettings = newAISettingsController(deps)
	app.usageSettings = newUsageSettingsController(deps)
	app.updateSettings = newUpdateSettingsController(deps)
	app.privacySettings = newPrivacySettingsController(deps)
	app.aboutSettings = newAboutSettingsController(deps)
	app.hotkeySettings = newHotkeySettingsController(deps)
	app.settingsSearch = newSettingsSearchController(deps)
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
		Title:           "Wox",
		Size:            woxui.Size{Width: float32(a.show.WindowWidth), Height: a.densityMetrics.queryBoxHeight + a.palette.appPadding.Top + a.palette.appPadding.Bottom + a.densityMetrics.toolbarHeight},
		OnFrame:         host.Frame,
		OnPointer:       host.Pointer,
		OnFileDrop:      a.handleFileDrop,
		OnFileDragEnded: a.handleResultDragEnded,
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
		OnFocus: func(event woxui.FocusEvent) {
			host.SetWindowFocused(event.Active)
			a.onFocus(event)
		},
		OnWebViewHideRequested: func() {
			util.Go(a.lifecycleCtx, "hide launcher from webview toolbar", func() {
				if err := a.hideWindow(true); err != nil {
					log.Printf("hide launcher from webview toolbar: %v", err)
				}
			})
		},
		OnWebViewNavigationChanged: func(state woxui.WebViewNavigationState) {
			if a.webViewPreviewData == "" {
				return
			}
			a.webViewNavigation = state
			if a.window != nil {
				_ = a.window.Invalidate()
			}
		},
		OnClosed: func() {
			host.Dispose()
			a.onLauncherWindowClosed()
		},
	})
	if err != nil {
		return err
	}
	a.launcher = launcher
	a.window = launcher.Window()
	a.host = host
	host.Attach(a.window)
	if err := a.window.SetAppearance(themeColorIsDark(a.palette.background)); err != nil {
		return fmt.Errorf("apply Wox UI appearance: %w", err)
	}
	if err := a.window.SetFontFamily(a.generalSettings.Data().AppFontFamily); err != nil {
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
		var launcher *woxui.ManagedWindow
		if err := a.runOnUI("resolve secondary launcher for close", func() {
			launcher = a.launcher
		}); err != nil {
			return err
		}
		if launcher != nil {
			return launcher.Close()
		}
		a.destroySecondary()
		return nil
	}

	a.destroyed.Store(true)
	cancel := a.cancel
	if cancel != nil {
		cancel()
	}
	var closeErr error
	if a.windows != nil {
		closeErr = a.windows.CloseAll()
	}
	a.unsubscribeAll()
	return closeErr
}

func (a *App) showWindow(params showAppParams) error {
	var launcher *woxui.ManagedWindow
	queryEmpty := false
	preserveQuery := false
	if err := a.runOnUI("prepare launcher show", func() {
		if params.WindowWidth <= 0 {
			params.WindowWidth = defaultWidth
		}
		if params.MaxResultCount <= 0 {
			params.MaxResultCount = defaultMaxResult
		}
		a.show = params
		a.queryHistories = append(a.queryHistories[:0], params.QueryHistories...)
		// A selection query on the primary launcher is always transient (Space Quick
		// Look preview or a selection-bearing query hotkey). Reopening through the
		// main hotkey must start fresh; otherwise the stale selection context keeps
		// every keystroke pinned to the previous selection query.
		if params.ShowSource == string(common.ShowSourceDefault) && a.query.QueryType == "selection" {
			a.query = newInputQuery("")
			a.queryContext = queryContext{IsGlobalQuery: true}
			a.queryContextKnown = true
			a.editor.SetText("", false)
			a.beginQueryTransitionLocked()
		}
		preserveQuery = a.applyLaunchModeOnShowLocked()
		a.canRecallHistory = a.query.QueryType == "input"
		if params.LaunchMode == "continue" {
			// The newest history is the current continued query, so the first Up recalls the entry before it.
			a.queryHistoryIndex = 0
		} else {
			a.queryHistoryIndex = -1
		}
		if params.SelectAll {
			a.editor.SelectAll()
		}
		a.actionPanel = false
		a.actionSelected = 0
		a.actionSelectionKey = ""
		a.actionFilter = nil
		a.form = nil
		a.visible = true
		queryEmpty = a.query.QueryText == ""
		launcher = a.launcher
		a.reconcileSelectedPreview()
		a.restoreQueryTextInput()
	}); err != nil {
		return err
	}
	if launcher == nil {
		return errors.New("launcher window is not initialized")
	}
	if err := a.window.SetHideOnBlur(params.HideOnBlur); err != nil {
		return err
	}
	if err := a.applyWindowBoundsAtShowPosition(); err != nil {
		return err
	}
	if _, err := launcher.Show(); err != nil {
		return err
	}
	if err := a.runOnUI("restore launcher query focus after show", func() {
		a.restoreQueryFocusAfterShow()
	}); err != nil {
		return err
	}
	if err := a.notifyShown(); err != nil {
		return err
	}
	if queryEmpty && params.StartPage == "mru" && !preserveQuery {
		if err := a.requestMRU(); err != nil {
			return err
		}
	}
	util.Go(a.lifecycleCtx, "refresh glance after window shown", func() {
		a.refreshGlance("windowShown", "", nil)
	})
	return nil
}

// restoreQueryFocusAfterShow resets retained preview focus once the visible query tree is mounted.
func (a *App) restoreQueryFocusAfterShow() bool {
	if a.host == nil || !a.queryCanFocus() {
		return false
	}
	a.host.RequestFocus(launcherview.LauncherQueryInputKey)
	if !a.host.HasFocus(launcherview.LauncherQueryInputKey) {
		return false
	}
	a.restoreQueryTextInput()
	return true
}

func (a *App) hideWindow(notify bool) error {
	// Secondaries are named and can coexist (selection, explorer, tray, webview).
	// Only cacheable WebView panels hide-and-retain so navigation can resume;
	// every other secondary still destroys on dismiss.
	if !a.isPrimary && !a.shouldRetainSecondaryOnHide() {
		return a.Close()
	}

	var launcher *woxui.ManagedWindow
	alreadyHidden := false
	if err := a.runOnUI("prepare launcher hide", func() {
		if !a.visible {
			alreadyHidden = true
			return
		}
		a.actionPanel = false
		a.actionSelected = 0
		a.actionSelectionKey = ""
		a.actionFilter = nil
		a.form = nil
		a.visible = false
		a.stopGlanceLocked(false)
		launcher = a.launcher
		a.reconcileSelectedPreview()
		a.requirementForm = nil
		a.triggerConflict = nil
		a.clearLauncherThemeEditorPreview()
		a.resetChatPreview()
	}); err != nil {
		return err
	}
	if alreadyHidden {
		return nil
	}
	if launcher == nil {
		return errors.New("launcher window is not initialized")
	}
	if err := launcher.Hide(); err != nil {
		return err
	}
	// Quick re-shows keep their warm icon cache; only trim decoded images after
	// the launcher stays hidden long enough to be considered idle.
	util.Go(a.lifecycleCtx, "trim hidden launcher image cache", func() {
		time.Sleep(10 * time.Second)
		if err := a.runOnUI("trim hidden launcher image cache", func() {
			if a.visible {
				return
			}
			a.trimIdleImageCache()
		}); err != nil {
			log.Printf("trim hidden launcher image cache: %v", err)
		}
	})
	if notify {
		return a.notifyHidden()
	}
	return nil
}

// shouldRetainSecondaryOnHide keeps only cacheable WebView secondaries alive across hide.
func (a *App) shouldRetainSecondaryOnHide() bool {
	retain := false
	if err := a.runOnUI("inspect secondary webview retain", func() {
		retain = a.hasCacheableWebViewPreviewLocked()
	}); err != nil {
		return false
	}
	return retain
}

// hasCacheableWebViewPreviewLocked reports an active WebView preview that opted into session reuse.
func (a *App) hasCacheableWebViewPreviewLocked() bool {
	if strings.TrimSpace(a.webViewPreviewData) == "" {
		return false
	}
	data, err := decodeWebViewPreview(a.webViewPreviewData)
	if err != nil || data.CacheDisabled {
		return false
	}
	return true
}

// closePreviewWindow dismisses only the launcher instance that owns the preview.
func (a *App) closePreviewWindow() {
	util.Go(a.lifecycleCtx, "close launcher from preview close", func() {
		if err := a.hideWindow(true); err != nil {
			log.Printf("close launcher from preview close: %v", err)
		}
	})
}

func (a *App) onFocus(event woxui.FocusEvent) {
	if event.Active {
		a.notifyQueryBoxFocusOnWindowActivation()
		return
	}
	a.queryFocusNotifiedInActiveWindow = false
	if !a.visible {
		return
	}
	hideOnBlur := a.show.HideOnBlur
	launcher := a.launcher
	retainSecondary := a.isPrimary || a.hasCacheableWebViewPreviewLocked()
	if hideOnBlur {
		a.visible = false
		a.stopGlanceLocked(false)
	}
	if hideOnBlur {
		if !retainSecondary {
			util.Go(a.lifecycleCtx, "close secondary launcher after blur", func() {
				if err := a.Close(); err != nil {
					log.Printf("close secondary launcher after blur: %v", err)
				}
			})
			return
		}
		a.reconcileSelectedPreview()
		if launcher != nil {
			_ = launcher.Hide()
		}
		a.resetChatPreview()
	}
	util.Go(a.lifecycleCtx, "notify launcher focus change", func() {
		if hideOnBlur {
			if err := a.notifyHidden(); err != nil {
				log.Printf("notify Wox core after blur hide: %v", err)
			}
			return
		}
		if err := a.notifyFocusLost(); err != nil {
			log.Printf("notify Wox core after focus loss: %v", err)
		}
	})
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
	if !a.isPrimary {
		return nil
	}
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
		query.QueryID = newID()
	}
	if query.QueryType == "" {
		query.QueryType = "input"
	}
	a.query = query
	a.queryContext = queryContext{}
	a.queryContextKnown = false
	a.editor.SetText(query.QueryText, false)
	a.resetQueryTransitionLocked()
	a.resetQueryLoadingLocked()
	a.results = nil
	a.resultsSectionRevision++
	a.resultsQueryID = ""
	a.queryComplete = false
	a.selected = -1
	a.hoveredResult = -1
	a.resultScroll.reset()
	a.resultScrollDetached = false
	a.layout = queryLayout{}
	a.stopGlanceLocked(true)
	a.refinements = nil
	a.refinementsSectionRevision++
	a.refinementOpen = false
	a.refinementScope = ""
	a.completionHint = nil
	a.actionPanel = false
	a.actionSelected = 0
	a.actionSelectionKey = ""
	a.actionFilter = nil
	a.form = nil
	a.reconcileSelectedPreview()
	a.requirementForm = nil
	a.triggerConflict = nil
	a.clearLauncherThemeEditorPreview()
	a.resetChatPreview()
	a.restoreQueryTextInput()
	_ = a.window.Invalidate()
}

func (a *App) sendCurrentQuery() error {
	var query plainQuery
	var startPage string
	var skipCompletionHint bool
	var preserveQuery bool
	if err := a.runOnUI("prepare current query", func() {
		query = a.query
		startPage = a.show.StartPage
		skipCompletionHint = !a.generalSettings.Data().EnableQueryCompletionHint
		preserveQuery = a.shouldPreserveQueryOnShowLocked()
		a.startQueryLoadingLocked()
	}); err != nil {
		return err
	}
	if err := a.startTypedQuery(query, skipCompletionHint); err != nil {
		_ = a.runOnUI("stop query loading after start failure", a.resetQueryLoadingLocked)
		return err
	}
	if !preserveQuery && query.QueryText == "" && startPage == "mru" {
		return a.requestMRU()
	}
	return nil
}

// applyLaunchModeOnShowLocked resets stale launcher state unless this show action carries an explicit query.
func (a *App) applyLaunchModeOnShowLocked() bool {
	preserveQuery := a.shouldPreserveQueryOnShowLocked()
	if a.show.LaunchMode == "fresh" && !preserveQuery {
		a.setQuery(newInputQuery(""))
	}
	return preserveQuery
}

// shouldPreserveQueryOnShowLocked mirrors Flutter's incoming-query preservation:
// selection/query-hotkey/tray/explorer shows inject a new query payload on show, and
// continue mode keeps an existing input or selection query. Both must survive the
// MRU/blank start-page handling that otherwise replaces an empty input query.
func (a *App) shouldPreserveQueryOnShowLocked() bool {
	switch a.show.ShowSource {
	case "query_hotkey", "selection", "tray_query", "explorer":
		return true
	}
	if a.show.LaunchMode == "continue" {
		if a.query.QueryType == "selection" {
			return true
		}
		return a.query.QueryType == "input" && a.query.QueryText != ""
	}
	return false
}

// usePinYin is a cross-domain reader for the general-domain UsePinYin setting.
// Query, action filter, and settings search all use pinyin matching when this is on.
func (a *App) usePinYin() bool {
	return a.generalSettings.Data().UsePinYin
}

func (a *App) requestMRU() error {
	queryID := ""
	if err := a.runOnUI("prepare MRU query", func() {
		a.query = newInputQuery("")
		a.queryContext = queryContext{IsGlobalQuery: true}
		a.queryContextKnown = true
		a.editor.SetText("", false)
		queryID = a.query.QueryID
		a.queryComplete = false
		a.beginQueryTransitionLocked()
		// MRU only replaces query results; the window-shown path owns the Glance refresh.
		a.stopGlanceLocked(false)
		a.refinements = nil
		a.refinementsSectionRevision++
		a.refinementOpen = false
		a.refinementScope = ""
		a.completionHint = nil
		a.actionPanel = false
		a.actionSelected = 0
		a.actionSelectionKey = ""
		a.actionFilter = nil
		a.form = nil
		a.reconcileSelectedPreview()
		a.requirementForm = nil
		a.triggerConflict = nil
		a.clearLauncherThemeEditorPreview()
		a.resetChatPreview()
	}); err != nil {
		return err
	}
	util.Go(a.lifecycleCtx, "load MRU results", func() {
		a.loadTypedMRU(queryID)
	})
	return nil
}

func (a *App) applyResults(queryID string, results []queryResult, layout *queryLayout, refinements *[]queryRefinement, context *queryContext, queryStartTimestamp int64, complete bool) {
	if a.destroyed.Load() || queryID == "" || queryID != a.query.QueryID {
		return
	}
	if a.isDev && a.generalSettings.Data().ShowPerformanceTail && a.generalSettings.Data().ShowPerformanceTailUIReceived && queryStartTimestamp > 0 {
		appendUIReceivedTails(results, max(int64(0), time.Now().UnixMilli()-queryStartTimestamp))
	}
	for index := range results {
		if results[index].QueryID == "" {
			results[index].QueryID = queryID
		}
	}
	if len(results) > 0 || complete {
		a.resetQueryLoadingLocked()
	}
	a.resetQueryTransitionLocked()
	for index := range results {
		a.resultRevision++
		results[index].Revision = a.resultRevision
	}
	a.results = results
	a.resultsSectionRevision++
	a.resultsQueryID = queryID
	a.queryComplete = complete
	a.hoveredResult = -1
	enterChatMode := layout != nil && layout.ChatMode
	if layout != nil {
		a.layout = *layout
		if !layout.ChatMode {
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
		a.scheduleGlanceRefreshLocked(a.generalSettings.Data().PrimaryGlance)
	} else if !glanceEligible {
		a.stopGlanceLocked(true)
	}
	a.selected = selectableIndex(results)
	preservedSelection := false
	if a.pendingSelection != nil {
		if a.pendingSelection.queryID == queryID {
			a.selected = selectableIndexFrom(results, a.pendingSelection.index)
			preservedSelection = true
		}
		a.pendingSelection = nil
	}
	if !preservedSelection {
		a.resultScrollDetached = false
	}
	closedActionPanel := false
	if a.actionPanel && len(unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg)) == 0 {
		closedActionPanel = a.resetActionPanelLocked()
	} else if a.actionPanel {
		a.normalizeActionSelectionLocked()
	}
	a.reconcileSelectedPreview()
	if enterChatMode {
		a.enterChatMode()
	}
	if closedActionPanel {
		a.restoreQueryTextInput()
	}
	if refreshGlance {
		util.Go(a.lifecycleCtx, "refresh glance after query results", func() {
			a.refreshGlance("manualRefresh", "", nil)
		})
	}
	if err := a.applyWindowBounds(); err != nil {
		log.Printf("resize launcher for query results: %v", err)
	}
	_ = a.window.Invalidate()
}

func (a *App) applyWindowBounds() error {
	return a.applyWindowBoundsWithPlacement(false)
}

func (a *App) applyWindowBoundsAtShowPosition() error {
	return a.applyWindowBoundsWithPlacement(true)
}

func (a *App) applyWindowBoundsWithPlacement(useShowPosition bool) error {
	var params showAppParams
	var results []queryResult
	var layout queryLayout
	var palette uiPalette
	var densityMetrics launcherDensityMetrics
	var resultCount int
	var actionCount int
	var formHeight int
	var refinementVisible bool
	var actionPanel bool
	var requirementPreview bool
	var previewVisible bool
	var toolbarMessageVisible bool
	var previewFullscreen bool
	var chatFullscreen bool
	var queryText string
	if err := a.runOnUI("snapshot launcher window bounds", func() {
		params = a.show
		results = append([]queryResult(nil), a.results...)
		resultCount = len(results)
		layout = a.layout
		refinementVisible = len(a.refinements) > 0 && a.refinementOpen && !params.HideQueryBox
		actionPanel = a.actionPanel
		palette = a.palette
		densityMetrics = a.densityMetrics.normalized()
		queryText = a.editor.State().Text
		if a.form != nil {
			formHeight = int(densityMetrics.scaled(formContentMaximumHeight) + 2*densityMetrics.scaled(10))
		}
		toolbarMessageVisible = a.toolbarMsg != nil
		chatFullscreen = a.chatFullscreen
		previewFullscreen = chatFullscreen || a.terminalFullscreen
		if actionPanel && a.actionFilter != nil {
			actionCount = len(filteredActionIndices(unifiedActionPanelEntries(a.results, a.selected, a.toolbarMsg), a.actionFilter.State().Text, a.translationSnapshot(), a.usePinYin()))
		}
		if a.selected >= 0 && a.selected < len(a.results) {
			requirementPreview = a.results[a.selected].Preview.PreviewType == "query_requirement_settings"
			previewVisible = a.results[a.selected].Preview.PreviewData != ""
		}
	}); err != nil {
		return err
	}
	width := params.WindowWidth
	if width <= 0 {
		width = defaultWidth
	}
	maxResults := params.MaxResultCount
	if maxResults <= 0 {
		maxResults = defaultMaxResult
	}
	visibleResults := min(resultCount, maxResults)
	resultRowHeight := int(densityMetrics.resultRowHeight(palette))
	resultVerticalPadding := int(palette.resultContainerPadding.Top + palette.resultContainerPadding.Bottom)
	queryAreaHeight := int(densityMetrics.queryBoxHeightForText(queryText, a.queryLineHeight(densityMetrics)) + palette.appPadding.Top + palette.appPadding.Bottom)
	// With the query box hidden, buildResults folds appPadding.Bottom into the
	// result list content, so the window height must reserve that margin too.
	// Otherwise the list overflows by it and a scrollbar appears even when every
	// result fits.
	resultBottomInset := 0
	if params.HideQueryBox {
		resultBottomInset = int(palette.appPadding.Bottom)
	}
	toolbarHasContent := resultCount > 0 || toolbarMessageVisible
	toolbarHeightIncluded := launcherToolbarHeightIncluded(params.HideToolbar, toolbarHasContent, previewFullscreen, chatFullscreen)
	height := 0
	if !params.HideQueryBox {
		height += queryAreaHeight
	} else {
		height += resultBottomInset
	}
	if refinementVisible {
		height += int(densityMetrics.refinementBarHeight)
	}
	if visibleResults > 0 {
		if layout.GridLayout != nil {
			height += min(gridResultsHeight(results, float32(width), layout.GridLayout), maxResults*resultRowHeight)
		} else {
			height += resultVerticalPadding + visibleResults*resultRowHeight + max(0, visibleResults-1)*resultRowGap
		}
	}
	if toolbarHeightIncluded {
		height += int(densityMetrics.toolbarHeight)
	}
	maximumResultWindowHeight := resultVerticalPadding + maxResults*resultRowHeight + max(0, maxResults-1)*resultRowGap
	if !params.HideQueryBox {
		maximumResultWindowHeight += queryAreaHeight
	} else {
		maximumResultWindowHeight += resultBottomInset
	}
	if refinementVisible {
		maximumResultWindowHeight += int(densityMetrics.refinementBarHeight)
	}
	if toolbarHeightIncluded {
		maximumResultWindowHeight += int(densityMetrics.toolbarHeight)
	}
	if previewVisible {
		height = max(height, maximumResultWindowHeight)
	}
	if requirementPreview {
		minimumHeight := 360
		if !params.HideQueryBox {
			minimumHeight += queryAreaHeight
		} else {
			minimumHeight += resultBottomInset
		}
		if refinementVisible {
			minimumHeight += int(densityMetrics.refinementBarHeight)
		}
		if toolbarHeightIncluded {
			minimumHeight += int(densityMetrics.toolbarHeight)
		}
		height = max(height, minimumHeight)
	}
	if actionPanel {
		actionHeight := int(actionPanelBaseHeightForPalette(palette)) + max(1, min(actionCount, maxVisibleActions))*actionRowHeight
		if !params.HideQueryBox {
			actionHeight += queryAreaHeight
		} else {
			actionHeight += resultBottomInset
		}
		if refinementVisible {
			actionHeight += int(densityMetrics.refinementBarHeight)
		}
		if toolbarHeightIncluded {
			actionHeight += int(densityMetrics.toolbarHeight)
		}
		// Opening the action panel restores Flutter's full configured result height while still allowing larger panels to fit.
		height = max(height, maximumResultWindowHeight, actionHeight)
	}
	if formHeight > 0 {
		formWindowHeight := formHeight + 20
		if !params.HideQueryBox {
			formWindowHeight += queryAreaHeight
		} else {
			formWindowHeight += resultBottomInset
		}
		if refinementVisible {
			formWindowHeight += int(densityMetrics.refinementBarHeight)
		}
		if toolbarHeightIncluded {
			formWindowHeight += int(densityMetrics.toolbarHeight)
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

// launcherToolbarHeightIncluded preserves the hidden toolbar's space only in Flutter's chat mode.
func launcherToolbarHeightIncluded(hideToolbar, hasContent, previewFullscreen, chatFullscreen bool) bool {
	return !hideToolbar && hasContent && (!previewFullscreen || chatFullscreen)
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
	if !a.formTableUsesSettingsWindow() && a.launcherTableEditor != nil {
		return a.onFormTableKey(event)
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
	if a.onRefinementHotkey(event) {
		return true
	}
	if a.onResultActionHotkey(event) {
		return true
	}
	if a.layout.GridLayout != nil {
		switch event.Key {
		case woxui.KeyArrowLeft:
			a.moveSelection(-1)
			return true
		case woxui.KeyArrowRight:
			a.moveSelection(1)
			return true
		}
	}
	// Copy/cut/paste on the query editor use the primary modifier and the cross-platform clipboard.
	if event.Down && !event.Composing && event.Modifiers.HasPrimary() && (event.Key == woxui.Key("c") || event.Key == woxui.Key("x") || event.Key == woxui.Key("v")) {
		switch event.Key {
		case woxui.Key("c"):
			if selected := a.editor.SelectedText(); selected != "" {
				_ = clipboard.WriteText(selected)
			}
			return true
		case woxui.Key("x"):
			previousText := a.editor.State().Text
			selected := a.editor.SelectedText()
			if selected != "" {
				_ = clipboard.WriteText(selected)
				if a.editor.DeleteSelection() {
					a.applyQueryTextChangeLocked(a.editor.State().Text)
				}
			}
			_ = a.window.Invalidate()
			a.reconcileSelectedPreview()
			if err := a.sendCurrentQuery(); err != nil {
				log.Printf("send query after cut: %v", err)
			}
			a.resizeLauncherForQueryLineChange(previousText)
			return true
		case woxui.Key("v"):
			text, err := clipboard.ReadText()
			if err != nil || text == "" {
				return true
			}
			previousText := a.editor.State().Text
			if a.editor.InsertText(normalizeQueryNewlines(text)) {
				a.applyQueryTextChangeLocked(a.editor.State().Text)
			}
			_ = a.window.Invalidate()
			a.reconcileSelectedPreview()
			if err := a.sendCurrentQuery(); err != nil {
				log.Printf("send query after paste: %v", err)
			}
			a.resizeLauncherForQueryLineChange(previousText)
			return true
		}
	}
	previousText := a.editor.State().Text
	textHandled, textChanged := handleQueryEditorKey(a.editor, event)
	if textChanged {
		a.applyQueryTextChangeLocked(a.editor.State().Text)
	}
	if textHandled {
		_ = a.window.Invalidate()
		if textChanged {
			a.reconcileSelectedPreview()
			if err := a.sendCurrentQuery(); err != nil {
				log.Printf("send query after editing command: %v", err)
			}
			a.resizeLauncherForQueryLineChange(previousText)
		}
		return true
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		if event.Modifiers == 0 {
			query, handled := a.previousQueryHistory()
			if handled {
				if query != nil {
					a.setQuery(*query)
					a.editor.SelectAll()
					if err := a.sendCurrentQuery(); err != nil {
						log.Printf("send recalled query history: %v", err)
					}
				}
				return true
			}
		}
		a.moveSelection(-a.resultNavigationColumns())
		return true
	case woxui.KeyArrowDown:
		if event.Modifiers == 0 {
			a.canRecallHistory = false
		}
		a.moveSelection(a.resultNavigationColumns())
		return true
	case woxui.KeyEnter:
		a.activateSelected()
		return true
	case woxui.KeyEscape:
		util.Go(a.lifecycleCtx, "hide launcher from escape", func() {
			if err := a.hideWindow(true); err != nil {
				log.Printf("hide launcher: %v", err)
			}
		})
		return true
	default:
		return false
	}
	return false
}

func handleQueryEditorKey(editor *woxui.TextEditor, event woxui.KeyEvent) (bool, bool) {
	if editor != nil && event.Down && !event.Composing && event.Key == woxui.KeyEnter && event.Modifiers == woxui.KeyModifierShift {
		return true, editor.InsertText("\n")
	}
	return editor.HandleKey(event)
}

// resizeLauncherForQueryLineChange avoids native window work for ordinary single-line edits.
func (a *App) resizeLauncherForQueryLineChange(previousText string) {
	if launcherQueryLineCount(previousText) == launcherQueryLineCount(a.editor.State().Text) {
		return
	}
	if err := a.applyWindowBounds(); err != nil {
		log.Printf("resize launcher after query line change: %v", err)
	}
}

// previousQueryHistory advances through the show-time history snapshot while history recall remains active.
func (a *App) previousQueryHistory() (*plainQuery, bool) {
	if !a.canRecallHistory {
		return nil, false
	}
	if a.queryHistoryIndex >= len(a.queryHistories)-1 {
		return nil, true
	}
	a.queryHistoryIndex++
	query := a.queryHistories[a.queryHistoryIndex]
	return &query, true
}

func (a *App) resultNavigationColumns() int {
	layout := a.layout.GridLayout
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
	previousText := a.editor.State().Text
	if event.Kind == woxui.TextInputCommit {
		event.Text = normalizeQueryNewlines(event.Text)
	}
	committed := a.editor.HandleTextInput(event)
	if committed {
		a.applyQueryTextChangeLocked(a.editor.State().Text)
	}
	_ = a.window.Invalidate()
	if committed {
		a.reconcileSelectedPreview()
		if err := a.sendCurrentQuery(); err != nil {
			log.Printf("send committed query: %v", err)
		}
		a.resizeLauncherForQueryLineChange(previousText)
	}
}

func normalizeQueryNewlines(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

func (a *App) moveSelection(delta int) {
	if len(a.results) == 0 {
		return
	}
	target := a.selected
	columns := a.resultNavigationColumns()
	if a.layout.GridLayout != nil && (delta == columns || delta == -columns) {
		direction := 1
		if delta < 0 {
			direction = -1
		}
		target = gridSelectionIndex(a.results, a.selected, columns, direction)
	} else {
		index := a.selected
		for attempts := 0; attempts < len(a.results); attempts++ {
			index = (index + delta + len(a.results)) % len(a.results)
			if !a.results[index].IsGroup {
				target = index
				break
			}
		}
	}
	if target != a.selected {
		a.selected = target
		a.resultScrollDetached = false
		a.actionPanel = false
		a.actionSelected = 0
		a.actionSelectionKey = ""
		a.actionFilter = nil
		a.chatFullscreen = false
		a.reconcileSelectedPreview()
		a.restoreQueryTextInput()
	}
	_ = a.window.Invalidate()
}

func (a *App) selectResult(index int) {
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
	if inside {
		if index >= 0 && index < len(a.results) && !a.results[index].IsGroup {
			a.hoveredResult = index
		}
	} else if a.hoveredResult == index {
		a.hoveredResult = -1
	}
}

func (a *App) activateSelected() {
	selected := a.selected
	a.activateResult(selected)
}

func (a *App) activateResult(index int) {
	if len(a.results) > 0 && (index < 0 || index >= len(a.results) || a.results[index].IsGroup) {
		return
	}
	entries := unifiedActionPanelEntries(a.results, index, a.toolbarMsg)
	if len(entries) == 0 {
		return
	}
	entry := entries[0]
	for _, candidate := range entries {
		if candidate.IsDefault {
			entry = candidate
			break
		}
	}
	a.activateActionPanelEntry(entry)
}

func selectableIndex(results []queryResult) int {
	for index, result := range results {
		if !result.IsGroup {
			return index
		}
	}
	return -1
}

// selectableIndexFrom restores an explicitly preserved refresh index while skipping group rows.
func selectableIndexFrom(results []queryResult, start int) int {
	for index := max(0, start); index < len(results); index++ {
		if !results[index].IsGroup {
			return index
		}
	}
	return selectableIndex(results)
}

type pendingResultSelection struct {
	queryID string
	index   int
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
		QueryID:          newID(),
		QueryType:        "input",
		QueryText:        text,
		QuerySelection:   selection{FilePaths: []string{}},
		QueryRefinements: map[string]string{},
		ContextData:      map[string]string{},
	}
}

type showAppParams struct {
	// ShowPreviewTitleBar is an internal launcher control for full preview windows.
	ShowPreviewTitleBar bool         `json:"-"`
	SelectAll           bool         `json:"SelectAll"`
	Position            position     `json:"Position"`
	WindowWidth         int          `json:"WindowWidth"`
	MaxResultCount      int          `json:"MaxResultCount"`
	QueryHistories      []plainQuery `json:"QueryHistories"`
	LaunchMode          string       `json:"LaunchMode"`
	StartPage           string       `json:"StartPage"`
	HideQueryBox        bool         `json:"HideQueryBox"`
	HideToolbar         bool         `json:"HideToolbar"`
	QueryBoxAtBottom    bool         `json:"QueryBoxAtBottom"`
	HideOnBlur          bool         `json:"HideOnBlur"`
	ShowSource          string       `json:"ShowSource"`
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
	QueryID  string               `json:"QueryId"`
	ID       string               `json:"Id"`
	Title    string               `json:"Title"`
	SubTitle string               `json:"SubTitle"`
	Icon     woxImage             `json:"Icon"`
	Preview  queryPreview         `json:"Preview"`
	Tails    []resultTail         `json:"Tails"`
	Actions  []resultAction       `json:"Actions"`
	DragData *queryResultDragData `json:"DragData"`
	IsGroup  bool                 `json:"IsGroup"`
	Revision uint64               `json:"-"`
}

type queryResultDragData struct {
	Type  string   `json:"Type"`
	Files []string `json:"Files"`
}

func (d *queryResultDragData) isFiles() bool {
	return d != nil && d.Type == "files" && len(d.Files) > 0
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
	MinimumRowCount   int               `json:"MinimumRowCount"`
	MinimumRowMessage string            `json:"MinimumRowMessage"`
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
	// EmptyAsZero maps blank editor text to persisted integer 0 (and the reverse on load).
	EmptyAsZero bool `json:"EmptyAsZero"`
}

type formOption struct {
	Label            string   `json:"Label"`
	Value            string   `json:"Value"`
	Icon             woxImage `json:"Icon"`
	ID               string   `json:"ID"`
	DisplayName      string   `json:"DisplayName"`
	Description      string   `json:"Description"`
	Languages        string   `json:"Languages"`
	Recommended      bool     `json:"Recommended"`
	Available        bool     `json:"Available"`
	Status           string   `json:"Status"`
	DownloadProgress int      `json:"DownloadProgress"`
	SizeMB           int      `json:"SizeMB"`
	Error            string   `json:"Error"`
}

type formValidator struct {
	Type  string             `json:"Type"`
	Value formValidatorValue `json:"Value"`
}

type formValidatorValue struct {
	IsInteger bool   `json:"IsInteger"`
	IsFloat   bool   `json:"IsFloat"`
	Optional  bool   `json:"Optional"`
	HasRange  bool   `json:"HasRange"`
	Min       int    `json:"Min"`
	Max       int    `json:"Max"`
	ErrorKey  string `json:"ErrorKey"`
}
