package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	woxui "github.com/Wox-launcher/wox.ui.go"
	"github.com/Wox-launcher/wox.ui.go/coreclient"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

const (
	defaultWidth     = 760
	defaultMaxResult = 10
	headerHeight     = 88
	footerHeight     = 44
	resultRowHeight  = 58
	resultRowGap     = 8
)

type viewMode uint8

const (
	viewLauncher viewMode = iota
	viewSettings
)

// App owns the launcher window, query state, and Wox core protocol client.
type App struct {
	mu                     sync.RWMutex
	terminalSubscriptionMu sync.Mutex
	terminalSubscribed     string

	port      int
	isDev     bool
	sessionID string
	client    *coreclient.Client
	window    *woxui.Window
	host      *woxwidget.Host

	query                plainQuery
	queryContext         queryContext
	queryContextKnown    bool
	editor               *woxui.TextEditor
	results              []queryResult
	selected             int
	layout               queryLayout
	refinements          []queryRefinement
	refinementOpen       bool
	refinementScope      string
	completionHint       *queryCompletionHint
	toolbarMsg           *toolbarMessage
	toolbarRevision      uint64
	form                 *formState
	requirementForm      *requirementFormState
	triggerConflict      *triggerConflictPreviewState
	themeEditor          *themeEditorPreviewState
	chatPreview          *chatPreviewState
	webViewPreviewData   string
	webViewPreviewError  string
	chatFullscreen       bool
	actionPanel          bool
	actionSelected       int
	visible              bool
	show                 showAppParams
	pendingMRU           string
	mode                 viewMode
	settings             settingsData
	settingsCtx          settingWindowContext
	settingTab           string
	settingRow           int
	settingNote          string
	settingSaving        bool
	settingEditKey       string
	settingEditor        *woxui.TextEditor
	settingChoicePicker  *settingChoicePickerState
	settingPageScroll    float32
	settingPageViewport  float32
	settingRailScroll    float32
	settingRailViewport  float32
	systemFontFamilies   []string
	systemFontsLoading   bool
	systemFontsLoaded    bool
	systemFontsError     string
	plugins              []pluginSettingsPlugin
	pluginsLoading       bool
	pluginsLoaded        bool
	pluginsError         string
	pluginSelected       int
	pluginListScroll     float32
	pluginListViewport   float32
	pluginForm           *pluginSettingsFormState
	pluginsStore         bool
	pluginOperation      string
	pluginOperationError string
	pluginUninstallArmed string
	hotkeySettingsForm   *formFieldsState
	hotkeyRecording      *hotkeyRecordingState
	hotkeyAppCandidates  []ignoredHotkeyApp
	hotkeyAppsLoading    bool
	hotkeyAppsLoaded     bool
	hotkeyAppsError      string
	themes               []themeSettingsTheme
	themesMode           string
	themesLoading        bool
	themesLoaded         bool
	themesError          string
	themeSelected        int
	themeListScroll      float32
	themeListViewport    float32
	themeOperation       string
	themeUninstallArmed  string
	aiSettingsForm       *formFieldsState
	aiProviderCatalog    []aiProviderInfo
	aiProvidersLoading   bool
	aiProvidersLoaded    bool
	aiProvidersError     string
	tableEditor          *formTableEditorState
	modelManager         *modelManagerState
	usageStats           usageStatsData
	usagePeriod          string
	usageLoading         bool
	usageLoaded          bool
	usageError           string
	usageRevision        uint64
	aboutVersion         string
	aboutLoading         bool
	aboutLoaded          bool
	aboutError           string
	privacySample        string
	privacyError         string
	dataBackups          []backupInfo
	dataLocation         string
	dataLoading          bool
	dataLoaded           bool
	dataBusy             string
	dataError            string
	dataRestoreArmed     string
	dataPendingLocation  string
	dataClearLogsArmed   bool
	dataListScroll       float32
	dataListViewport     float32
	runtimeStatuses      []runtimeStatus
	runtimeLoading       bool
	runtimeLoaded        bool
	runtimeError         string
	runtimeRestarting    string
	runtimeRevision      uint64
	runtimePageScroll    float32
	runtimePageViewport  float32
	runtimePageContent   float32
	runtimeRowsTop       float32
	cloudAccount         cloudAccountStatus
	cloudSync            cloudSyncStatus
	cloudDevices         cloudDeviceList
	cloudLoading         bool
	cloudLoaded          bool
	cloudBusy            string
	cloudError           string
	cloudRevision        uint64
	cloudPageScroll      float32
	cloudPageViewport    float32
	cloudPageContent     float32
	cloudForm            *cloudFormState
	cloudPlugins         []pluginSettingsPlugin
	cloudPluginScroll    float32
	cloudPluginViewport  float32
	glanceItem           *glanceItem
	glanceLoading        bool
	glanceRevision       uint64
	glanceTimer          *time.Timer
	glanceCatalog        []glanceCatalogItem
	glanceCatalogLoading bool
	glanceCatalogLoaded  bool
	glanceCatalogError   string
	palette              uiPalette
	translations         map[string]string
	images               map[string]*woxui.Image
	imageRequested       map[string]bool
	imageErrors          map[string]string
	remotePreviews       map[string]queryPreview
	previewRequests      map[string]bool
	filePreviews         map[string]filePreviewContent
	fileRequests         map[string]bool
	previewScroll        map[string]float32
	previewLayouts       map[string]woxwidget.TextBlockLayout
	terminalPreview      *terminalPreviewState
	aiModels             []aiModel
	aiModelsLoading      bool
	aiModelsLoaded       bool
	aiModelsError        string
	aiSkills             []chatSkill
	aiSkillsLoading      bool
	aiSkillsLoaded       bool
	aiSkillsError        string
}

