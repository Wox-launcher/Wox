package launcher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wox/common"
	"wox/network"
	"wox/setting"
	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/screen"
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
	chatWindowID       woxui.WindowID = "wox.chat"
)

// pendingResultSelection restores the highlight after RefreshQuery or query undo/redo.
// resultID follows one result when the plugin rebuilds it with the same id.
// preserveIndex keeps the old row when resultID is empty or that result is gone.
type pendingResultSelection struct {
	queryID       string
	index         int
	resultID      string
	preserveIndex bool
	// querySnapshot uses text matching and first-row overflow fallback only for undo/redo.
	querySnapshot *queryResultSelection
}

type plainQuery struct {
	QueryHint        *common.QueryHint `json:"QueryHint,omitempty"`
	QueryID          string            `json:"QueryId"`
	QueryType        string            `json:"QueryType"`
	QueryText        string            `json:"QueryText"`
	QuerySelection   selection         `json:"QuerySelection"`
	QueryScope       queryScope        `json:"QueryScope"`
	QueryRefinements map[string]string `json:"QueryRefinements"`
	ContextData      map[string]string `json:"ContextData"`
}

type queryScope struct {
	Plugins []queryScopePlugin `json:"Plugins,omitempty"`
}

type queryScopePlugin struct {
	PluginID string `json:"PluginId"`
	Command  string `json:"Command,omitempty"`
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
		QueryScope:       queryScope{},
		QueryRefinements: map[string]string{},
		ContextData:      map[string]string{},
	}
}

type showAppParams struct {
	// ShowPreviewTitleBar is an internal launcher control for full preview windows.
	ShowPreviewTitleBar bool                         `json:"-"`
	SelectAll           bool                         `json:"SelectAll"`
	Position            position                     `json:"Position"`
	PositionHeight      int                          `json:"PositionHeight"`
	WindowWidth         int                          `json:"WindowWidth"`
	MaxResultCount      int                          `json:"MaxResultCount"`
	QueryHistories      []plainQuery                 `json:"QueryHistories"`
	LaunchMode          string                       `json:"LaunchMode"`
	StartPage           string                       `json:"StartPage"`
	HideQueryBox        bool                         `json:"HideQueryBox"`
	HideToolbar         bool                         `json:"HideToolbar"`
	QueryBoxAtBottom    bool                         `json:"QueryBoxAtBottom"`
	HideOnBlur          bool                         `json:"HideOnBlur"`
	RememberPosition    bool                         `json:"-"`
	ShowSource          string                       `json:"ShowSource"`
	RestoreWindow       *common.ActiveWindowSnapshot `json:"-"`
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
	ScopeIcons              []woxImage  `json:"ScopeIcons,omitempty"`
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
	QueryID           string               `json:"QueryId"`
	ID                string               `json:"Id"`
	Title             string               `json:"Title"`
	SubTitle          string               `json:"SubTitle"`
	Icon              woxImage             `json:"Icon"`
	IconShowContainer bool                 `json:"IconShowContainer"`
	Preview           queryPreview         `json:"Preview"`
	TitleTags         []resultTitleTag     `json:"TitleTags"`
	Tails             []resultTail         `json:"Tails"`
	Actions           []resultAction       `json:"Actions"`
	DragData          *queryResultDragData `json:"DragData"`
	IsGroup           bool                 `json:"IsGroup"`
	Revision          uint64               `json:"-"`
}

type queryResultDragData struct {
	Type                 string   `json:"Type"`
	Files                []string `json:"Files"`
	PreventHideAfterDrag bool     `json:"PreventHideAfterDrag"`
}

func (d *queryResultDragData) isFiles() bool {
	return d != nil && d.Type == "files" && len(d.Files) > 0
}

type resultTitleTag struct {
	Text    string   `json:"Text"`
	Kind    string   `json:"Kind"`
	Labels  []string `json:"Labels"`
	Tooltip string   `json:"Tooltip"`
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
	DefaultHidden      bool              `json:"DefaultHidden"`
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
	ID                     string            `json:"Id"`
	Type                   string            `json:"Type"`
	Name                   string            `json:"Name"`
	SearchAliases          []string          `json:"SearchAliases"`
	Icon                   woxImage          `json:"Icon"`
	IsDefault              bool              `json:"IsDefault"`
	PreventHideAfterAction bool              `json:"PreventHideAfterAction"`
	ShowInToolbar          bool              `json:"ShowInToolbar"`
	Hotkey                 string            `json:"Hotkey"`
	Form                   []formDefinition  `json:"Form"`
	IsSystemAction         bool              `json:"IsSystemAction"`
	Tail                   string            `json:"Tail"`
	TailIcon               woxImage          `json:"TailIcon"`
	ContextData            map[string]string `json:"ContextData"`
}

type formDefinition struct {
	Type          string              `json:"Type"`
	Value         formDefinitionValue `json:"Value"`
	SearchAliases []string            `json:"SearchAliases"`
}

type formDefinitionValue struct {
	Key          string `json:"Key"`
	Label        string `json:"Label"`
	Title        string `json:"Title"`
	Suffix       string `json:"Suffix"`
	DefaultValue string `json:"DefaultValue"`
	Tooltip      string `json:"Tooltip"`
	// Filterable shows a search field in a select dropdown.
	Filterable bool `json:"Filterable,omitempty"`
	// ColumnType keeps the original table column type after the row editor maps it to a textbox.
	ColumnType string `json:"ColumnType"`
	// QueryVariableKind selects the {wox:...} picker set for query-variable columns.
	QueryVariableKind string `json:"QueryVariableKind"`
	// QueryTest shows the test button that runs this field's value as a launcher query.
	QueryTest         bool              `json:"QueryTest"`
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
	EnableSearch      bool              `json:"EnableSearch"`
	SearchColumnKey   string            `json:"SearchColumnKey"`
	Groups            []formTableGroup  `json:"Groups"`
	// Group is the row-editor section this field belongs to after a table column is mapped.
	Group       string              `json:"Group"`
	Rows        []formStatsRow      `json:"Rows"`
	Description string              `json:"Description"`
	Status      string              `json:"Status"`
	Detail      string              `json:"Detail"`
	Actions     []formServiceAction `json:"Actions"`
}

type formServiceAction struct {
	ID      string `json:"ID"`
	Label   string `json:"Label"`
	Primary bool   `json:"Primary"`
	Danger  bool   `json:"Danger"`
	Enabled bool   `json:"Enabled"`
}

type formStatsRow struct {
	Label string `json:"Label"`
	Value string `json:"Value"`
}

type formTableColumn struct {
	Key           string          `json:"Key"`
	Label         string          `json:"Label"`
	Tooltip       string          `json:"Tooltip"`
	Width         int             `json:"Width"`
	Type          string          `json:"Type"`
	Validators    []formValidator `json:"Validators"`
	SelectOptions []formOption    `json:"SelectOptions"`
	// Filterable shows a search field in this column's select dropdown.
	Filterable         bool     `json:"Filterable,omitempty"`
	TextMaxLines       int      `json:"TextMaxLines"`
	HideInTable        bool     `json:"HideInTable"`
	HideInUpdate       bool     `json:"HideInUpdate"`
	AllowedHotkeyKinds []string `json:"AllowedHotkeyKinds"`
	// EmptyAsZero maps blank editor text to persisted integer 0 (and the reverse on load).
	EmptyAsZero bool `json:"EmptyAsZero"`
	// VisibleWhen hides the column in the add/edit dialog unless another field matches.
	VisibleWhen formTableColumnVisibleWhen `json:"VisibleWhen"`
	// PreviewMatchedApps shows a live indexed-app list under this text column.
	PreviewMatchedApps bool `json:"PreviewMatchedApps"`
	// QueryVariableKind selects the {wox:...} picker set for query-variable columns.
	QueryVariableKind string `json:"QueryVariableKind"`
	// QueryTest shows the test button that runs this column's value as a launcher query.
	QueryTest bool `json:"QueryTest"`
	// Group is a formTableGroup.Key. Empty keeps the field ungrouped at the top.
	Group string `json:"Group"`
}

// formTableGroup names a collapsible section in the add/edit row dialog.
type formTableGroup struct {
	Key                string `json:"Key"`
	Title              string `json:"Title"`
	Tooltip            string `json:"Tooltip"`
	CollapsedByDefault bool   `json:"CollapsedByDefault"`
}

// formTableColumnVisibleWhen shows a row-editor field only when another field has one of Values.
type formTableColumnVisibleWhen struct {
	Key    string   `json:"Key"`
	Values []string `json:"Values"`
	Not    bool     `json:"Not,omitempty"`
}

type formOption struct {
	Label string   `json:"Label"`
	Value string   `json:"Value"`
	Icon  woxImage `json:"Icon"`
	// IsSelectAll stores only this option and clears every other selection.
	IsSelectAll bool `json:"IsSelectAll,omitempty"`
	// Group is a translated or i18n section label shown above this option.
	Group            string `json:"Group,omitempty"`
	GroupTooltip     string `json:"GroupTooltip,omitempty"`
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
	IsInteger bool   `json:"IsInteger"`
	IsFloat   bool   `json:"IsFloat"`
	Optional  bool   `json:"Optional"`
	HasRange  bool   `json:"HasRange"`
	Min       int    `json:"Min"`
	Max       int    `json:"Max"`
	ErrorKey  string `json:"ErrorKey"`
}

// App composes one launcher instance and coordinates cross-feature transitions.
// Feature controllers own their state and receive only the capabilities they need.
type App struct {
	attention    attentionController
	resultDrag   resultDragController
	appearance   appearanceController
	requirements requirementController
	conflicts    triggerConflictController
	quickSelect  quickSelectController
	previews     previewController
	notes        notesController
	forms        formActionController
	formTables   formTableController
	// hotkeyRecording is shared by Settings, onboarding and launcher forms; only one field records at a time.
	hotkeyRecording hotkeyRecordingController
	windowGroups    windowGroupEditorController
	glance          glanceController
	toolbar         toolbarController
	settingsWindow  settingsWindowController
	onboarding      onboardingController
	chatWindow      chatWindowController
	chat            chatController
	querySession    queryController
	resultList      resultController
	actions         actionPanelController
	dictation       dictationPreviewController
	terminal        terminalController
	webView         webViewPreviewController
	nativeFile      nativeFilePreviewController
	remotePreview   remotePreviewController
	filePreview     filePreviewController
	language        languageCatalog
	tooltips        tooltipController
	imageStore      imageStore

	unsubscribersMu sync.Mutex

	isDev         bool
	isPrimary     bool
	instanceName  string
	sessionID     string
	windowID      woxui.WindowID
	services      contract.Services
	uiCall        func(func()) error
	uiPost        func(func()) error
	windows       *woxui.WindowManager
	instances     *appInstanceRegistry
	primary       *App
	destroyOnce   sync.Once
	unsubscribers []func()
	lifecycleCtx  context.Context
	cancel        context.CancelFunc
	destroyed     atomic.Bool
	launcher      *woxui.ManagedWindow

	window *woxui.Window
	host   *woxwidget.Host

	visible bool
	show    showAppParams
	// permissionFlow remembers which Wox windows the macOS permission guide hid so
	// they can be restored when System Settings is closed.
	permissionFlow *macOSPermissionFlowHost
	// bottomAnchorY keeps QueryBoxAtBottom windows from drifting when DPI
	// round-trips make Bounds().Height slightly larger than the logical height
	// we last requested.
	bottomAnchorY float32
	// automationFocusInstance routes smoke Snapshot/Bounds/Perform to a named
	// secondary when set; empty keeps the primary launcher surface.
	automationFocusInstance string

	// Settings controllers (zero App back-dependency; populated by newApp).
	generalSettings    *generalSettingsController
	appearanceSettings *appearanceSettingsController
	networkSettings    *networkSettingsController
	dataSettings       *dataSettingsController
	cloudSettings      *cloudSettingsController
	runtimeSettings    *runtimeSettingsController
	themeSettings      *themeSettingsController
	pluginSettings     *pluginSettingsController

	aiSettings      *aiSettingsController
	usageSettings   *usageSettingsController
	updateSettings  *updateSettingsController
	privacySettings *privacySettingsController
	aboutSettings   *aboutSettingsController
	hotkeySettings  *hotkeySettingsController
	settingsSearch  *settingsSearchController
	sharedEdit      *sharedEditState

	// appIcon is decoded during app construction so native title bars never start without their icon.
	appIcon *woxui.Image
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

// WindowManager returns the process-local registry shared by launcher-owned secondary surfaces.
func (a *App) WindowManager() *woxui.WindowManager {
	return a.windows
}

// newApp builds isolated launcher state while sharing only process-wide window and message infrastructure.
func newApp(isDev bool, services contract.Services, windows *woxui.WindowManager, instances *appInstanceRegistry, primary *App, isPrimary bool, instanceName string, windowID woxui.WindowID) *App {
	sessionID := newID()
	lifecycleCtx, cancel := context.WithCancel(context.Background())
	if windowID == "" {
		windowID = woxui.WindowID("wox.instance." + sessionID)
	}
	// Title bars draw this at 20 logical px, so 64 px covers 3x scaling; decoding the
	// full 256 px source kept a 256 KB raster per launcher instance for a 20 px glyph.
	appIcon, _ := decodeWoxImageWithTint(appIconImageSource, nil, appIconTitleBarPixels)
	app := &App{
		isDev:        isDev,
		isPrimary:    isPrimary,
		instanceName: instanceName,
		sessionID:    sessionID,
		windowID:     windowID,
		services:     services,
		uiCall:       woxui.Call,
		uiPost:       woxui.Post,
		windows:      windows,
		instances:    instances,
		primary:      primary,
		lifecycleCtx: lifecycleCtx,
		cancel:       cancel,
		appIcon:      appIcon,
		querySession: queryController{
			query:  newInputQuery(""),
			editor: woxui.NewTextEditor(""),
		},
		resultList: resultController{
			selected:      -1,
			hoveredResult: -1,
		},
		settingsWindow: settingsWindowController{tab: "general"},
		appearance: appearanceController{
			palette:        defaultPalette(),
			densityMetrics: launcherDensityMetricsFor(""),
		},
		language: languageCatalog{translations: map[string]string{}},
		imageStore: imageStore{
			images:           map[string]*woxui.Image{},
			imageRequested:   map[string]string{},
			imageVariants:    map[string]string{},
			imageVariantKeys: map[string]string{},
			imageLastUsed:    map[string]uint64{},
			imageErrors:      map[string]string{},
		},
		remotePreview: remotePreviewController{
			remotePreviews:  map[string]queryPreview{},
			previewRequests: map[string]bool{},
		},
		filePreview: filePreviewController{
			filePreviews:           map[string]filePreviewContent{},
			fileRequests:           map[string]bool{},
			filePreviewManualPaths: map[string]bool{},
		},
		previews: previewController{
			mdDocs:         map[string]woxcomponent.MarkdownDocument{},
			previewLayouts: map[string]*textLayoutCache{},
		},
		notes: notesController{noteWindows: map[string]*notesWindowController{}},
		show: showAppParams{
			WindowWidth:    defaultWidth,
			MaxResultCount: defaultMaxResult,
			StartPage:      "mru",
		},
	}
	// Settings controllers exist before binding so feature bindings can wire them.
	deps := CommonDeps{
		Invalidate: app.settingsWindow.invalidateSettingsWindow,
		Translate:  app.language.translate,
		IsDev:      isDev,
		RunOnUI:    app.runOnUI,
	}
	app.sharedEdit = newSharedEditState()
	app.generalSettings = newGeneralSettingsController(deps, app.sharedEdit)
	app.appearanceSettings = newAppearanceSettingsController(deps)
	app.networkSettings = newNetworkSettingsController(deps)
	app.dataSettings = newDataSettingsController(deps)
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
	app.bindControllers()
	app.terminal.terminalDesired.Store("")
	app.unsubscribersMu.Lock()
	app.unsubscribers = append(app.unsubscribers, app.windows.SubscribeMessages(app.windowID, settingsChangedTopic, app.onSharedSettingsChanged))
	app.unsubscribers = append(app.unsubscribers, network.Subscribe(func(enabled bool) {
		_ = app.runOnUI("apply offline mode", func() {
			if enabled {
				app.webView.resetWebView()
			} else {
				app.imageStore.retryErrors()
			}
			_ = app.window.Invalidate()
			app.settingsWindow.invalidateSettingsWindow()
		})
	}))
	app.unsubscribersMu.Unlock()
	return app
}

// Start connects to core and creates the hidden native window on the UI runtime thread.
func (a *App) Start() error {
	return a.start()
}

// launcherQueryWindowTopmost reports whether the query window shares the always-on-top
// band with overlay HUDs. Windows needs HWND_TOPMOST to stay above other apps.
// macOS floating windows and Linux layer-shell TOP already sit above ordinary
// apps, so the launcher stays below Topmost timer and tooltip overlays.
func launcherQueryWindowTopmost(goos string) bool {
	return goos == "windows"
}

// start initializes one independent launcher session against the shared window runtime.
func (a *App) start() error {
	if a.services == nil {
		return errors.New("core lifecycle services are required")
	}
	if err := a.appearance.reloadTheme(); err != nil {
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
		Title: "Wox",
		// The idle launcher has no results yet, so this first size omits the about-menu-only toolbar.
		Size: woxui.Size{Width: float32(a.show.WindowWidth), Height: a.appearance.densityMetrics.queryBoxHeight + a.appearance.palette.appPadding.Top + a.appearance.palette.appPadding.Bottom},
		// Windows uses HWND_TOPMOST so the query window stays above other apps.
		// macOS floating level and Linux layer-shell TOP already do that without
		// occupying the overlay band, so timer/tooltip HUDs can sit above Wox.
		Topmost: launcherQueryWindowTopmost(runtime.GOOS),
		OnFrame: host.Frame,
		OnPointer: func(event woxui.PointerEvent) {
			if event.Kind == woxui.PointerDown {
				a.resultDrag.releaseResultFileDragHold()
			}
			host.Pointer(event)
		},
		OnFileDrop:      a.handleFileDrop,
		OnFileDragEnded: a.resultDrag.handleResultDragEnded,
		OnKey: func(event woxui.KeyEvent) bool {
			if event.Down {
				a.resultDrag.releaseResultFileDragHold()
			}
			if event.Down && !event.Composing && !a.hotkeyRecordingUsesSettingsWindow() && !a.hotkeyRecordingUsesOnboardingWindow() && a.hotkeyRecording.onHotkeyRecordingKey(event) {
				return true
			}
			settingsPluginID := ""
			if a.chat.chatFullscreen {
				settingsPluginID = common.AIChatPluginID
			} else if a.webView.webViewPreviewData != "" {
				settingsPluginID = firstNonEmpty(a.querySession.queryContext.PluginID, common.WebViewPluginID)
			}
			return a.dispatchWindowKey(event, host, settingsPluginID, a.requestLauncherShortcutClose, a.onKey)
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
			if a.isDestroyed() || a.webView.webViewPreviewData == "" {
				return
			}
			a.webView.webViewNavigation = state
			if a.window != nil {
				_ = a.window.Invalidate()
			}
		},
		OnClosed: func() {
			host.Dispose()
			a.onLauncherWindowClosed()
		},
		OnCloseRequested: a.closePreviewWindow,
	})
	if err != nil {
		return err
	}
	a.launcher = launcher
	a.window = launcher.Window()
	a.host = host
	host.Attach(a.window)
	if err := a.window.SetAppearance(a.appearance.palette.isDark()); err != nil {
		return fmt.Errorf("apply Wox UI appearance: %w", err)
	}
	if err := a.window.SetWindowChrome(a.appearance.palette.AppWindowChrome, a.appearance.palette.AppBorderRadius, a.appearance.palette.background.A != 0); err != nil {
		return fmt.Errorf("apply Wox UI chrome: %w", err)
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
		if err := a.chatWindow.closeChatWindow(); err != nil {
			return err
		}
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
	cleanupErr := a.runOnUI("dispose launcher controllers", a.disposeControllers)
	a.terminal.releaseTerminalSubscription()
	var closeErr error
	if a.windows != nil {
		closeErr = a.windows.CloseAll()
	}
	a.unsubscribeAll()
	return errors.Join(cleanupErr, closeErr)
}

func (a *App) showWindow(params showAppParams) error {
	var launcher *woxui.ManagedWindow
	queryEmpty := false
	preserveQuery := false
	refreshInlinePreview := false
	wasVisible := false
	if err := a.runOnUI("prepare launcher show", func() {
		if params.WindowWidth <= 0 {
			params.WindowWidth = defaultWidth
		}
		if params.MaxResultCount <= 0 {
			params.MaxResultCount = defaultMaxResult
		}
		a.resultDrag.releaseResultFileDragHold()
		wasVisible = a.visible
		a.show = params
		// A selection query on the primary launcher is always transient (Space Quick
		// Look preview or a selection-bearing query hotkey). Reopening through the
		// main hotkey must start fresh; otherwise the stale selection context keeps
		// every keystroke pinned to the previous selection query.
		if params.ShowSource == string(common.ShowSourceDefault) && a.querySession.query.QueryType == "selection" {
			a.querySession.queryHintEditorState = queryHintEditor{}
			a.querySession.resetToGlobalInputLocked()
		}
		preserveQuery = a.applyLaunchModeOnShowLocked()
		refreshInlinePreview = !wasVisible && a.previews.hasInlinePreviewToRestore()
		a.querySession.armHistoryRecall(params.QueryHistories, params.LaunchMode == "continue")
		if params.SelectAll {
			a.querySession.selectEntireQuery()
		}
		a.actions.clearActionPanelStateLocked()
		a.forms.form = nil
		a.visible = true
		queryEmpty = a.querySession.query.QueryText == ""
		launcher = a.launcher
		a.webView.keepQueryFocusOnWebViewActivate = true
		a.previews.reconcileSelectedPreview()
		a.webView.keepQueryFocusOnWebViewActivate = false
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
	// Explorer type-to-search re-Shows the same secondary on every key. Re-applying
	// the initial show position would fight the bottom anchor; keep resizing in place.
	if wasVisible && params.QueryBoxAtBottom {
		if err := a.applyWindowBounds(); err != nil {
			return err
		}
	} else if err := a.applyWindowBoundsAtShowPosition(); err != nil {
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
	if refreshInlinePreview {
		util.GetLogger().Info(a.lifecycleCtx, "refresh inline WebView state after launcher restore")
		if err := a.RefreshQuery(a.lifecycleCtx, common.RefreshQueryOptions{PreserveSelectedIndex: true}); err != nil {
			return err
		}
	}
	if queryEmpty && params.StartPage == "mru" && !preserveQuery {
		if err := a.requestMRU(); err != nil {
			return err
		}
	}
	util.Go(a.lifecycleCtx, "refresh glance after window shown", func() {
		a.glance.refreshGlance("windowShown", "", nil)
	})
	return nil
}

// restoreQueryFocusAfterShow resets retained preview focus once the visible query tree is mounted.
func (a *App) restoreQueryFocusAfterShow() bool {
	// A restored WebView must not retry page focus after the query box is selected.
	a.webView.webViewWantKeyboardFocus = false
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
		var restoreWindow *common.ActiveWindowSnapshot
		if err := a.runOnUI("capture launcher source window", func() {
			if a.show.RestoreWindow != nil {
				snapshot := *a.show.RestoreWindow
				restoreWindow = &snapshot
			}
		}); err != nil {
			return err
		}
		closeErr := a.Close()
		if restoreWindow != nil && a.services != nil {
			if err := a.services.RestoreActiveWindow(context.Background(), a.sessionID, *restoreWindow); err != nil {
				util.GetLogger().Warn(context.Background(), fmt.Sprintf("restore launcher source window after hide: %v", err))
			}
		}
		return closeErr
	}

	var launcher *woxui.ManagedWindow
	alreadyHidden := false
	var hideErr error
	var lastX, lastY int
	saveLast := false
	if err := a.runOnUI("prepare launcher hide", func() {
		if !a.visible {
			alreadyHidden = true
			return
		}
		if a.isPrimary && a.show.RememberPosition {
			lastX, lastY, saveLast = a.captureLauncherOriginLocked()
		}
		a.actions.clearActionPanelStateLocked()
		a.forms.form = nil
		a.quickSelect.stopQuickSelectLocked()
		a.visible = false
		a.bottomAnchorY = 0
		a.glance.stopGlanceLocked(false)
		a.dismissLauncherHoverTooltipsOnUI()
		launcher = a.launcher
		a.previews.reconcileSelectedPreview()
		a.requirements.requirementForm = nil
		a.conflicts.triggerConflict = nil
		a.chat.resetChatPreview()
		a.webView.clearWebViewPreviewModeLocked()
		if launcher != nil {
			hideErr = launcher.Hide()
		}
	}); err != nil {
		return err
	}
	if alreadyHidden {
		return nil
	}
	if launcher == nil {
		return errors.New("launcher window is not initialized")
	}
	if hideErr != nil {
		return hideErr
	}
	if saveLast {
		if err := saveLastWindowOrigin(lastX, lastY); err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("save last window location: %v", err))
		}
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
			return
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
	if strings.TrimSpace(a.webView.webViewPreviewData) == "" {
		return false
	}
	data, err := decodeWebViewPreview(a.webView.webViewPreviewData)
	if err != nil {
		return false
	}
	return data.HTML != "" || !data.CacheDisabled
}

// closePreviewWindow dismisses only the launcher instance that owns the preview.
func (a *App) closePreviewWindow() {
	util.Go(a.lifecycleCtx, "close launcher from preview close", func() {
		if err := a.hideWindow(true); err != nil {
			log.Printf("close launcher from preview close: %v", err)
		}
	})
}

// captureLauncherOriginLocked reads the primary window origin in logical coordinates.
func (a *App) captureLauncherOriginLocked() (int, int, bool) {
	if a.window == nil {
		return 0, 0, false
	}
	bounds, err := a.window.Bounds()
	if err != nil {
		return 0, 0, false
	}
	return roundLogicalCoordinate(bounds.X), roundLogicalCoordinate(bounds.Y), true
}

func roundLogicalCoordinate(value float32) int {
	if value >= 0 {
		return int(value + 0.5)
	}
	return int(value - 0.5)
}

// saveLastWindowOrigin writes one local value before the next show reads it.
func saveLastWindowOrigin(x, y int) error {
	woxSetting := setting.CurrentWoxSetting()
	if woxSetting == nil || woxSetting.ShowPosition == nil || woxSetting.ShowPosition.Get() != setting.PositionTypeLastLocation {
		return nil
	}
	if woxSetting.LastWindowPosition == nil {
		return nil
	}
	previous := woxSetting.LastWindowPosition.Get()
	if previous.Valid && previous.X == x && previous.Y == y {
		return nil
	}
	return woxSetting.LastWindowPosition.Set(setting.SavedWindowPosition{X: x, Y: y, Valid: true})
}

func (a *App) onFocus(event woxui.FocusEvent) {
	if event.Active {
		a.notifyQueryBoxFocusOnWindowActivation()
		return
	}
	a.querySession.queryFocusNotifiedInActiveWindow = false
	if !a.visible {
		return
	}
	// Result file drag visibility is owned by PreventHideAfterDrag, not hide-on-blur.
	if a.resultDrag.resultFileDragActive {
		return
	}
	hideOnBlur := a.show.HideOnBlur
	launcher := a.launcher
	// A pop-out still uses this session's services after the launcher loses focus.
	retainSecondary := a.isPrimary || a.chatWindow.chatWindowOpen() || a.hasCacheableWebViewPreviewLocked()
	var lastX, lastY int
	saveLast := false
	if hideOnBlur && a.isPrimary && a.show.RememberPosition {
		lastX, lastY, saveLast = a.captureLauncherOriginLocked()
	}
	if hideOnBlur {
		a.visible = false
		a.bottomAnchorY = 0
		a.glance.stopGlanceLocked(false)
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
		a.previews.reconcileSelectedPreview()
		if launcher != nil {
			_ = launcher.Hide()
		}
		a.chat.resetChatPreview()
		a.webView.clearWebViewPreviewModeLocked()
	}
	if saveLast {
		if err := saveLastWindowOrigin(lastX, lastY); err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("save last window location after blur: %v", err))
		}
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

// beginQueryGenerationLocked stamps a new query generation in place. Every field it
// clears describes the results some earlier generation produced, so a caller that mints
// a QueryID without clearing them keeps reporting that older state for the new query.
// Callers that replace the whole query, such as setQuery, own this bookkeeping already.
func (a *App) beginQueryGenerationLocked() {
	a.querySession.releaseRefreshResultHoldLocked()
	if previous, known := a.querySession.startNewGeneration(); known {
		a.attention.attentionQueryWasGlobal = previous.IsGlobalQuery
	}
	a.resultList.pendingSelection = nil
}

// clearQueryAccessoriesLocked drops the controls and previews that belonged to a replaced query.
func (a *App) clearQueryAccessoriesLocked() {
	a.resultList.clearRefinements()
	a.querySession.completionHint = nil
	a.actions.clearActionPanelStateLocked()
	a.forms.form = nil
	a.previews.reconcileSelectedPreview()
	a.requirements.requirementForm = nil
	a.conflicts.triggerConflict = nil
	a.chat.resetChatPreview()
	a.webView.clearWebViewPreviewModeLocked()
}

func (a *App) setQuery(query plainQuery) {
	a.replaceQuery(query, true)
}

// resetQuery replaces the query without creating an undo step, used for session clears.
func (a *App) resetQuery(query plainQuery) {
	a.replaceQuery(query, false)
}

// replaceQuery installs a complete query. When rememberPrevious is set, Ctrl/Cmd+Z
// restores the prior query box state, including plugin context from ChangeQuery.
func (a *App) replaceQuery(query plainQuery, rememberPrevious bool) {
	var previous queryHintSnapshot
	var previousUndo []queryHintSnapshot
	if rememberPrevious {
		previous = a.querySession.captureQuerySnapshot()
		previousUndo = append([]queryHintSnapshot(nil), a.querySession.queryHintEditorState.undo...)
	}
	if query.QueryID == "" {
		query.QueryID = newID()
	}
	if query.QueryType == "" {
		query.QueryType = "input"
	}
	a.querySession.queryHintEditorState = queryHintEditor{}
	hasExplicitHint := query.QueryHint != nil
	query.QueryHint = query.QueryHint.Clone()
	query.QueryHint = query.QueryHint.NormalizeForQuery(query.QueryType, query.QueryText)
	a.querySession.query = query
	a.resultList.pendingSelection = nil
	a.attention.attentionQueryWasGlobal = false
	a.querySession.queryContext = queryContext{}
	a.querySession.queryContextKnown = false
	a.querySession.editor.SetText(query.QueryText, false)
	if query.QueryHint != nil {
		a.querySession.installQueryHint(query.QueryHint)
	} else if !hasExplicitHint && query.QueryType == "input" {
		// Indicator and other ChangeQuery replacements resolve the same guidance as typing.
		a.querySession.query.QueryText = a.querySession.updateQueryHintText(query.QueryText)
	}
	a.querySession.resetQueryTransitionLocked()
	a.querySession.releaseRefreshResultHoldLocked()
	a.querySession.resetQueryLoadingLocked()
	a.querySession.queryComplete = false
	a.resultList.clearRows()
	a.glance.stopGlanceLocked(true)
	a.clearQueryAccessoriesLocked()
	a.restoreQueryTextInput()
	if rememberPrevious {
		a.querySession.queryHintEditorState.undo = previousUndo
		if !querySnapshotSameContent(previous, a.querySession.captureQuerySnapshot()) {
			a.querySession.appendQueryUndo(previous)
		}
	}
	_ = a.window.Invalidate()
}

func (a *App) sendCurrentQuery() error {
	var query plainQuery
	var startPage string
	var skipCompletionHint bool
	if err := a.runOnUI("prepare current query", func() {
		query = a.querySession.query
		query.QueryHint = a.querySession.query.QueryHint.Clone()
		startPage = a.show.StartPage
		skipCompletionHint = a.querySession.query.QueryHint != nil || a.querySession.queryHintEditorState.candidate != nil || !a.generalSettings.Data().EnableQueryCompletionHint
		a.querySession.startQueryLoadingLocked()
	}); err != nil {
		return err
	}
	// A selection query keeps its payload in QuerySelection, so an empty QueryText
	// is not an empty query box; replacing it with MRU reassigns QueryID and the
	// in-flight selection results are discarded by applyResults.
	// Show-source preservation only applies when opening the launcher; clearing
	// an input query must return to MRU even after a query hotkey or deeplink.
	if query.QueryType != "selection" && query.QueryText == "" && len(query.QueryScope.Plugins) == 0 && startPage == "mru" {
		return a.requestMRU()
	}
	if err := a.startTypedQuery(query, skipCompletionHint); err != nil {
		_ = a.runOnUI("stop query loading after start failure", a.querySession.resetQueryLoadingLocked)
		return err
	}
	return nil
}

// applyLaunchModeOnShowLocked resets stale launcher state unless this show action carries an explicit query.
func (a *App) applyLaunchModeOnShowLocked() bool {
	preserveQuery := a.shouldPreserveQueryOnShowLocked()
	if a.show.LaunchMode == "fresh" && !preserveQuery {
		a.resetQuery(newInputQuery(""))
	}
	return preserveQuery
}

// shouldPreserveQueryOnShowLocked mirrors Flutter's incoming-query preservation:
// selection/query-hotkey/tray/quickjump/deeplink shows inject a new query payload on show, and
// continue mode keeps an existing input or selection query. Both must survive the
// MRU/blank start-page handling that otherwise replaces an empty input query.
func (a *App) shouldPreserveQueryOnShowLocked() bool {
	switch a.show.ShowSource {
	case "query_hotkey", "selection", "tray_query", "quickjump", "deeplink":
		return true
	}
	if a.show.LaunchMode == "continue" {
		if a.querySession.query.QueryType == "selection" {
			return true
		}
		return a.querySession.query.QueryType == "input" && (a.querySession.query.QueryText != "" || len(a.querySession.query.QueryScope.Plugins) > 0)
	}
	return false
}

// usePinYin is a cross-domain reader for the general-domain UsePinYin setting.
// Query, action filter, settings search, plugin catalog search, and table search all use pinyin matching when this is on.
func (a *App) usePinYin() bool {
	return a.generalSettings.Data().UsePinYin
}

func (a *App) requestMRU() error {
	queryID := ""
	if err := a.runOnUI("prepare MRU query", func() {
		previousQueryID := a.querySession.query.QueryID
		a.querySession.queryComplete = false
		a.querySession.resetToGlobalInputLocked()
		queryID = a.querySession.query.QueryID
		// MRU replaces the refresh generation; carry its selection into the actual response.
		if a.resultList.pendingSelection != nil && a.resultList.pendingSelection.queryID == previousQueryID {
			a.resultList.pendingSelection.queryID = queryID
		}
		// MRU only replaces query results; the window-shown path owns the Glance refresh.
		a.glance.stopGlanceLocked(false)
		a.clearQueryAccessoriesLocked()
	}); err != nil {
		return err
	}
	util.Go(a.lifecycleCtx, "load MRU results", func() {
		a.loadTypedMRU(queryID)
	})
	return nil
}

func (a *App) applyResults(queryID string, results []queryResult, layout *queryLayout, refinements *[]queryRefinement, context *queryContext, queryStartTimestamp int64, complete bool) {
	if a.destroyed.Load() || queryID == "" || queryID != a.querySession.query.QueryID {
		return
	}
	if a.querySession.holdRefreshResultsLocked(queryID, results, layout, refinements, context, queryStartTimestamp, complete) {
		return
	}
	a.querySession.releaseRefreshResultHoldLocked()
	if a.isDev && a.generalSettings.Data().ShowPerformanceTail && a.generalSettings.Data().ShowPerformanceTailUIReceived && queryStartTimestamp > 0 {
		appendUIReceivedTails(results, max(int64(0), time.Now().UnixMilli()-queryStartTimestamp))
	}
	for index := range results {
		if results[index].QueryID == "" {
			results[index].QueryID = queryID
		}
	}
	if len(results) > 0 || complete {
		a.querySession.resetQueryLoadingLocked()
	}
	a.querySession.resetQueryTransitionLocked()
	a.resultList.publishRows(queryID, results)
	a.querySession.queryComplete = complete
	enterChatMode := layout != nil && layout.ChatMode
	a.webView.clearWebViewPreviewModeLocked()
	if layout != nil {
		a.resultList.layout = *layout
		if !layout.ChatMode {
			a.chat.chatFullscreen = false
		}
	}
	if refinements != nil {
		a.applyRefinementsLocked(*refinements)
	}
	if context != nil {
		a.querySession.queryContext = *context
		a.querySession.queryContextKnown = true
	}
	glanceEligible := a.glanceEligibleLocked()
	refreshGlance := glanceEligible && a.glance.glanceItem == nil && !a.glance.glanceLoading
	if glanceEligible && a.glance.glanceItem != nil && !a.glance.glanceLoading && a.glance.glanceTimer == nil {
		a.glance.scheduleGlanceRefreshLocked(a.generalSettings.Data().PrimaryGlance)
	} else if !glanceEligible {
		a.glance.stopGlanceLocked(true)
	}
	a.resultList.restoreSelection(queryID, complete)
	closedActionPanel := false
	if a.actions.shouldSyncActionPanelWithResults() && len(a.actions.currentActionPanelEntries()) == 0 {
		closedActionPanel = a.actions.resetActionPanelLocked()
	} else if a.actions.shouldSyncActionPanelWithResults() {
		a.actions.normalizeActionSelectionLocked()
	}
	a.previews.reconcileSelectedPreview()
	if enterChatMode {
		a.chat.enterChatMode()
	}
	if closedActionPanel {
		a.restoreQueryTextInput()
	}
	if refreshGlance {
		util.Go(a.lifecycleCtx, "refresh glance after query results", func() {
			a.glance.refreshGlance("manualRefresh", "", nil)
		})
	}
	// Queue the state update before resizing so macOS can consume it in the
	// synchronous target-size frame instead of rendering the same state twice.
	_ = a.window.Invalidate()
	if err := a.applyWindowBounds(); err != nil {
		log.Printf("resize launcher for query results: %v", err)
	}
}

// launcherResultAreaHeight sizes the result viewport for list and grid.
// The full-height budget is the list contract: container padding plus
// MaxResultCount list rows. Grid content is capped to that same row budget so a
// full grid window matches a full list window.
func launcherResultAreaHeight(results []queryResult, layout queryLayout, width float32, maxResults, resultRowHeight, resultPaddingTop, resultPaddingBottom int, groupHeaderHeight float32) int {
	if len(results) == 0 || maxResults <= 0 {
		return 0
	}
	contentHeight := 0
	if layout.GridLayout != nil {
		contentHeight = min(gridResultsHeight(results, width, layout.GridLayout), maxResults*resultRowHeight)
	} else {
		visibleRows := listVisibleRowCount(results, maxResults)
		contentHeight = int(listResultsContentHeight(results[:visibleRows], 0, 0, float32(resultRowHeight), groupHeaderHeight, float32(resultRowGap)))
	}
	return resultPaddingTop + resultPaddingBottom + contentHeight
}

// resultAreaHeightLocked resolves the same logical geometry for refresh snapshots and window sizing.
func (a *App) resultAreaHeightLocked(results []queryResult, layout queryLayout) int {
	width := a.show.WindowWidth
	if width <= 0 {
		width = defaultWidth
	}
	if a.webView.webViewFullscreen && a.webView.webViewPreviewWidth > 0 {
		width = a.webView.webViewPreviewWidth
	}
	contentWidth := max(float32(0), float32(width)-2*a.appearance.palette.AppContentInset)
	if a.appearance.palette.Surfaces != nil {
		extra := a.appearance.palette.Surfaces.ContentInsets
		contentWidth = max(0, contentWidth-extra.Left-extra.Right)
	}
	maxResults := a.show.MaxResultCount
	if maxResults <= 0 {
		maxResults = defaultMaxResult
	}
	paddingTop, paddingBottom := int(a.appearance.palette.resultContainerPadding.Top), int(a.appearance.palette.resultContainerPadding.Bottom)
	if layout.GridLayout == nil && len(results) > 0 {
		padding := launcherListPadding(a.appearance.palette, a.show)
		paddingTop, paddingBottom = int(padding.Top), int(padding.Bottom)
	}
	density := a.appearance.densityMetrics.normalized()
	return launcherResultAreaHeight(results, layout, contentWidth, maxResults, int(density.resultRowHeight(a.appearance.palette)), paddingTop, paddingBottom, density.groupHeaderHeight())
}

func (a *App) applyWindowBounds() error {
	return a.applyWindowBoundsWithPlacement(false)
}

func (a *App) applyWindowBoundsAtShowPosition() error {
	return a.applyWindowBoundsWithPlacement(true)
}

// applyWindowBoundsWithPlacement keeps state reads and native resizing in one UI turn.
func (a *App) applyWindowBoundsWithPlacement(useShowPosition bool) error {
	var boundsErr error
	if err := a.runOnUI("apply launcher window bounds", func() {
		boundsErr = a.applyWindowBoundsOnUI(useShowPosition)
	}); err != nil {
		return err
	}
	return boundsErr
}

// applyWindowBoundsOnUI prevents an older hotkey layout from overwriting newer query results.
func (a *App) applyWindowBoundsOnUI(useShowPosition bool) error {
	if runtime.GOOS == "darwin" && a.window != nil {
		controls := woxui.TitleBarControls{}
		snapshot := viewSnapshot{
			show: a.show, results: a.resultList.results, selected: a.resultList.selected, layout: a.previews.selectedPreviewLayout(),
			chatFullscreen: a.chat.chatFullscreen, webViewFullscreen: a.webView.webViewFullscreen, terminalFullscreen: a.terminal.terminalFullscreen,
		}
		if launcherPreviewTitleBarVisible(snapshot) {
			controls = woxui.TitleBarControls{Height: launcherview.SettingsTitleBarHeight, Close: true}
		}
		if err := a.window.SetTitleBarControls(controls); err != nil {
			return err
		}
	}
	params := a.show
	results := a.resultList.results
	resultCount := len(results)
	layout := a.previews.selectedPreviewLayout()
	refinementVisible := len(a.resultList.refinements) > 0 && a.resultList.refinementOpen && !params.HideQueryBox
	actionPanel := a.actions.actionPanel
	palette := a.appearance.palette
	densityMetrics := a.appearance.densityMetrics.normalized()
	queryText := a.querySession.editor.State().Text
	formHeight := 0
	if a.forms.form != nil {
		formHeight = int(densityMetrics.scaled(formContentMaximumHeight) + 2*densityMetrics.scaled(10))
	}
	toolbarMessageVisible := a.toolbar.effectiveToolbarMessage() != nil
	chatFullscreen := a.chat.chatFullscreen
	previewFullscreen := a.isPreviewFullscreen()
	actionListHeight := 0
	if actionPanel {
		entries := a.actions.currentActionPanelEntries()
		actionListHeight = int(actionPanelVisibleListHeight(entries, actionPanelUnfilteredIndices(entries)))
	}
	guidanceFormPreview, previewVisible := false, false
	if a.resultList.selected >= 0 && a.resultList.selected < len(a.resultList.results) {
		previewType := a.resultList.results[a.resultList.selected].Preview.PreviewType
		guidanceFormPreview = previewType == "query_requirement_settings" || previewType == "trigger_keyword_conflict"
		// Grid results can carry preview data without displaying a preview pane.
		previewVisible = launcherPreviewVisible(layout, a.resultList.results[a.resultList.selected].Preview)
	}
	width := params.WindowWidth
	if width <= 0 {
		width = defaultWidth
	}
	maxResults := params.MaxResultCount
	if maxResults <= 0 {
		maxResults = defaultMaxResult
	}
	if a.webView.webViewFullscreen && a.webView.webViewPreviewWidth > 0 {
		width = a.webView.webViewPreviewWidth
	}
	visibleResults := min(resultCount, maxResults)
	resultRowHeight := int(densityMetrics.resultRowHeight(palette))
	resultPaddingTop := int(palette.resultContainerPadding.Top)
	resultPaddingBottom := int(palette.resultContainerPadding.Bottom)
	queryBoxHeight := densityMetrics.queryBoxHeightForText(queryText, a.queryLineHeight(densityMetrics))
	queryAreaHeight := int(queryBoxHeight + palette.appPadding.Top + palette.appPadding.Bottom)
	resultBottomInset := 0
	if layout.GridLayout == nil && resultCount > 0 {
		// Use the same fixed list gutters and query chrome as the rendered tree.
		// The toolbar owns its edge, so app padding must not enlarge the row viewport.
		padding := launcherListPadding(palette, params)
		resultPaddingTop, resultPaddingBottom = int(padding.Top), int(padding.Bottom)
		queryHeight, _ := launcherQueryChromeMetrics(queryBoxHeight, palette.appPadding, params.QueryBoxAtBottom)
		queryAreaHeight = int(queryHeight)
	} else if params.HideQueryBox {
		// Preserve the existing outer-edge budget for grids and empty launchers.
		resultBottomInset = int(palette.appPadding.Bottom)
	}
	aboutMenuOpen := actionPanel && a.actions.actionPanelPurpose == actionPanelPurposeAbout
	toolbarHasContent := launcherToolbarHasContent(resultCount, toolbarMessageVisible, aboutMenuOpen || a.forms.form != nil)
	toolbarHeightIncluded := launcherToolbarHeightIncluded(params.HideToolbar, toolbarHasContent, previewFullscreen, chatFullscreen || a.webView.webViewFullscreen)
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
		height += a.resultAreaHeightLocked(results, layout)
	}
	if toolbarHeightIncluded {
		height += int(densityMetrics.toolbarHeight)
	}
	maximumResultWindowHeight := resultPaddingTop + resultPaddingBottom + maxResults*resultRowHeight + max(0, maxResults-1)*resultRowGap
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
	if launcherReservesFullPreviewHeight(params, previewVisible) {
		height = max(height, maximumResultWindowHeight)
	}
	if guidanceFormPreview {
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
		actionHeight := int(actionPanelBaseHeightForPalette(palette)+launcherview.ActionPanelBottomOffset(palette.appPadding.Bottom)+launcherview.ActionPanelTopGap) + max(actionRowHeight, actionListHeight)
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
		// Keep the full result area while actions are open, including when filtering leaves fewer rows.
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
	minimumHeight := 0
	if !params.HideQueryBox {
		minimumHeight += queryAreaHeight
	}
	if refinementVisible {
		minimumHeight += int(densityMetrics.refinementBarHeight)
	}
	if toolbarHeightIncluded {
		minimumHeight += int(densityMetrics.toolbarHeight)
	}
	if minimumHeight <= 0 {
		minimumHeight = min(height, resultRowHeight)
	}
	// Keep the requested result capacity inside the panel; the material rim is additional window chrome.
	if palette.Surfaces != nil {
		extra := palette.Surfaces.ContentInsets
		height += int(extra.Top + extra.Bottom)
		minimumHeight += int(extra.Top + extra.Bottom)
	}
	height += int(2 * palette.AppContentInset)
	minimumHeight += int(2 * palette.AppContentInset)
	if a.webView.webViewFullscreen && a.webView.webViewPreviewHeight > 0 {
		// Width is the launcher window width; height uses the same logical-pixel window size.
		height = max(a.webView.webViewPreviewHeight, minimumHeight)
	}
	current, err := a.window.Bounds()
	if err != nil {
		return err
	}
	bottomAnchor := a.bottomAnchorY
	targetHeight := float32(height)
	x, y, nextAnchor := launcherWindowOrigin(params, current, targetHeight, useShowPosition, bottomAnchor)
	requested := woxui.Rect{X: x, Y: y, Width: float32(width), Height: targetHeight}
	var targetDisplay screen.Display
	displays, listErr := screen.ListDisplays()
	if listErr == nil {
		var displayErr error
		targetDisplay, displayErr = a.window.BoundsDisplay(requested, !useShowPosition, displays)
		if displayErr != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("launcher-display resolve failed: showPos=%t requested=%+v error=%v", useShowPosition, requested, displayErr))
		}
		layoutDisplays := displays
		if targetDisplay.ID != "" {
			// Keep clipping and native placement on the same display. Mixed-DPI logical
			// rectangles can overlap even when the physical monitors do not (#4572).
			layoutDisplays = []screen.Display{targetDisplay}
		}
		y, targetHeight, nextAnchor = constrainLauncherHeightToWorkArea(params.QueryBoxAtBottom, x, y, float32(width), targetHeight, float32(minimumHeight), nextAnchor, layoutDisplays)
		if useShowPosition || y != requested.Y || targetHeight != requested.Height {
			util.GetLogger().Info(context.Background(), fmt.Sprintf("launcher-display showPos=%t useNativeCurrent=%t requestedLogical=%+v minimumHeight=%.1f selected=%+v displays=%+v", useShowPosition, !useShowPosition, requested, float32(minimumHeight), targetDisplay, displays))
		}
	} else {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("launcher-display enumeration failed: %v", listErr))
	}
	target := woxui.Rect{
		X:      x,
		Y:      y,
		Width:  float32(width),
		Height: targetHeight,
	}
	// TEMPORARY launcher-bounds diagnostic (remove once the Ctrl+G expand-direction
	// flicker is pinned down): records every layout pass so the visible one can be told
	// apart from the hidden pre-Show passes.
	util.GetLogger().Info(context.Background(), fmt.Sprintf("launcher-bounds showPos=%t atBottom=%t visible=%t results=%d posY=%d posH=%d anchorIn=%.1f anchorOut=%.1f current=(%.1f,%.1f %.1fx%.1f) target=(%.1f,%.1f %.1fx%.1f)",
		useShowPosition, params.QueryBoxAtBottom, a.visible, resultCount, params.Position.Y, params.PositionHeight,
		bottomAnchor, nextAnchor,
		current.X, current.Y, current.Width, current.Height,
		target.X, target.Y, target.Width, target.Height))
	sameDisplay := true
	if useShowPosition && targetDisplay.ID != "" {
		currentDisplay, displayErr := a.window.BoundsDisplay(current, true, displays)
		// Equal logical bounds can still refer to different physical displays.
		sameDisplay = displayErr == nil && currentDisplay.ID == targetDisplay.ID
	}
	if sameDisplay && launcherBoundsEffectivelyEqual(current, target) {
		if params.QueryBoxAtBottom && nextAnchor != bottomAnchor {
			a.bottomAnchorY = nextAnchor
		}
		return nil
	}
	if params.QueryBoxAtBottom {
		a.bottomAnchorY = nextAnchor
	}
	if err := a.window.SetBoundsOnDisplay(target, targetDisplay); err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("launcher-bounds apply failed: target=%+v display=%q error=%v", target, targetDisplay.ID, err))
		return err
	}
	actual, boundsErr := a.window.Bounds()
	actualDisplay, displayErr := a.window.BoundsDisplay(actual, true, displays)
	util.GetLogger().Info(context.Background(), fmt.Sprintf("launcher-bounds applied: display=%q scale=%.2f requestedLogical=%+v targetLogical=%+v actualLogical=%+v actualDisplay=%q actualScale=%.2f boundsError=%v displayError=%v", targetDisplay.ID, targetDisplay.Scale, requested, target, actual, actualDisplay.ID, actualDisplay.Scale, boundsErr, displayErr))
	return nil
}