// New creates a hidden launcher app connected to the given Wox core port when Start runs.
func New(port int, isDev bool) *App {
	return &App{
		port:            port,
		isDev:           isDev,
		sessionID:       coreclient.NewID(),
		query:           newInputQuery(""),
		editor:          woxui.NewTextEditor(""),
		selected:        -1,
		mode:            viewLauncher,
		settingTab:      "general",
		usagePeriod:     "30d",
		palette:         defaultPalette(),
		translations:    map[string]string{},
		images:          map[string]*woxui.Image{},
		imageRequested:  map[string]bool{},
		imageErrors:     map[string]string{},
		remotePreviews:  map[string]queryPreview{},
		previewRequests: map[string]bool{},
		filePreviews:    map[string]filePreviewContent{},
		fileRequests:    map[string]bool{},
		previewScroll:   map[string]float32{},
		previewLayouts:  map[string]woxwidget.TextBlockLayout{},
		show: showAppParams{
			WindowWidth:    defaultWidth,
			MaxResultCount: defaultMaxResult,
			StartPage:      "mru",
		},
	}
}

// Start connects to core and creates the hidden native window on the UI runtime thread.
func (a *App) Start() error {
	a.client = coreclient.New(a.port, a.sessionID, a.handleRequest, a.handleResponse, func(err error) {
		log.Printf("Wox core connection failed: %v", err)
	})
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

	a.host = woxwidget.NewHost(a.build)
	window, err := woxui.Open(woxui.WindowOptions{
		Title:       "Wox",
		Size:        woxui.Size{Width: defaultWidth, Height: headerHeight + footerHeight},
		OnFrame:     a.host.Frame,
		OnPointer:   a.host.Pointer,
		OnKey:       a.onKey,
		OnTextInput: a.onTextInput,
		OnFocus:     a.onFocus,
	})
	if err != nil {
		_ = a.client.Close()
		return err
	}
	a.window = window
	a.host.Attach(window)
	if err := window.SetFontFamily(a.settings.AppFontFamily); err != nil {
		return fmt.Errorf("apply Wox UI font: %w", err)
	}
	if err := window.SetTextInputState(woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: 130, Y: 29, Width: 1, Height: 24}}); err != nil {
		return err
	}

	readyContext, cancelReady := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelReady()
	if err := a.client.Post(readyContext, "/on/ready", map[string]any{"Pid": os.Getpid()}, nil); err != nil {
		return fmt.Errorf("notify Wox core that Go UI is ready: %w", err)
	}
	return nil
}

// Close releases the protocol connection after the final native window closes.
func (a *App) Close() error {
	if a.client == nil {
		return nil
	}
	return a.client.Close()
}

func (a *App) handleRequest(message coreclient.Message) (any, error) {
	switch message.Method {
	case "ShowApp":
		var params showAppParams
		if err := json.Unmarshal(message.Data, &params); err != nil {
			return nil, err
		}
		return nil, a.showWindow(params)
	case "HideApp":
		return nil, a.hideWindow(true)
	case "ToggleApp":
		a.mu.RLock()
		visible := a.visible
		a.mu.RUnlock()
		if visible {
			return nil, a.hideWindow(true)
		}
		var params showAppParams
		if err := json.Unmarshal(message.Data, &params); err != nil {
			return nil, err
		}
		return nil, a.showWindow(params)
	case "ChangeQuery":
		var query plainQuery
		if err := json.Unmarshal(message.Data, &query); err != nil {
			return nil, err
		}
		a.setQuery(query)
		return nil, a.sendCurrentQuery()
	case "RefreshQuery":
		a.mu.Lock()
		a.query.QueryID = coreclient.NewID()
		a.queryContext = queryContext{}
		a.queryContextKnown = false
		a.completionHint = nil
		a.stopGlanceLocked(true)
		a.mu.Unlock()
		return nil, a.sendCurrentQuery()
	case "GetCurrentQuery":
		a.mu.RLock()
		query := a.query
		a.mu.RUnlock()
		return query, nil
	case "OpenSettingWindow":
		var windowContext settingWindowContext
		if err := json.Unmarshal(message.Data, &windowContext); err != nil {
			return nil, err
		}
		return nil, a.openSettings(windowContext)
	case "FocusSettingWindow":
		_, err := a.window.Show()
		return nil, err
	case "ReloadSetting":
		if err := a.reloadSettings(); err != nil {
			return nil, err
		}
		if err := a.reloadTranslations(); err != nil {
			return nil, err
		}
		a.mu.Lock()
		a.stopGlanceLocked(true)
		refreshGlance := a.glanceEligibleLocked()
		a.mu.Unlock()
		if refreshGlance {
			go a.refreshGlance("settingsChanged", "", nil)
		}
		return nil, nil
	case "CloudSyncProgressChanged":
		return nil, a.applyCloudSyncProgress(message.Data)
	case "RefreshAccountStatus":
		go a.reloadCloudSync()
		return nil, nil
	case "PickFiles":
		var params struct {
			IsDirectory bool `json:"IsDirectory"`
		}
		if err := json.Unmarshal(message.Data, &params); err != nil {
			return nil, err
		}
		path, err := a.window.PickFile(woxui.FileDialogOptions{Directory: params.IsDirectory})
		if err != nil {
			return nil, err
		}
		if path == "" {
			return []string{}, nil
		}
		return []string{path}, nil
	case "WriteClipboardImageFile":
		var params struct {
			FilePath string `json:"filePath"`
		}
		if err := json.Unmarshal(message.Data, &params); err != nil {
			return nil, err
		}
		return nil, a.window.WriteClipboardImageFile(params.FilePath)
	case "UpdateResult":
		return a.updateResult(message.Data)
	case "PushResults":
		return a.pushResults(message.Data)
	case "ChangeTheme":
		return nil, a.changeTheme(message.Data)
	case "RecordHotkey":
		return nil, a.receiveRecordedHotkey(message.Data)
	case "ShowToolbarMsg":
		return nil, a.showToolbarMessage(message.Data)
	case "ClearToolbarMsg":
		return nil, a.clearToolbarMessage(message.Data)
	case "TerminalChunk":
		var chunk terminalChunk
		if err := json.Unmarshal(message.Data, &chunk); err != nil {
			return nil, err
		}
		a.applyTerminalChunk(chunk)
		return nil, nil
	case "TerminalState":
		var state terminalSessionState
		if err := json.Unmarshal(message.Data, &state); err != nil {
			return nil, err
		}
		a.applyTerminalState(state)
		return nil, nil
	case "SendChatResponse":
		var chat chatData
		if err := json.Unmarshal(message.Data, &chat); err != nil {
			return nil, err
		}
		a.applyChatResponse(chat)
		return nil, nil
	case "AIQuestion":
		return nil, a.applyAIQuestion(message.Data)
	case "ReloadChatResources":
		a.reloadChatResource(message.Data)
		return nil, nil
	case "RefreshGlance":
		return nil, a.handleRefreshGlance(message.Data)
	case "ReloadSettingPlugins":
		go a.reloadGlanceCatalogFromCore()
		return nil, nil
	case "DiagnosticStatusChanged", "AttentionUnreadCountChanged", "ReloadSettingThemes":
		return nil, nil
	default:
		return nil, fmt.Errorf("Go UI has not implemented %s yet", message.Method)
	}
}