func launcherReservesFullPreviewHeight(params showAppParams, previewVisible bool) bool {
	return previewVisible || params.ShowPreviewTitleBar && params.HideQueryBox && params.HideToolbar
}

// launcherToolbarHasContent keeps the footer for result actions, status, or an open about menu.
// A query with no results would only show the about-menu button, so that strip stays hidden
// until the window grows with the results.
func launcherToolbarHasContent(resultCount int, messageVisible, overlayOpen bool) bool {
	return resultCount > 0 || messageVisible || overlayOpen
}

// launcherToolbarHeightIncluded preserves the hidden toolbar's space only in Flutter's chat mode.
func launcherToolbarHeightIncluded(hideToolbar, hasContent, previewFullscreen, chatFullscreen bool) bool {
	return !hideToolbar && hasContent && (!previewFullscreen || chatFullscreen)
}

// launcherWindowOrigin keeps user-moved windows in place while preserving a bottom query box during height changes.
func launcherWindowOrigin(params showAppParams, current woxui.Rect, targetHeight float32, useShowPosition bool, bottomAnchor float32) (x, y, nextAnchor float32) {
	if !params.QueryBoxAtBottom {
		if useShowPosition {
			return float32(params.Position.X), float32(params.Position.Y), 0
		}
		return current.X, current.Y, 0
	}

	x = current.X
	if useShowPosition {
		x = float32(params.Position.X)
	}
	nextAnchor = bottomAnchor
	switch {
	case useShowPosition && params.PositionHeight > 0:
		// Explorer/tray compute Position for an initial short height; remember the
		// implied bottom edge so later shows and result-driven resizes stay put.
		nextAnchor = float32(params.Position.Y + params.PositionHeight)
	case useShowPosition:
		nextAnchor = float32(params.Position.Y) + targetHeight
	case nextAnchor == 0:
		// Prefer the last requested logical height over Bounds() round-trip noise.
		nextAnchor = current.Y + current.Height
	}
	return x, nextAnchor - targetHeight, nextAnchor
}