func (a *App) handleResponse(message coreclient.Message) {
	if !message.Success {
		log.Printf("Wox core method %s failed: %s", message.Method, string(message.Data))
		return
	}
	if message.Method == "Query" {
		var response queryResponse
		if err := json.Unmarshal(message.Data, &response); err != nil {
			log.Printf("decode query response: %v", err)
			return
		}
		a.applyResults(response.QueryID, response.Results, &response.Layout, &response.Refinements, &response.Context, response.QueryStartTimestamp)
		return
	}
	if message.Method == "QueryCompletionHint" {
		a.applyQueryCompletionHint(message.Data)
		return
	}

	a.mu.Lock()
	isMRU := message.RequestID != "" && message.RequestID == a.pendingMRU
	if isMRU {
		a.pendingMRU = ""
	}
	queryID := a.query.QueryID
	a.mu.Unlock()
	if !isMRU {
		return
	}
	var results []queryResult
	if err := json.Unmarshal(message.Data, &results); err != nil {
		log.Printf("decode MRU response: %v", err)
		return
	}
	for index := range results {
		results[index].QueryID = queryID
	}
	a.applyResults(queryID, results, nil, nil, nil, 0)
}

func (a *App) showWindow(params showAppParams) error {
	a.mu.Lock()
	wasSettings := a.mode == viewSettings
	a.mode = viewLauncher
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
	a.form = nil
	a.requirementForm = nil
	a.visible = true
	queryEmpty := a.query.QueryText == ""
	a.mu.Unlock()
	a.restoreQueryTextInput()
	if wasSettings {
		if err := a.postEvent("/on/setting", map[string]any{"inSettingView": false}); err != nil {
			return err
		}
	}

	if err := a.window.SetHideOnBlur(params.HideOnBlur); err != nil {
		return err
	}
	if err := a.applyWindowBounds(); err != nil {
		return err
	}
	if _, err := a.window.Show(); err != nil {
		return err
	}
	if err := a.postEvent("/on/show", map[string]any{}); err != nil {
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
	wasSettings := a.mode == viewSettings
	a.mode = viewLauncher
	a.actionPanel = false
	a.actionSelected = 0
	a.form = nil
	a.requirementForm = nil
	a.triggerConflict = nil
	a.themeEditor = nil
	a.visible = false
	a.stopGlanceLocked(false)
	a.mu.Unlock()
	a.deactivateTerminalPreview()
	a.resetChatPreview()
	if err := a.window.Hide(); err != nil {
		return err
	}
	if notify {
		if wasSettings {
			if err := a.postEvent("/on/setting", map[string]any{"inSettingView": false}); err != nil {
				return err
			}
		}
		return a.postEvent("/on/hide", map[string]any{})
	}
	return nil
}

func (a *App) onFocus(event woxui.FocusEvent) {
	if event.Active {
		return
	}
	a.mu.Lock()
	if !a.visible || a.mode != viewLauncher {
		a.mu.Unlock()
		return
	}
	hideOnBlur := a.show.HideOnBlur
	if hideOnBlur {
		a.visible = false
		a.stopGlanceLocked(false)
		if a.requirementForm != nil {
			a.requirementForm.active = false
		}
		if a.triggerConflict != nil {
			a.triggerConflict.active = false
		}
		if a.themeEditor != nil {
			a.themeEditor.active = false
		}
	}
	a.mu.Unlock()
	if hideOnBlur {
		a.deactivateTerminalPreview()
		a.resetChatPreview()
	}
	go func() {
		if hideOnBlur {
			if err := a.postEvent("/on/hide", map[string]any{}); err != nil {
				log.Printf("notify Wox core after blur hide: %v", err)
			}
			return
		}
		if err := a.postEvent("/on/focus/lost", map[string]any{}); err != nil {
			log.Printf("notify Wox core after focus loss: %v", err)
		}
	}()
}

func (a *App) postEvent(path string, data any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return a.client.Post(ctx, path, data, nil)
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
	a.results = nil
	a.selected = -1
	a.layout = queryLayout{}
	a.stopGlanceLocked(true)
	a.refinements = nil
	a.refinementOpen = false
	a.refinementScope = ""
	a.completionHint = nil
	a.actionPanel = false
	a.actionSelected = 0
	a.form = nil
	a.requirementForm = nil
	a.triggerConflict = nil
	a.themeEditor = nil
	a.mu.Unlock()
	a.deactivateTerminalPreview()
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
	_, err := a.client.SendRequest("Query", map[string]any{
		"queryId":            query.QueryID,
		"queryType":          query.QueryType,
		"queryText":          query.QueryText,
		"querySelection":     query.QuerySelection,
		"queryRefinements":   query.QueryRefinements,
		"contextData":        query.ContextData,
		"skipCompletionHint": skipCompletionHint,
	})
	if err != nil {
		return err
	}
	if query.QueryText == "" && startPage == "mru" {
		return a.requestMRU()
	}
	return nil
}

func (a *App) requestMRU() error {
	requestID := coreclient.NewID()
	a.mu.Lock()
	a.query = newInputQuery("")
	a.queryContext = queryContext{IsGlobalQuery: true}
	a.queryContextKnown = true
	a.editor.SetText("", false)
	queryID := a.query.QueryID
	a.results = nil
	a.selected = -1
	a.layout = queryLayout{}
	a.stopGlanceLocked(true)
	a.refinements = nil
	a.refinementOpen = false
	a.refinementScope = ""
	a.completionHint = nil
	a.actionPanel = false
	a.actionSelected = 0
	a.form = nil
	a.requirementForm = nil
	a.triggerConflict = nil
	a.themeEditor = nil
	a.pendingMRU = requestID
	a.mu.Unlock()
	a.deactivateTerminalPreview()
	a.resetChatPreview()
	if err := a.client.SendRequestWithID(requestID, "QueryMRU", map[string]any{"queryId": queryID}); err != nil {
		a.mu.Lock()
		if a.pendingMRU == requestID {
			a.pendingMRU = ""
		}
		a.mu.Unlock()
		return err
	}
	return nil
}

func (a *App) applyResults(queryID string, results []queryResult, layout *queryLayout, refinements *[]queryRefinement, context *queryContext, queryStartTimestamp int64) {
	a.mu.Lock()
	if queryID == "" || queryID != a.query.QueryID {
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
	a.results = results
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
	refreshGlance := a.glanceEligibleLocked() && a.glanceItem == nil && !a.glanceLoading
	if !a.glanceEligibleLocked() {
		a.stopGlanceLocked(true)
	}
	a.selected = selectableIndex(results, selectedID)
	if a.requirementForm != nil {
		if a.selected < 0 || a.selected >= len(results) || results[a.selected].Preview.PreviewType != "query_requirement_settings" || !strings.HasPrefix(a.requirementForm.key, queryID+"|"+results[a.selected].ID+"|") {
			a.requirementForm = nil
		}
	}
	if a.triggerConflict != nil {
		if a.selected < 0 || a.selected >= len(results) || results[a.selected].Preview.PreviewType != "trigger_keyword_conflict" || !strings.HasPrefix(a.triggerConflict.key, queryID+"|"+results[a.selected].ID+"|") {
			a.triggerConflict.active = false
		}
	}
	if a.themeEditor != nil {
		if a.selected < 0 || a.selected >= len(results) || results[a.selected].Preview.PreviewType != "theme_edit" || !strings.HasPrefix(a.themeEditor.key, queryID+"|"+results[a.selected].ID+"|") {
			a.themeEditor.active = false
		}
	}
	if a.chatPreview != nil {
		if a.selected < 0 || a.selected >= len(results) || results[a.selected].Preview.PreviewType != "chat" || !strings.HasPrefix(a.chatPreview.key, queryID+"|"+results[a.selected].ID+"|") {
			a.chatPreview.active = false
			a.chatFullscreen = false
		}
	}
	if a.actionPanel && (a.selected < 0 || a.selected >= len(results) || len(results[a.selected].Actions) == 0) {
		a.actionPanel = false
		a.actionSelected = 0
	} else if a.actionPanel && a.actionSelected >= len(results[a.selected].Actions) {
		a.actionSelected = len(results[a.selected].Actions) - 1
	}
	a.mu.Unlock()
	if refreshGlance {
		go a.refreshGlance("manualRefresh", "", nil)
	}
	if err := a.applyWindowBounds(); err != nil {
		log.Printf("resize launcher for query results: %v", err)
	}
	_ = a.window.Invalidate()
}

func (a *App) updateResult(raw json.RawMessage) (bool, error) {
	var update updatableResult
	if err := json.Unmarshal(raw, &update); err != nil {
		return false, err
	}
	a.mu.Lock()
	updated := false
	for index := range a.results {
		if a.results[index].ID != update.ID {
			continue
		}
		if update.Title != nil {
			a.results[index].Title = *update.Title
		}
		if update.SubTitle != nil {
			a.results[index].SubTitle = *update.SubTitle
		}
		if update.Icon != nil {
			a.results[index].Icon = *update.Icon
		}
		if update.Preview != nil {
			a.results[index].Preview = *update.Preview
		}
		if update.Tails != nil {
			a.results[index].Tails = *update.Tails
		}
		if update.Actions != nil {
			a.results[index].Actions = *update.Actions
		}
		updated = true
		break
	}
	a.mu.Unlock()
	if updated {
		_ = a.window.Invalidate()
	}
	return updated, nil
}

func (a *App) pushResults(raw json.RawMessage) (bool, error) {
	var payload struct {
		QueryID string        `json:"QueryId"`
		Results []queryResult `json:"Results"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false, err
	}
	a.mu.Lock()
	if payload.QueryID != a.query.QueryID {
		a.mu.Unlock()
		return false, nil
	}
	for index := range payload.Results {
		if payload.Results[index].QueryID == "" {
			payload.Results[index].QueryID = payload.QueryID
		}
	}
	a.results = append(a.results, payload.Results...)
	if a.selected < 0 {
		a.selected = selectableIndex(a.results, "")
	}
	a.mu.Unlock()
	if err := a.applyWindowBounds(); err != nil {
		return false, err
	}
	_ = a.window.Invalidate()
	return true, nil
}

func (a *App) applyWindowBounds() error {
	a.mu.RLock()
	params := a.show
	results := append([]queryResult(nil), a.results...)
	resultCount := len(results)
	layout := a.layout
	refinementVisible := len(a.refinements) > 0 && a.refinementOpen && !params.HideQueryBox
	actionPanel := a.actionPanel
	formHeight := formPanelHeight(a.form)
	actionCount := 0
	requirementPreview := false
	previewVisible := false
	if a.selected >= 0 && a.selected < len(a.results) {
		actionCount = len(a.results[a.selected].Actions)
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
	height := 0
	if !params.HideQueryBox {
		height += headerHeight
	}
	if refinementVisible {
		height += refinementBarHeight
	}
	if visibleResults > 0 {
		if layout.GridLayout != nil {
			height += gridResultsHeight(results[:visibleResults], float32(width), layout.GridLayout)
		} else {
			height += visibleResults*resultRowHeight + max(0, visibleResults-1)*resultRowGap
		}
	}
	if !params.HideToolbar {
		height += footerHeight
	}
	if previewVisible {
		previewHeight := maxResults*resultRowHeight + max(0, maxResults-1)*resultRowGap
		if !params.HideQueryBox {
			previewHeight += headerHeight
		}
		if refinementVisible {
			previewHeight += refinementBarHeight
		}
		if !params.HideToolbar {
			previewHeight += footerHeight
		}
		height = max(height, previewHeight)
	}
	if requirementPreview {
		minimumHeight := 360
		if !params.HideQueryBox {
			minimumHeight += headerHeight
		}
		if refinementVisible {
			minimumHeight += refinementBarHeight
		}
		if !params.HideToolbar {
			minimumHeight += footerHeight
		}
		height = max(height, minimumHeight)
	}
	if actionPanel && actionCount > 0 {
		actionHeight := 66 + min(actionCount, maxVisibleActions)*actionRowHeight + 20
		if !params.HideQueryBox {
			actionHeight += headerHeight
		}
		if refinementVisible {
			actionHeight += refinementBarHeight
		}
		if !params.HideToolbar {
			actionHeight += footerHeight
		}
		height = max(height, actionHeight)
	}
	if formHeight > 0 {
		formWindowHeight := formHeight + 20
		if !params.HideQueryBox {
			formWindowHeight += headerHeight
		}
		if refinementVisible {
			formWindowHeight += refinementBarHeight
		}
		if !params.HideToolbar {
			formWindowHeight += footerHeight
		}
		height = max(height, formWindowHeight)
	}
	if height <= 0 {
		height = resultRowHeight
	}
	return a.window.SetBounds(woxui.Rect{
		X:      float32(params.Position.X),
		Y:      float32(params.Position.Y),
		Width:  float32(width),
		Height: float32(height),
	})
}

func (a *App) onKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	if a.onHotkeyRecordingKey(event) {
		return true
	}
	if a.onFormTableKey(event) {
		return true
	}
	a.mu.RLock()
	mode := a.mode
	a.mu.RUnlock()
	if mode == viewSettings {
		return a.onSettingsKey(event)
	}
	if a.onFormKey(event) {
		return true
	}
	if a.onRequirementFormKey(event) {
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
	if a.onActionKey(event) {
		return true
	}
	if event.Key == woxui.KeyTab && a.acceptQueryCompletionHint() {
		return true
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
			a.deactivateTerminalPreview()
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
	if a.onFormTableTextInput(event) {
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
	if a.onCloudFormTextInput(event) {
		return
	}
	if a.onBuiltInSettingsTextInput(event) {
		return
	}
	if a.onPluginSettingsTextInput(event) {
		return
	}
	a.mu.Lock()
	if a.mode != viewLauncher {
		a.mu.Unlock()
		return
	}
	committed := a.editor.HandleTextInput(event)
	if committed {
		a.applyQueryTextChangeLocked(a.editor.State().Text)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
	if committed {
		a.deactivateTerminalPreview()
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
			changed = true
			a.actionPanel = false
			a.actionSelected = 0
			if a.requirementForm != nil {
				a.requirementForm.active = false
			}
			if a.triggerConflict != nil {
				a.triggerConflict.active = false
			}
			if a.themeEditor != nil {
				a.themeEditor.active = false
			}
			if a.chatPreview != nil {
				a.chatPreview.active = false
			}
			a.chatFullscreen = false
			break
		}
	}
	a.mu.Unlock()
	if changed {
		a.restoreQueryTextInput()
	}
	_ = a.window.Invalidate()
}

func (a *App) selectResult(index int) {
	a.mu.Lock()
	closedPanel := false
	if index >= 0 && index < len(a.results) && !a.results[index].IsGroup {
		changed := a.selected != index
		a.selected = index
		if changed {
			closedPanel = a.actionPanel
			a.actionPanel = false
			a.actionSelected = 0
			if a.requirementForm != nil {
				a.requirementForm.active = false
			}
			if a.triggerConflict != nil {
				a.triggerConflict.active = false
			}
			if a.themeEditor != nil {
				a.themeEditor.active = false
			}
			if a.chatPreview != nil {
				a.chatPreview.active = false
			}
			a.chatFullscreen = false
		}
	}
	a.mu.Unlock()
	if index >= 0 {
		a.restoreQueryTextInput()
	}
	if closedPanel {
		_ = a.applyWindowBounds()
	}
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
	if index < 0 || index >= len(a.results) || a.results[index].IsGroup {
		a.mu.RUnlock()
		return
	}
	actionIndex := defaultActionIndex(a.results[index].Actions)
	a.mu.RUnlock()
	a.activateAction(index, actionIndex)
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

func defaultAction(actions []resultAction) (resultAction, bool) {
	index := defaultActionIndex(actions)
	if index >= 0 {
		return actions[index], true
	}
	return resultAction{}, false
}

func defaultActionIndex(actions []resultAction) int {
	for index, action := range actions {
		if action.IsDefault {
			return index
		}
	}
	if len(actions) > 0 {
		return 0
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
	QueryID             string            `json:"QueryId"`
	Results             []queryResult     `json:"Results"`
	Refinements         []queryRefinement `json:"Refinements"`
	Layout              queryLayout       `json:"Layout"`
	Context             queryContext      `json:"Context"`
	IsFinal             bool              `json:"IsFinal"`
	QueryStartTimestamp int64             `json:"QueryStartTimestamp"`
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
	Type  string              `json:"Type"`
	Value formDefinitionValue `json:"Value"`
}

type formDefinitionValue struct {
	Key           string            `json:"Key"`
	Label         string            `json:"Label"`
	Title         string            `json:"Title"`
	Suffix        string            `json:"Suffix"`
	DefaultValue  string            `json:"DefaultValue"`
	Tooltip       string            `json:"Tooltip"`
	Content       string            `json:"Content"`
	MaxLines      int               `json:"MaxLines"`
	IsMulti       bool              `json:"IsMulti"`
	Options       []formOption      `json:"Options"`
	Validators    []formValidator   `json:"Validators"`
	Columns       []formTableColumn `json:"Columns"`
	SortColumnKey string            `json:"SortColumnKey"`
	SortOrder     string            `json:"SortOrder"`
	MaxHeight     int               `json:"MaxHeight"`
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

type updatableResult struct {
	ID       string          `json:"Id"`
	Title    *string         `json:"Title"`
	SubTitle *string         `json:"SubTitle"`
	Icon     *woxImage       `json:"Icon"`
	Preview  *queryPreview   `json:"Preview"`
	Tails    *[]resultTail   `json:"Tails"`
	Actions  *[]resultAction `json:"Actions"`
}