// constrainLauncherHeightToWorkArea keeps the query chrome anchored while shrinking overflowing content into its scroll viewport.
func constrainLauncherHeightToWorkArea(queryAtBottom bool, x, y, width, height, minimumHeight, bottomAnchor float32, displays []screen.Display) (constrainedY, constrainedHeight, constrainedBottomAnchor float32) {
	anchorInset := max(float32(1), minimumHeight/2)
	anchorY := y + anchorInset
	if queryAtBottom {
		anchorY = bottomAnchor - anchorInset
	}
	workArea, ok := launcherWorkAreaAt(x+width/2, anchorY, displays)
	if !ok {
		return y, height, bottomAnchor
	}

	workTop := float32(workArea.Y)
	workBottom := float32(workArea.Bottom())
	minimumHeight = min(max(0, minimumHeight), float32(workArea.Height))
	if queryAtBottom {
		bottomAnchor = max(workTop+minimumHeight, min(bottomAnchor, workBottom))
		height = min(height, bottomAnchor-workTop)
		return bottomAnchor - height, height, bottomAnchor
	}

	y = max(workTop, min(y, workBottom-minimumHeight))
	height = min(height, workBottom-y)
	return y, height, bottomAnchor
}

// launcherWorkAreaAt resolves the nearest logical display work area for a launcher anchor.
func launcherWorkAreaAt(x, y float32, displays []screen.Display) (screen.Rect, bool) {
	if len(displays) == 0 {
		return screen.Rect{}, false
	}

	best := screen.Rect{}
	bestDistance := float32(-1)
	bestPrimary := false
	for _, display := range displays {
		bounds := display.Bounds
		if bounds.IsEmpty() {
			bounds = display.WorkArea
		}
		if bounds.IsEmpty() {
			continue
		}
		dx := max(float32(bounds.X)-x, 0, x-float32(bounds.Right()))
		dy := max(float32(bounds.Y)-y, 0, y-float32(bounds.Bottom()))
		distance := dx*dx + dy*dy
		if bestDistance >= 0 && (distance > bestDistance || distance == bestDistance && bestPrimary) {
			continue
		}
		best = display.WorkArea
		if best.IsEmpty() {
			best = bounds
		}
		bestDistance = distance
		bestPrimary = display.Primary
	}
	return best, bestDistance >= 0
}

// launcherBoundsEffectivelyEqual tolerates DPI round-trip noise from SetBounds/Bounds.
func launcherBoundsEffectivelyEqual(current, target woxui.Rect) bool {
	const epsilon = float32(0.75)
	return absFloat32(current.X-target.X) < epsilon && absFloat32(current.Y-target.Y) < epsilon && absFloat32(current.Width-target.Width) < epsilon && absFloat32(current.Height-target.Height) < epsilon
}

func absFloat32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func (a *App) onKey(event woxui.KeyEvent) bool {
	if event.Composing {
		return false
	}
	if !event.Down {
		return a.quickSelect.onQuickSelectKey(event)
	}
	if !a.hotkeyRecordingUsesSettingsWindow() && a.hotkeyRecording.onHotkeyRecordingKey(event) {
		return true
	}
	if !a.formTables.formTableUsesSettingsWindow() && a.formTables.launcherEditor != nil {
		return a.formTables.onFormTableKey(event)
	}
	// Handle the hold state before feature-local handlers can consume the key.
	if a.quickSelect.onQuickSelectKey(event) {
		return true
	}
	if a.forms.onFormKey(event) {
		return true
	}
	if a.requirements.onRequirementFormKey(event) {
		return true
	}
	if a.actions.onActionKey(event) {
		return true
	}
	if hotkeyMatches(primaryHotkey("p"), event) && !event.Repeat && a.toggleSelectedPreview() {
		return true
	}
	if a.conflicts.onTriggerConflictPreviewKey(event) {
		return true
	}
	if a.chat.onChatPreviewKey(event) {
		return true
	}
	if a.webView.onWebViewPreviewModeKey(event) {
		return true
	}
	if a.terminal.onTerminalPreviewKey(event) {
		return true
	}
	if a.onFilePreviewKey(event) {
		return true
	}
	if a.toolbar.onToolbarKey(event) {
		return true
	}
	if a.querySession.onQueryHintKey(event) {
		return true
	}
	if event.Key == woxui.KeyTab {
		if event.Modifiers == woxui.KeyModifierShift {
			a.querySession.autoCompleteQueryFromSelectedResult()
			return true
		}
		if event.Modifiers == 0 {
			if !a.querySession.acceptQueryCompletionHint() {
				a.querySession.rejectQueryTab()
			}
			return true
		}
	}
	if event.Key == woxui.Key("f") && event.Modifiers.HasPrimary() && a.toggleRefinementBar() {
		return true
	}
	if event.Key == woxui.Key("u") && event.Modifiers.HasPrimary() && a.attention.activateAttentionUnread() {
		return true
	}
	if a.onRefinementHotkey(event) {
		return true
	}
	if a.actions.onResultActionHotkey(event) {
		return true
	}
	if a.resultList.layout.GridLayout != nil {
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
			if a.querySession.queryHintEditorState.allSelected {
				_ = clipboard.WriteText(a.querySession.query.QueryText)
				return true
			}
			if selected := a.querySession.editor.SelectedText(); selected != "" {
				_ = clipboard.WriteText(selected)
			}
			return true
		case woxui.Key("x"):
			if a.querySession.queryHintEditorState.allSelected {
				if clipboard.WriteText(a.querySession.query.QueryText) == nil {
					a.querySession.replaceWholeQueryHint("")
				}
				return true
			}
			previousText := a.querySession.editor.State().Text
			selected := a.querySession.editor.SelectedText()
			if selected != "" {
				_ = clipboard.WriteText(selected)
				if a.querySession.editor.DeleteSelection() {
					a.applyQueryTextChangeLocked(a.querySession.editor.State().Text)
				}
			}
			_ = a.window.Invalidate()
			a.previews.reconcileSelectedPreview()
			if err := a.sendCurrentQuery(); err != nil {
				log.Printf("send query after cut: %v", err)
			}
			a.resizeLauncherForQueryLineChange(previousText)
			return true
		case woxui.Key("v"):
			text, err := clipboard.ReadText()
			text = normalizeQueryPaste(text)
			if err != nil || text == "" {
				return true
			}
			if a.querySession.queryHintEditorState.allSelected {
				a.querySession.replaceWholeQueryHint(text)
				return true
			}
			previousText := a.querySession.editor.State().Text
			if a.querySession.editor.InsertTextSeparate(text) {
				a.applyQueryTextChangeLocked(a.querySession.editor.State().Text)
			}
			_ = a.window.Invalidate()
			a.previews.reconcileSelectedPreview()
			if err := a.sendCurrentQuery(); err != nil {
				log.Printf("send query after paste: %v", err)
			}
			a.resizeLauncherForQueryLineChange(previousText)
			return true
		}
	}
	// Intercept before the query editor so the group-jump shortcut cannot move the caret.
	if event.Modifiers == queryGroupJumpModifier() {
		switch event.Key {
		case woxui.KeyArrowUp:
			a.moveSelectionByGroup(-1)
			return true
		case woxui.KeyArrowDown:
			a.querySession.canRecallHistory = false
			a.moveSelectionByGroup(1)
			return true
		}
	}
	previousText := a.querySession.editor.State().Text
	if event.Down && !event.Composing && event.Key == woxui.KeyBackspace && event.Modifiers == 0 &&
		a.isPrimary && !a.show.HideQueryBox && previousText == "" && len(a.querySession.query.QueryScope.Plugins) > 0 {
		a.querySession.rememberQueryHint()
		a.clearQueryScopeLocked()
		_ = a.window.Invalidate()
		a.previews.reconcileSelectedPreview()
		if err := a.sendCurrentQuery(); err != nil {
			log.Printf("send query after clearing scope: %v", err)
		}
		return true
	}
	textHandled, textChanged := handleQueryEditorKey(a.querySession.editor, event)
	if textChanged {
		a.applyQueryTextChangeLocked(a.querySession.editor.State().Text)
	}
	if textHandled {
		_ = a.window.Invalidate()
		if textChanged {
			a.previews.reconcileSelectedPreview()
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
			query, handled := a.querySession.previousQueryHistory()
			if handled {
				if query != nil {
					a.setQuery(*query)
					a.querySession.selectEntireQuery()
					if err := a.sendCurrentQuery(); err != nil {
						log.Printf("send recalled query history: %v", err)
					}
				}
				return true
			}
		}
		a.moveSelection(-a.resultList.resultNavigationColumns())
		return true
	case woxui.KeyArrowDown:
		if event.Modifiers == 0 {
			a.querySession.canRecallHistory = false
		}
		a.moveSelection(a.resultList.resultNavigationColumns())
		return true
	case woxui.KeyEnter:
		a.activateSelected()
		return true
	case woxui.KeyEscape:
		if a.host != nil && a.host.HasOverlay() && a.host.OverlayOwner() == launcherview.LauncherQueryInputKey {
			a.host.ClearOverlay(launcherview.LauncherQueryInputKey, 0)
			return true
		}
		util.Go(a.lifecycleCtx, "hide launcher from escape", func() {
			if err := a.hideWindow(true); err != nil {
				log.Printf("hide launcher: %v", err)
			}
		})
		return true
	default:
		return false
	}
}

// clearQueryScopeLocked exits plugin-scoped query mode on the primary launcher.
func (a *App) clearQueryScopeLocked() {
	a.querySession.query.QueryScope = queryScope{}
	a.querySession.query.QueryRefinements = map[string]string{}
	a.resultList.clearRefinements()
	a.resultList.layout = queryLayout{}
	a.beginQueryGenerationLocked()
	a.querySession.beginQueryTransitionLocked(false)
	a.glance.stopGlanceLocked(false)
}

func handleQueryEditorKey(editor *woxui.TextEditor, event woxui.KeyEvent) (bool, bool) {
	if editor != nil && event.Down && !event.Composing && event.Key == woxui.KeyEnter && event.Modifiers == woxui.KeyModifierShift {
		return true, editor.InsertText("\n")
	}
	return editor.HandleKey(event)
}

// resizeLauncherForQueryLineChange avoids native window work for ordinary single-line edits.
func (a *App) resizeLauncherForQueryLineChange(previousText string) {
	if launcherQueryLineCount(previousText) == launcherQueryLineCount(a.querySession.editor.State().Text) {
		return
	}
	if err := a.applyWindowBounds(); err != nil {
		log.Printf("resize launcher after query line change: %v", err)
	}
}

func (a *App) onTextInput(event woxui.TextInputEvent) {
	if a.actions.onActionTextInput(event) {
		return
	}
	if !a.formTables.formTableUsesSettingsWindow() && a.formTables.onFormTableTextInput(event) {
		return
	}
	if a.forms.onFormTextInput(event) {
		return
	}
	if a.requirements.onRequirementFormTextInput(event) {
		return
	}
	if a.conflicts.onTriggerConflictPreviewTextInput(event) {
		return
	}
	if a.chat.onChatPreviewTextInput(event) {
		return
	}
	if a.terminal.onTerminalPreviewTextInput(event) {
		return
	}
	previousText := a.querySession.editor.State().Text
	if event.Kind == woxui.TextInputCommit {
		event.Text = normalizeQueryNewlines(event.Text)
	}
	if a.querySession.queryHintEditorState.allSelected && event.Kind == woxui.TextInputCommit {
		a.querySession.replaceWholeQueryHint(event.Text)
		return
	}
	a.querySession.selectTouchedQueryBlocks()
	committed := a.querySession.editor.HandleTextInput(event)
	if committed {
		a.applyQueryTextChangeLocked(a.querySession.editor.State().Text)
	}
	_ = a.window.Invalidate()
	if committed {
		a.previews.reconcileSelectedPreview()
		if err := a.sendCurrentQuery(); err != nil {
			log.Printf("send committed query: %v", err)
		}
		a.resizeLauncherForQueryLineChange(previousText)
	}
}

func normalizeQueryNewlines(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

// normalizeQueryPaste drops copied line endings at the edges while preserving internal lines and spaces.
func normalizeQueryPaste(text string) string {
	return strings.Trim(normalizeQueryNewlines(text), "\n")
}

func (a *App) moveSelection(delta int) {
	if len(a.resultList.results) == 0 {
		return
	}
	target := a.resultList.selected
	columns := a.resultList.resultNavigationColumns()
	if a.resultList.layout.GridLayout != nil && (delta == columns || delta == -columns) {
		direction := 1
		if delta < 0 {
			direction = -1
		}
		target = gridSelectionIndex(a.resultList.results, a.resultList.selected, columns, direction)
	} else {
		index := a.resultList.selected
		for attempts := 0; attempts < len(a.resultList.results); attempts++ {
			index = (index + delta + len(a.resultList.results)) % len(a.resultList.results)
			if !a.resultList.results[index].IsGroup {
				target = index
				break
			}
		}
	}
	if target >= 0 && target < len(a.resultList.results) && !a.resultList.results[target].IsGroup {
		a.cancelQuerySelectionRestore(target)
	}
	if target != a.resultList.selected {
		a.resultList.selected = target
		a.resultList.resultScrollDetached = false
		a.actions.clearActionPanelStateLocked()
		a.chat.chatFullscreen = false
		a.webView.clearWebViewPreviewModeLocked()
		a.previews.reconcileSelectedPreview()
		a.restoreQueryTextInput()
		_ = a.applyWindowBounds()
	}
	_ = a.window.Invalidate()
}

// moveSelectionByGroup jumps by result group. Down goes to the next group's
// first item, or the current group's last item when no later group exists. Up
// first returns to the current group's first item, then to the previous group's
// first item. Lists without group headers still treat the whole list as one group.
func (a *App) moveSelectionByGroup(direction int) {
	if len(a.resultList.results) == 0 {
		return
	}
	target := groupSelectionIndex(a.resultList.results, a.resultList.selected, direction)
	if target >= 0 && target < len(a.resultList.results) && !a.resultList.results[target].IsGroup {
		a.cancelQuerySelectionRestore(target)
	}
	if target != a.resultList.selected {
		a.resultList.selected = target
		a.resultList.resultScrollDetached = false
		a.actions.clearActionPanelStateLocked()
		a.chat.chatFullscreen = false
		a.webView.clearWebViewPreviewModeLocked()
		a.previews.reconcileSelectedPreview()
		a.restoreQueryTextInput()
		_ = a.applyWindowBounds()
	}
	_ = a.window.Invalidate()
}

func (a *App) selectResult(index int) {
	closedPanel := false
	closedForm := false
	valid := false
	if index >= 0 && index < len(a.resultList.results) && !a.resultList.results[index].IsGroup {
		valid = true
		a.cancelQuerySelectionRestore(index)
		changed := a.resultList.selected != index
		a.resultList.selected = index
		closedPanel = a.actions.resetActionPanelLocked()
		closedForm = a.forms.form != nil
		a.forms.form = nil
		if changed {
			a.resultList.resultScrollDetached = false
			a.chat.chatFullscreen = false
			a.webView.clearWebViewPreviewModeLocked()
		}
	}
	if valid {
		a.previews.reconcileSelectedPreview()
		a.restoreQueryTextInput()
	}
	if valid || closedPanel || closedForm {
		_ = a.applyWindowBounds()
	}
	_ = a.window.Invalidate()
}

func (a *App) activateSelected() {
	selected := a.resultList.selected
	a.activateResult(selected)
}

func (a *App) activateResult(index int) {
	if len(a.resultList.results) > 0 && (index < 0 || index >= len(a.resultList.results) || a.resultList.results[index].IsGroup) {
		return
	}
	entries := unifiedActionPanelEntries(a.resultList.results, index, a.toolbar.toolbarMsg)
	if len(entries) == 0 {
		return
	}
	var entry actionPanelEntry
	found := false
	for _, candidate := range entries {
		if candidate.Source == actionPanelSourceLocal {
			continue
		}
		if !found {
			entry = candidate
			found = true
		}
		if candidate.IsDefault {
			entry = candidate
			break
		}
	}
	if found {
		a.activateActionPanelEntry(entry)
	}
}

func selectableIndex(results []queryResult) int {
	for index, result := range results {
		if !result.IsGroup {
			return index
		}
	}
	return -1
}

// queryGroupJumpModifier is Option on macOS so it does not collide with Cocoa
// Cmd+Up/Down document caret movement, and Control elsewhere to avoid
// Mission Control and desktop switching.
func queryGroupJumpModifier() woxui.KeyModifiers {
	if runtime.GOOS == "darwin" {
		return woxui.KeyModifierAlt
	}
	return woxui.KeyModifierControl
}

// groupSelectionIndex returns the selectable index for group jumps.
func groupSelectionIndex(results []queryResult, current, direction int) int {
	if len(results) == 0 {
		return current
	}
	starts := selectableGroupStarts(results)
	if len(starts) == 0 {
		return current
	}
	if current < 0 {
		current = 0
	} else if current >= len(results) {
		current = len(results) - 1
	}
	group := 0
	for index, start := range starts {
		if start <= current {
			group = index
			continue
		}
		break
	}
	if direction > 0 {
		if group+1 < len(starts) {
			return starts[group+1]
		}
		return lastSelectableBefore(results, groupEndExclusive(starts, group, len(results)))
	}
	start := starts[group]
	if current > start {
		return start
	}
	if group > 0 {
		return starts[group-1]
	}
	return start
}

func selectableGroupStarts(results []queryResult) []int {
	starts := make([]int, 0)
	start := -1
	for index, result := range results {
		if result.IsGroup {
			if start >= 0 {
				starts = append(starts, start)
				start = -1
			}
			continue
		}
		if start < 0 {
			start = index
		}
	}
	if start >= 0 {
		starts = append(starts, start)
	}
	return starts
}

func groupEndExclusive(starts []int, group, resultCount int) int {
	if group+1 < len(starts) {
		return starts[group+1]
	}
	return resultCount
}

func lastSelectableBefore(results []queryResult, exclusiveEnd int) int {
	for index := min(exclusiveEnd, len(results)) - 1; index >= 0; index-- {
		if !results[index].IsGroup {
			return index
		}
	}
	return 0
}

// selectableIndexFrom restores an explicitly preserved refresh index while skipping group rows.
func selectableIndexFrom(results []queryResult, start int) int {
	for index := max(0, start); index < len(results); index++ {
		if !results[index].IsGroup {
			return index
		}
	}
	// A removed final row keeps selection near the end instead of jumping to the top.
	for index := len(results) - 1; index >= 0; index-- {
		if !results[index].IsGroup {
			return index
		}
	}
	return selectableIndex(results)
}

// resultIndexByID finds a refreshed result by the stable id the plugin set.
// Group headers are not selectable, so they never count as a match.
func resultIndexByID(results []queryResult, id string) int {
	if id == "" {
		return -1
	}
	for index, result := range results {
		if !result.IsGroup && result.ID == id {
			return index
		}
	}
	return -1
}

// restoreRefreshSelection applies one refresh snapshot.
// A result id stays pending until the query is complete so a later snapshot can
// still follow the item. Index preservation is applied once, matching the previous refresh.
// keepPending reports whether the caller should retain pending for the next snapshot.
func restoreRefreshSelection(results []queryResult, pending *pendingResultSelection, queryID string, complete bool) (selected int, preserved bool, keepPending bool) {
	selected = selectableIndex(results)
	if pending == nil || pending.queryID != queryID {
		return selected, false, false
	}
	if pending.querySnapshot != nil {
		return restoreQueryResultSelection(results, *pending.querySnapshot), true, !complete
	}
	if pending.resultID != "" {
		if index := resultIndexByID(results, pending.resultID); index >= 0 {
			return index, true, !complete
		}
		if pending.preserveIndex {
			return selectableIndexFrom(results, pending.index), true, !complete
		}
		return selected, false, !complete
	}
	return selectableIndexFrom(results, pending.index), true, false
}

// restoreQueryResultSelection follows matching text before falling back to the old row or first result.
func restoreQueryResultSelection(results []queryResult, snapshot queryResultSelection) int {
	if snapshot.index >= 0 && snapshot.index < len(results) {
		result := results[snapshot.index]
		if !result.IsGroup && result.Title == snapshot.title && result.SubTitle == snapshot.subTitle {
			return snapshot.index
		}
	}
	for index, result := range results {
		if !result.IsGroup && result.Title == snapshot.title && result.SubTitle == snapshot.subTitle {
			return index
		}
	}
	if snapshot.index >= 0 && snapshot.index < len(results) {
		for index := snapshot.index; index < len(results); index++ {
			if !results[index].IsGroup {
				return index
			}
		}
	}
	return selectableIndex(results)
}

// cancelQuerySelectionRestore lets explicit navigation override a pending undo target.
func (a *App) cancelQuerySelectionRestore(index int) {
	if a.resultList.pendingSelection != nil && a.resultList.pendingSelection.querySnapshot != nil {
		// IDs remain stable within this query, so later snapshots can retain the user's choice.
		a.resultList.pendingSelection = &pendingResultSelection{
			queryID: a.querySession.query.QueryID, index: index, resultID: a.resultList.results[index].ID, preserveIndex: true,
		}
	}
}
