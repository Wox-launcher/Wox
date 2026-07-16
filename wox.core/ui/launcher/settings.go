package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	woxui "wox/ui/runtime"
)

const (
	settingsWindowWidth  = 1200
	settingsWindowHeight = 800
)

type settingWindowContext struct {
	Path   string `json:"Path"`
	Param  string `json:"Param"`
	Source string `json:"Source"`
}

type settingsData struct {
	EnableAutostart                    bool
	LogLevel                           string
	MainHotkey                         string
	SelectionHotkey                    string
	IgnoredHotkeyApps                  json.RawMessage
	QueryHotkeys                       []queryHotkeySetting
	QueryShortcuts                     []queryShortcutSetting
	TrayQueries                        json.RawMessage
	IsLinuxWaylandSession              bool
	UsePinYin                          bool
	SwitchInputMethodABC               bool
	HideOnStart                        bool
	HideOnLostFocus                    bool
	ShowTray                           bool
	LangCode                           string
	LaunchMode                         string
	StartPage                          string
	HttpProxyEnabled                   bool
	HttpProxyURL                       string `json:"HttpProxyUrl"`
	ShowPosition                       string
	EnableAutoBackup                   bool
	EnableAutoUpdate                   bool
	ReleaseChannel                     string
	EnableAnonymousUsageStats          bool
	CustomPythonPath                   string
	CustomNodejsPath                   string
	CloudSyncServerURL                 string `json:"CloudSyncServerUrl"`
	AppWidth                           int
	MaxResultCount                     int
	UIDensity                          string `json:"UiDensity"`
	ThemeID                            string `json:"ThemeId"`
	AppFontFamily                      string
	EnableQueryCompletionHint          bool
	EnableGlance                       bool
	PrimaryGlance                      glanceRef
	HideGlanceIcon                     bool
	AIProviders                        json.RawMessage
	AIMCPServers                       json.RawMessage
	AISkills                           json.RawMessage
	CloudSyncDisabledPlugins           []string
	ShowScoreTail                      bool
	ShowPerformanceTail                bool
	ShowPerformanceTailBatch           bool
	ShowPerformanceTailPluginQuery     bool
	ShowPerformanceTailBackendPrepared bool
	ShowPerformanceTailUIReceived      bool `json:"ShowPerformanceTailUiReceived"`
}

type queryHotkeySetting struct {
	Name              string
	Hotkey            string
	Query             string
	IsSilentExecution bool
	HideQueryBox      bool
	HideToolbar       bool
	Width             int
	MaxResultCount    int
	Position          string
	Disabled          bool
}

type queryShortcutSetting struct {
	Shortcut string
	Query    string
	Disabled bool
}

type settingChoice struct {
	value string
	label string
}

type settingItem struct {
	key         string
	title       string
	description string
	value       string
	choices     []settingChoice
	text        bool
	browseFile  bool
	disabled    bool
}

type settingsSnapshot struct {
	isDev                bool
	tab                  string
	row                  int
	note                 string
	saving               bool
	editKey              string
	editing              woxui.TextEditingState
	choicePicker         *settingChoicePickerSnapshot
	pageScroll           float32
	railScroll           float32
	data                 settingsData
	palette              uiPalette
	plugins              []pluginSettingsPlugin
	pluginsLoading       bool
	pluginsError         string
	pluginSelected       int
	pluginListScroll     float32
	pluginForm           *pluginSettingsFormSnapshot
	pluginsStore         bool
	pluginOperation      string
	pluginOperationError string
	pluginUninstallArmed string
	hotkeyForm           *formFieldsSnapshot
	glanceCatalog        []glanceCatalogItem
	glanceCatalogLoading bool
	glanceCatalogError   string
	systemFontFamilies   []string
	systemFontsLoading   bool
	systemFontsError     string
	themes               []themeSettingsTheme
	themesMode           string
	themesLoading        bool
	themesError          string
	themeSelected        int
	themeListScroll      float32
	themeOperation       string
	themeUninstallArmed  string
	aiForm               *formFieldsSnapshot
	aiProvidersLoading   bool
	aiProvidersError     string
	tableEditor          *formTableEditorSnapshot
	modelManager         *modelManagerSnapshot
	usage                usageStatsData
	usagePeriod          string
	usageLoading         bool
	usageError           string
	aboutVersion         string
	aboutLoading         bool
	aboutError           string
	privacySample        string
	privacyError         string
	dataBackups          []backupInfo
	dataLocation         string
	dataLoading          bool
	dataBusy             string
	dataError            string
	dataRestoreArmed     string
	dataPendingLocation  string
	dataClearLogsArmed   bool
	dataListScroll       float32
	runtimeStatuses      []runtimeStatus
	runtimeLoading       bool
	runtimeError         string
	runtimeRestarting    string
	runtimePageScroll    float32
	cloudAccount         cloudAccountStatus
	cloudSync            cloudSyncStatus
	cloudDevices         cloudDeviceList
	cloudLoading         bool
	cloudBusy            string
	cloudError           string
	cloudPageScroll      float32
	cloudForm            *cloudFormSnapshot
	cloudPlugins         []pluginSettingsPlugin
	cloudPluginScroll    float32
}

type settingTab struct {
	id    string
	label string
}

var baseSettingTabs = []settingTab{
	{id: "general", label: "General"},
	{id: "hotkeys", label: "Hotkeys"},
	{id: "appearance", label: "Appearance"},
	{id: "network", label: "Network"},
	{id: "data", label: "Data & backup"},
	{id: "cloud", label: "Cloud Sync"},
	{id: "runtime", label: "Runtime"},
	{id: "theme", label: "Themes"},
	{id: "plugins", label: "Plugins"},
	{id: "ai", label: "AI"},
	{id: "usage", label: "Usage"},
	{id: "updates", label: "Updates"},
	{id: "privacy", label: "Privacy"},
	{id: "about", label: "About"},
}

func settingTabs(isDev bool) []settingTab {
	tabs := append([]settingTab(nil), baseSettingTabs...)
	if !isDev {
		return tabs
	}
	for index, tab := range tabs {
		if tab.id == "updates" {
			return append(tabs[:index], append([]settingTab{{id: "debug", label: "Debug"}}, tabs[index:]...)...)
		}
	}
	return append(tabs, settingTab{id: "debug", label: "Debug"})
}

var boolChoices = []settingChoice{{value: "false", label: "Off"}, {value: "true", label: "On"}}

// openSettings switches the shared launcher window into the platform-neutral management layout.
func (a *App) openSettings(windowContext settingWindowContext) error {
	if err := a.reloadSettings(); err != nil {
		return err
	}
	tab, note := settingTabForPath(windowContext.Path)
	if tab == "debug" && !a.isDev {
		tab = "general"
		note = "Debug settings are only available in development builds."
	}
	themeMode := ""
	if tab == "theme" {
		themeMode = themeSettingsModeForPath(windowContext.Path)
	}
	if tab == "plugins" {
		store := pluginSettingsPathIsStore(windowContext.Path)
		a.mu.Lock()
		a.pluginsStore = store
		a.mu.Unlock()
		if err := a.reloadPlugins(store, windowContext.Param); err != nil {
			note = "Could not load plugins: " + err.Error()
		}
	}
	a.mu.Lock()
	a.mode = viewSettings
	a.settingsCtx = windowContext
	a.settingTab = tab
	a.settingRow = 0
	a.settingNote = note
	a.settingSaving = false
	a.settingEditKey = ""
	a.settingEditor = nil
	a.settingChoicePicker = nil
	a.modelManager = nil
	a.runtimePageScroll = 0
	a.cloudPageScroll = 0
	a.cloudForm = nil
	a.form = nil
	a.requirementForm = nil
	a.triggerConflict = nil
	a.themeEditor = nil
	if a.hotkeySettingsForm != nil {
		a.hotkeySettingsForm.active = tab == "hotkeys"
	}
	if tab == "theme" {
		a.themesMode = themeMode
		a.themes = nil
		a.themesLoaded = false
		a.themesLoading = false
		a.themesError = ""
		a.themeSelected = -1
		a.themeListScroll = 0
		a.themeOperation = ""
		a.themeUninstallArmed = ""
	}
	if a.pluginForm != nil {
		a.pluginForm.active = false
	}
	a.visible = true
	a.mu.Unlock()
	a.deactivateTerminalPreview()
	a.resetChatPreview()
	if tab == "theme" && themeMode == "editor" {
		if err := a.loadSettingsThemeEditor(); err != nil {
			a.mu.Lock()
			a.settingNote = "Could not load theme editor: " + err.Error()
			a.mu.Unlock()
		}
	}
	if tab == "theme" && themeMode != "editor" {
		if err := a.reloadThemes(themeMode, ""); err != nil {
			a.mu.Lock()
			a.settingNote = "Could not load themes: " + err.Error()
			a.mu.Unlock()
		}
	}
	if tab == "usage" {
		go a.reloadUsageStats(a.currentUsagePeriod())
	}
	if tab == "ai" {
		go a.loadAIProviderCatalog()
	}
	if tab == "hotkeys" {
		go a.loadHotkeyAppCandidates()
	}
	if tab == "appearance" {
		go a.loadGlanceCatalog()
		go a.loadSystemFontFamilies()
	}
	if tab == "data" {
		go a.reloadDataSettings()
	}
	if tab == "cloud" {
		go a.reloadCloudSync()
	}
	if tab == "runtime" {
		go a.reloadRuntimeStatuses()
	}
	if tab == "about" {
		go a.reloadAboutVersion()
	}
	if tab == "privacy" {
		go a.reloadAboutVersion()
	}

	if err := a.window.SetHideOnBlur(false); err != nil {
		return err
	}
	if err := a.window.SetTextInputState(woxui.TextInputState{}); err != nil {
		return err
	}
	if err := a.window.Center(woxui.Size{Width: settingsWindowWidth, Height: settingsWindowHeight}); err != nil {
		return err
	}
	if err := a.notifySettingViewChanged(true); err != nil {
		return err
	}
	if _, err := a.window.Show(); err != nil {
		_ = a.notifySettingViewChanged(false)
		return err
	}
	return a.window.Invalidate()
}

// reloadSettings refreshes the shared DTO without coupling the widget layer to Wox core packages.
func (a *App) reloadSettings() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var data settingsData
	if err := a.client.Post(ctx, "/setting/wox", map[string]any{}, &data); err != nil {
		return fmt.Errorf("load Wox settings: %w", err)
	}
	if data.LaunchMode == "" {
		data.LaunchMode = "continue"
	}
	if data.StartPage == "" {
		data.StartPage = "mru"
	}
	if data.ShowPosition == "" {
		data.ShowPosition = "mouse_screen"
	}
	if data.UIDensity == "" {
		data.UIDensity = "normal"
	}
	if data.ReleaseChannel == "" {
		data.ReleaseChannel = "stable"
	}
	if data.LogLevel == "" {
		data.LogLevel = "INFO"
	}
	aiForm := newAISettingsForm(data)
	hotkeyForm := newHotkeySettingsForm(data)
	a.mu.Lock()
	applyAIProviderCatalogLocked(&aiForm, a.aiProviderCatalog)
	aiForm.active = a.mode == viewSettings && a.settingTab == "ai"
	hotkeyForm.active = a.mode == viewSettings && a.settingTab == "hotkeys"
	a.settings = data
	a.aiSettingsForm = &aiForm
	a.hotkeySettingsForm = &hotkeyForm
	a.mu.Unlock()
	if a.window != nil {
		if err := a.window.SetFontFamily(data.AppFontFamily); err != nil {
			return fmt.Errorf("apply Wox UI font: %w", err)
		}
		_ = a.window.Invalidate()
	}
	return nil
}

func (a *App) closeSettings() error {
	a.stopHotkeyRecording()
	a.mu.Lock()
	windowContext := a.settingsCtx
	hideOnBlur := a.settings.HideOnLostFocus
	a.mode = viewLauncher
	a.show.HideOnBlur = hideOnBlur
	a.settingSaving = false
	a.settingEditKey = ""
	a.settingEditor = nil
	a.settingChoicePicker = nil
	a.cloudForm = nil
	if a.pluginForm != nil {
		syncFormFieldsEditorLocked(&a.pluginForm.formFieldsState)
		a.pluginForm.active = false
	}
	if a.themeEditor != nil {
		a.themeEditor.active = false
	}
	a.mu.Unlock()
	if err := a.notifySettingViewChanged(false); err != nil {
		return err
	}
	if windowContext.Source == "tray" {
		return a.hideWindow(true)
	}
	if err := a.window.SetHideOnBlur(hideOnBlur); err != nil {
		return err
	}
	if err := a.window.SetTextInputState(woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: 130, Y: 29, Width: 1, Height: 24}}); err != nil {
		return err
	}
	if err := a.applyWindowBoundsAtShowPosition(); err != nil {
		return err
	}
	if _, err := a.window.Show(); err != nil {
		return err
	}
	if err := a.window.Invalidate(); err != nil {
		return err
	}
	go a.refreshGlance("settingsChanged", "", nil)
	return nil
}

func (a *App) onSettingsKey(event woxui.KeyEvent) bool {
	if a.onModelManagerKey(event) {
		return true
	}
	if a.onCloudSettingsKey(event) {
		return true
	}
	if a.onSettingChoicePickerKey(event) {
		return true
	}
	if a.onPluginSettingsKey(event) {
		return true
	}
	if a.onHotkeySettingsKey(event) {
		return true
	}
	if a.onThemeSettingsKey(event) {
		return true
	}
	a.mu.RLock()
	themeTab := a.settingTab == "theme"
	a.mu.RUnlock()
	if themeTab && a.onThemeEditorPreviewKey(event) {
		return true
	}
	if a.onAISettingsKey(event) {
		return true
	}
	if a.onBuiltInSettingsEditorKey(event) {
		return true
	}
	switch event.Key {
	case woxui.KeyEscape:
		go func() {
			if err := a.closeSettings(); err != nil {
				log.Printf("close settings: %v", err)
			}
		}()
	case woxui.KeyTab:
		direction := 1
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			direction = -1
		}
		a.moveSettingTab(direction)
	case woxui.KeyArrowUp:
		a.moveSettingRow(-1)
	case woxui.KeyArrowDown:
		a.moveSettingRow(1)
	case woxui.KeyArrowLeft:
		a.activateSetting(-1)
	case woxui.KeyArrowRight:
		a.activateSetting(1)
	case woxui.KeyEnter, woxui.KeySpace:
		a.openOrActivateSetting()
	default:
		return false
	}
	return true
}

func (a *App) settingsSnapshot() settingsSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var tableEditor *formTableEditorSnapshot
	if a.tableEditor != nil && a.formTableTargetCurrentLocked(a.tableEditor.target) {
		tableEditor = snapshotFormTableEditorLocked(a.tableEditor)
	}
	var editing woxui.TextEditingState
	if a.settingEditor != nil {
		editing = a.settingEditor.State()
	}
	var aiForm *formFieldsSnapshot
	if a.aiSettingsForm != nil {
		snapshot := snapshotFormFieldsLocked(a.aiSettingsForm)
		aiForm = &snapshot
	}
	var hotkeyForm *formFieldsSnapshot
	if a.hotkeySettingsForm != nil {
		snapshot := snapshotFormFieldsLocked(a.hotkeySettingsForm)
		hotkeyForm = &snapshot
	}
	choicePicker := snapshotSettingChoicePickerLocked(a.settingChoicePicker)
	cloudForm := snapshotCloudFormLocked(a.cloudForm)
	modelManager := snapshotModelManagerLocked(a.modelManager)
	return settingsSnapshot{
		isDev:                a.isDev,
		tab:                  a.settingTab,
		row:                  a.settingRow,
		note:                 a.settingNote,
		saving:               a.settingSaving,
		editKey:              a.settingEditKey,
		editing:              editing,
		choicePicker:         choicePicker,
		pageScroll:           a.settingPageScroll,
		railScroll:           a.settingRailScroll,
		data:                 a.settings,
		palette:              a.palette,
		plugins:              append([]pluginSettingsPlugin(nil), a.plugins...),
		pluginsLoading:       a.pluginsLoading,
		pluginsError:         a.pluginsError,
		pluginSelected:       a.pluginSelected,
		pluginListScroll:     a.pluginListScroll,
		pluginForm:           snapshotPluginSettingsFormLocked(a.pluginForm),
		pluginsStore:         a.pluginsStore,
		pluginOperation:      a.pluginOperation,
		pluginOperationError: a.pluginOperationError,
		pluginUninstallArmed: a.pluginUninstallArmed,
		hotkeyForm:           hotkeyForm,
		glanceCatalog:        append([]glanceCatalogItem(nil), a.glanceCatalog...),
		glanceCatalogLoading: a.glanceCatalogLoading,
		glanceCatalogError:   a.glanceCatalogError,
		systemFontFamilies:   append([]string(nil), a.systemFontFamilies...),
		systemFontsLoading:   a.systemFontsLoading,
		systemFontsError:     a.systemFontsError,
		themes:               append([]themeSettingsTheme(nil), a.themes...),
		themesMode:           a.themesMode,
		themesLoading:        a.themesLoading,
		themesError:          a.themesError,
		themeSelected:        a.themeSelected,
		themeListScroll:      a.themeListScroll,
		themeOperation:       a.themeOperation,
		themeUninstallArmed:  a.themeUninstallArmed,
		aiForm:               aiForm,
		aiProvidersLoading:   a.aiProvidersLoading,
		aiProvidersError:     a.aiProvidersError,
		tableEditor:          tableEditor,
		modelManager:         modelManager,
		usage:                cloneUsageStats(a.usageStats),
		usagePeriod:          a.usagePeriod,
		usageLoading:         a.usageLoading,
		usageError:           a.usageError,
		aboutVersion:         a.aboutVersion,
		aboutLoading:         a.aboutLoading,
		aboutError:           a.aboutError,
		privacySample:        a.privacySample,
		privacyError:         a.privacyError,
		dataBackups:          append([]backupInfo(nil), a.dataBackups...),
		dataLocation:         a.dataLocation,
		dataLoading:          a.dataLoading,
		dataBusy:             a.dataBusy,
		dataError:            a.dataError,
		dataRestoreArmed:     a.dataRestoreArmed,
		dataPendingLocation:  a.dataPendingLocation,
		dataClearLogsArmed:   a.dataClearLogsArmed,
		dataListScroll:       a.dataListScroll,
		runtimeStatuses:      cloneRuntimeStatuses(a.runtimeStatuses),
		runtimeLoading:       a.runtimeLoading,
		runtimeError:         a.runtimeError,
		runtimeRestarting:    a.runtimeRestarting,
		runtimePageScroll:    a.runtimePageScroll,
		cloudAccount:         a.cloudAccount,
		cloudSync:            a.cloudSync,
		cloudDevices:         cloneCloudDeviceList(a.cloudDevices),
		cloudLoading:         a.cloudLoading,
		cloudBusy:            a.cloudBusy,
		cloudError:           a.cloudError,
		cloudPageScroll:      a.cloudPageScroll,
		cloudForm:            cloudForm,
		cloudPlugins:         append([]pluginSettingsPlugin(nil), a.cloudPlugins...),
		cloudPluginScroll:    a.cloudPluginScroll,
	}
}

func (a *App) selectSettingTab(tab string) {
	if tab == "debug" && !a.isDev {
		return
	}
	a.stopHotkeyRecording()
	loadPlugins := false
	loadTheme := false
	loadThemes := false
	loadUsage := false
	loadAbout := false
	loadAIProviders := false
	loadHotkeyApps := false
	loadGlanceCatalog := false
	loadSystemFonts := false
	loadData := false
	loadRuntime := false
	loadCloud := false
	a.mu.Lock()
	a.settingChoicePicker = nil
	if a.settingTab != tab {
		if a.pluginForm != nil {
			syncFormFieldsEditorLocked(&a.pluginForm.formFieldsState)
			a.pluginForm.active = false
		}
		a.settingTab = tab
		a.settingRow = 0
		a.settingNote = ""
		a.settingEditKey = ""
		a.settingEditor = nil
		a.settingPageScroll = 0
		a.runtimePageScroll = 0
		a.cloudPageScroll = 0
		a.cloudPluginScroll = 0
		a.cloudForm = nil
		if tab != "plugins" {
			a.modelManager = nil
		}
		if a.themeEditor != nil {
			a.themeEditor.active = false
		}
	}
	a.ensureSettingTabVisibleLocked(tab)
	if a.aiSettingsForm != nil {
		a.aiSettingsForm.active = tab == "ai"
		if tab == "ai" {
			setFormFieldsFocusLocked(a.aiSettingsForm, 0)
		}
	}
	if a.hotkeySettingsForm != nil {
		a.hotkeySettingsForm.active = tab == "hotkeys"
		if tab == "hotkeys" {
			setFormFieldsFocusLocked(a.hotkeySettingsForm, max(0, a.hotkeySettingsForm.focused))
		}
	}
	if tab == "theme" && a.themesMode == "" {
		a.themesMode = "installed"
	}
	loadPlugins = tab == "plugins" && !a.pluginsLoaded && !a.pluginsLoading
	loadTheme = tab == "theme" && a.themesMode == "editor" && (a.themeEditor == nil || !strings.HasPrefix(a.themeEditor.key, "settings-theme|"))
	loadThemes = tab == "theme" && a.themesMode != "editor" && !a.themesLoaded && !a.themesLoading
	loadUsage = tab == "usage" && !a.usageLoaded && !a.usageLoading
	loadAbout = (tab == "about" || tab == "privacy") && !a.aboutLoaded && !a.aboutLoading
	loadAIProviders = tab == "ai" && !a.aiProvidersLoaded && !a.aiProvidersLoading
	loadHotkeyApps = tab == "hotkeys" && !a.hotkeyAppsLoaded && !a.hotkeyAppsLoading
	loadGlanceCatalog = tab == "appearance" && !a.glanceCatalogLoaded && !a.glanceCatalogLoading
	loadSystemFonts = tab == "appearance" && !a.systemFontsLoaded && !a.systemFontsLoading
	loadData = tab == "data" && !a.dataLoaded && !a.dataLoading
	loadRuntime = tab == "runtime" && !a.runtimeLoaded && !a.runtimeLoading
	loadCloud = tab == "cloud" && !a.cloudLoaded && !a.cloudLoading
	a.mu.Unlock()
	_ = a.window.SetTextInputState(woxui.TextInputState{})
	if loadPlugins {
		go func() {
			a.mu.RLock()
			store := a.pluginsStore
			a.mu.RUnlock()
			if err := a.reloadPlugins(store, ""); err != nil {
				log.Printf("load plugins: %v", err)
			}
		}()
	}
	if loadTheme {
		go func() {
			if err := a.loadSettingsThemeEditor(); err != nil {
				a.mu.Lock()
				a.settingNote = "Could not load theme editor: " + err.Error()
				a.mu.Unlock()
				_ = a.window.Invalidate()
			}
		}()
	}
	if loadThemes {
		a.mu.RLock()
		mode := a.themesMode
		a.mu.RUnlock()
		go func() {
			if err := a.reloadThemes(mode, ""); err != nil {
				log.Printf("load themes: %v", err)
			}
		}()
	}
	if loadUsage {
		go a.reloadUsageStats(a.currentUsagePeriod())
	}
	if loadAbout {
		go a.reloadAboutVersion()
	}
	if loadAIProviders {
		go a.loadAIProviderCatalog()
	}
	if loadHotkeyApps {
		go a.loadHotkeyAppCandidates()
	}
	if loadGlanceCatalog {
		go a.loadGlanceCatalog()
	}
	if loadSystemFonts {
		go a.loadSystemFontFamilies()
	}
	if loadData {
		go a.reloadDataSettings()
	}
	if loadRuntime {
		go a.reloadRuntimeStatuses()
	}
	if loadCloud {
		go a.reloadCloudSync()
	}
	_ = a.window.Invalidate()
}

func (a *App) moveSettingTab(delta int) {
	a.mu.RLock()
	current := a.settingTab
	a.mu.RUnlock()
	index := 0
	tabs := settingTabs(a.isDev)
	for candidate, tab := range tabs {
		if tab.id == current {
			index = candidate
			break
		}
	}
	index = (index + delta + len(tabs)) % len(tabs)
	a.selectSettingTab(tabs[index].id)
}

func (a *App) setSettingsRailViewport(height float32) {
	a.mu.Lock()
	a.settingRailViewport = max(float32(1), height)
	a.ensureSettingTabVisibleLocked(a.settingTab)
	a.mu.Unlock()
}

func (a *App) scrollSettingsRail(delta float32) {
	a.mu.Lock()
	contentHeight := settingsRailContentHeight(len(settingTabs(a.isDev)))
	maximum := max(float32(0), contentHeight-a.settingRailViewport)
	a.settingRailScroll = min(max(float32(0), a.settingRailScroll+delta), maximum)
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) ensureSettingTabVisibleLocked(tabID string) {
	tabs := settingTabs(a.isDev)
	index := -1
	for candidate, tab := range tabs {
		if tab.id == tabID {
			index = candidate
			break
		}
	}
	if index < 0 {
		return
	}
	viewport := max(float32(1), a.settingRailViewport)
	top := float32(66 + index*56)
	bottom := top + 48
	if top < a.settingRailScroll {
		a.settingRailScroll = top
	} else if bottom > a.settingRailScroll+viewport {
		a.settingRailScroll = bottom - viewport
	}
	maximum := max(float32(0), settingsRailContentHeight(len(tabs))-viewport)
	a.settingRailScroll = min(max(float32(0), a.settingRailScroll), maximum)
}

func settingsRailContentHeight(tabCount int) float32 {
	return 58 + float32(tabCount*56)
}

func (a *App) moveSettingRow(delta int) {
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	if len(items) == 0 {
		return
	}
	a.mu.Lock()
	a.settingRow = (a.settingRow + delta + len(items)) % len(items)
	a.ensureSettingRowVisibleLocked(len(items))
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) selectSettingRow(index int) {
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	if index < 0 || index >= len(items) {
		return
	}
	a.mu.Lock()
	if a.settingEditKey != "" {
		if a.settingRow != index {
			a.mu.Unlock()
			return
		}
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	a.settingRow = index
	a.ensureSettingRowVisibleLocked(len(items))
	a.mu.Unlock()
	_ = a.window.SetTextInputState(woxui.TextInputState{})
	_ = a.window.Invalidate()
}

func (a *App) setSettingsPageViewport(height float32, itemCount int) {
	a.mu.Lock()
	a.settingPageViewport = max(float32(1), height)
	a.ensureSettingRowVisibleLocked(itemCount)
	a.mu.Unlock()
}

func (a *App) scrollSettingsPage(delta float32) {
	snapshot := a.settingsSnapshot()
	contentHeight := settingsPageContentHeight(len(settingItemsForSnapshot(snapshot)))
	a.mu.Lock()
	maximum := max(float32(0), contentHeight-a.settingPageViewport)
	a.settingPageScroll = min(max(float32(0), a.settingPageScroll+delta), maximum)
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) ensureSettingRowVisibleLocked(itemCount int) {
	if a.settingRow < 0 || a.settingRow >= itemCount {
		return
	}
	if a.settingTab == "runtime" {
		a.ensureRuntimeSettingRowVisibleLocked()
		return
	}
	viewport := max(float32(1), a.settingPageViewport)
	top := float32(74 + a.settingRow*79)
	bottom := top + 70
	if top < a.settingPageScroll {
		a.settingPageScroll = top
	} else if bottom > a.settingPageScroll+viewport {
		a.settingPageScroll = bottom - viewport
	}
	maximum := max(float32(0), settingsPageContentHeight(itemCount)-viewport)
	a.settingPageScroll = min(max(float32(0), a.settingPageScroll), maximum)
}

func (a *App) activateSetting(direction int) {
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	if snapshot.saving || snapshot.row < 0 || snapshot.row >= len(items) {
		return
	}
	item := items[snapshot.row]
	if item.disabled {
		return
	}
	if item.key == "PrivacySample" {
		a.togglePrivacySample()
		return
	}
	if item.key == "UsagePeriod" {
		next, ok := nextSettingChoice(item, direction)
		if ok {
			go a.reloadUsageStats(next.value)
		}
		return
	}
	if item.text {
		a.startBuiltInSettingEdit(item, -1)
		return
	}
	next, ok := nextSettingChoice(item, direction)
	if !ok {
		return
	}
	a.mu.Lock()
	a.settingSaving = true
	a.settingNote = "Saving " + item.title + "…"
	a.mu.Unlock()
	_ = a.window.Invalidate()
	go a.saveSetting(item, next)
}

// startBuiltInSettingEdit gives a core-backed text value shared editor and native IME ownership.
func (a *App) startBuiltInSettingEdit(item settingItem, caret int) {
	if !item.text {
		return
	}
	a.mu.Lock()
	if a.settingSaving {
		a.mu.Unlock()
		return
	}
	if a.settingEditKey != item.key || a.settingEditor == nil {
		a.settingEditKey = item.key
		a.settingEditor = woxui.NewTextEditor(item.value)
	}
	if caret >= 0 {
		a.settingEditor.SetCaret(caret)
	}
	a.settingNote = "Editing " + item.title + " · Enter saves · Esc cancels"
	a.mu.Unlock()
	a.updateFormTextInput(true)
	_ = a.window.Invalidate()
}

// cancelBuiltInSettingEdit discards an unsaved text value without mutating the loaded settings DTO.
func (a *App) cancelBuiltInSettingEdit() {
	a.mu.Lock()
	a.settingEditKey = ""
	a.settingEditor = nil
	a.settingNote = ""
	a.mu.Unlock()
	_ = a.window.SetTextInputState(woxui.TextInputState{})
	_ = a.window.Invalidate()
}

// submitBuiltInSettingEdit persists the active text row through the same key-value route as choice settings.
func (a *App) submitBuiltInSettingEdit() {
	snapshot := a.settingsSnapshot()
	if snapshot.editKey == "" || snapshot.saving {
		return
	}
	items := settingItemsForSnapshot(snapshot)
	index := -1
	for candidate, item := range items {
		if item.key == snapshot.editKey && item.text {
			index = candidate
			break
		}
	}
	if index < 0 {
		a.cancelBuiltInSettingEdit()
		return
	}
	item := items[index]
	value := snapshot.editing.Text
	a.mu.Lock()
	a.settingSaving = true
	a.settingNote = "Saving " + item.title + "…"
	a.mu.Unlock()
	_ = a.window.SetTextInputState(woxui.TextInputState{})
	_ = a.window.Invalidate()
	go a.saveSetting(item, settingChoice{value: value, label: value})
}

// onBuiltInSettingsEditorKey keeps text editing separate from rail and choice navigation.
func (a *App) onBuiltInSettingsEditorKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	active := a.mode == viewSettings && a.settingEditKey != "" && a.settingEditor != nil
	saving := a.settingSaving
	a.mu.RUnlock()
	if !active {
		return false
	}
	if saving {
		return true
	}
	if event.Key == woxui.KeyEscape {
		a.cancelBuiltInSettingEdit()
		return true
	}
	if event.Key == woxui.KeyEnter || (event.Modifiers.HasPrimary() && event.Key == woxui.Key("s")) {
		a.submitBuiltInSettingEdit()
		return true
	}
	a.mu.Lock()
	if a.settingEditor != nil {
		a.settingEditor.HandleKey(event)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
	return true
}

// onBuiltInSettingsTextInput commits native text and IME events into the active settings editor.
func (a *App) onBuiltInSettingsTextInput(event woxui.TextInputEvent) bool {
	if a.onSettingChoicePickerTextInput(event) {
		return true
	}
	a.mu.Lock()
	if a.mode != viewSettings || a.settingSaving || a.settingEditKey == "" || a.settingEditor == nil {
		a.mu.Unlock()
		return false
	}
	a.settingEditor.HandleTextInput(event)
	a.mu.Unlock()
	_ = a.window.Invalidate()
	return true
}

// browseBuiltInSettingFile uses the common Window picker and leaves persistence on explicit Enter.
func (a *App) browseBuiltInSettingFile(item settingItem) {
	if !item.text || !item.browseFile {
		return
	}
	path, err := a.window.PickFile(woxui.FileDialogOptions{})
	if err != nil {
		a.mu.Lock()
		a.settingNote = "Could not select " + item.title + ": " + err.Error()
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	if path == "" {
		return
	}
	a.startBuiltInSettingEdit(item, -1)
	a.mu.Lock()
	if a.settingEditKey == item.key && a.settingEditor != nil {
		a.settingEditor.SetText(path, false)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) saveSetting(item settingItem, choice settingChoice) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	err := a.client.Post(ctx, "/setting/wox/update", map[string]string{"Key": item.key, "Value": choice.value}, nil)
	if err == nil && item.key == "CloudSyncServerUrl" {
		err = a.client.Post(ctx, "/account/logout", map[string]any{}, nil)
	}
	cancel()
	if err == nil {
		err = a.reloadSettings()
	}
	restoreTextInput := false
	a.mu.Lock()
	a.settingSaving = false
	if a.settingEditKey == item.key {
		if err == nil {
			a.settingEditKey = ""
			a.settingEditor = nil
		} else if item.text {
			a.settingEditor = woxui.NewTextEditor(choice.value)
			restoreTextInput = true
		}
	}
	if err != nil {
		a.settingNote = "Could not save " + item.title + ": " + err.Error()
	} else {
		a.settingNote = item.title + " · " + choice.label
	}
	a.mu.Unlock()
	refreshGlance := false
	if err == nil && (item.key == "EnableGlance" || item.key == "PrimaryGlance") {
		a.mu.Lock()
		a.stopGlanceLocked(true)
		refreshGlance = a.glanceEligibleLocked()
		a.mu.Unlock()
	}
	if restoreTextInput {
		a.updateFormTextInput(true)
	} else {
		_ = a.window.SetTextInputState(woxui.TextInputState{})
	}
	if refreshGlance {
		go a.refreshGlance("settingsChanged", "", nil)
	}
	if err == nil && (item.key == "CustomPythonPath" || item.key == "CustomNodejsPath") {
		go a.reloadRuntimeStatuses()
	}
	_ = a.window.Invalidate()
}

func settingTabForPath(path string) (string, string) {
	switch strings.TrimSpace(path) {
	case "", "/", "/general":
		return "general", ""
	case "/ui", "/appearance":
		return "appearance", ""
	case "/hotkeys", "hotkeys", "/query/hotkeys":
		return "hotkeys", ""
	case "/network":
		return "network", ""
	case "/data", "/data/backup", "/data.backup", "data", "data.backup":
		return "data", ""
	case "/data/cloudsync", "/cloud", "/cloud-sync", "data.cloudsync":
		return "cloud", ""
	case "/runtime", "/plugins/runtime", "plugins.runtime":
		return "runtime", ""
	case "/themes", "/themes/installed", "themes.installed", "/themes/store", "themes.store", "/themes/edit", "/themes.edit", "themes.edit":
		return "theme", ""
	case "/plugin", "/plugins", "/plugins/installed", "plugins.installed", "/plugin/setting":
		return "plugins", ""
	case "/plugins/store", "plugins.store":
		return "plugins", ""
	case "/ai", "ai":
		return "ai", ""
	case "/debug", "debug":
		return "debug", ""
	case "/update", "/updates":
		return "updates", ""
	case "/privacy", "privacy":
		return "privacy", ""
	case "/usage", "usage":
		return "usage", ""
	case "/about", "about":
		return "about", ""
	default:
		return "general", "This deep-linked settings section is not in the Go UI yet."
	}
}

// settingItemsForSnapshot adds page-local controls without storing them in the core settings DTO.
func settingItemsForSnapshot(snapshot settingsSnapshot) []settingItem {
	if snapshot.tab == "usage" {
		return []settingItem{{
			key: "UsagePeriod", title: "Reporting period", value: snapshot.usagePeriod,
			choices: []settingChoice{{"7d", "7 days"}, {"30d", "30 days"}, {"365d", "365 days"}, {"all", "All time"}},
		}}
	}
	if snapshot.tab == "ai" || snapshot.tab == "hotkeys" || snapshot.tab == "data" || snapshot.tab == "cloud" || snapshot.tab == "plugins" || snapshot.tab == "theme" || snapshot.tab == "about" {
		return nil
	}
	items := settingItems(snapshot.tab, snapshot.data)
	if snapshot.tab == "appearance" {
		items = append(items, systemFontSettingItem(snapshot))
		items = append(items, primaryGlanceSettingItem(snapshot))
	}
	return items
}

func primaryGlanceSettingItem(snapshot settingsSnapshot) settingItem {
	current := snapshot.data.PrimaryGlance
	currentValue := glanceRefJSON(current)
	choices := make([]settingChoice, 0, len(snapshot.glanceCatalog)+1)
	found := false
	for _, glance := range snapshot.glanceCatalog {
		value := glanceRefJSON(glance.Ref)
		label := glance.Name
		if strings.TrimSpace(label) == "" {
			label = glance.Ref.GlanceID
		}
		if strings.TrimSpace(glance.PluginName) != "" {
			label += " · " + glance.PluginName
		}
		choices = append(choices, settingChoice{value: value, label: label})
		if glance.Ref == current {
			found = true
		}
	}
	if !found && current.PluginID != "" && current.GlanceID != "" {
		choices = append([]settingChoice{{value: currentValue, label: current.GlanceID}}, choices...)
	}
	description := "Select the status shown in the global query box"
	if snapshot.glanceCatalogLoading {
		description = "Loading available Glance providers…"
	} else if snapshot.glanceCatalogError != "" {
		description = "Could not load Glance providers: " + snapshot.glanceCatalogError
	}
	return settingItem{key: "PrimaryGlance", title: "Primary glance", description: description, value: currentValue, choices: choices}
}

func glanceRefJSON(ref glanceRef) string {
	encoded, _ := json.Marshal(ref)
	return string(encoded)
}

func settingItems(tab string, data settingsData) []settingItem {
	boolValue := func(value bool) string {
		if value {
			return "true"
		}
		return "false"
	}
	switch tab {
	case "appearance":
		widthChoices := make([]settingChoice, 0, 21)
		for width := 600; width <= 1600; width += 50 {
			widthChoices = append(widthChoices, settingChoice{value: fmt.Sprintf("%d", width), label: fmt.Sprintf("%d", width)})
		}
		resultChoices := make([]settingChoice, 0, 11)
		for count := 5; count <= 15; count++ {
			resultChoices = append(resultChoices, settingChoice{value: fmt.Sprintf("%d", count), label: fmt.Sprintf("%d", count)})
		}
		return []settingItem{
			{key: "AppWidth", title: "Launcher width", description: "Logical width of the query and result window", value: fmt.Sprintf("%d", data.AppWidth), choices: widthChoices},
			{key: "MaxResultCount", title: "Maximum results", description: "Number of result rows visible before scrolling", value: fmt.Sprintf("%d", data.MaxResultCount), choices: resultChoices},
			{key: "UiDensity", title: "UI density", description: "Spacing and row size across the launcher", value: data.UIDensity, choices: []settingChoice{{"compact", "Compact"}, {"normal", "Normal"}, {"comfortable", "Comfortable"}}},
			{key: "LaunchMode", title: "Launch mode", description: "Start fresh or continue the previous query", value: data.LaunchMode, choices: []settingChoice{{"fresh", "Fresh"}, {"continue", "Continue"}}},
			{key: "StartPage", title: "Start page", description: "Content shown for an empty query", value: data.StartPage, choices: []settingChoice{{"blank", "Blank"}, {"mru", "Recent"}}},
			{key: "ShowPosition", title: "Window position", description: "Display used when Wox opens", value: data.ShowPosition, choices: []settingChoice{{"mouse_screen", "Mouse display"}, {"active_screen", "Active display"}, {"last_location", "Last location"}}},
			{key: "EnableGlance", title: "Glance", description: "Show glance content beside the query", value: boolValue(data.EnableGlance), choices: boolChoices},
			{key: "HideGlanceIcon", title: "Hide glance icon", description: "Keep the query box visually minimal", value: boolValue(data.HideGlanceIcon), choices: boolChoices},
		}
	case "network":
		return []settingItem{
			{key: "HttpProxyEnabled", title: "HTTP proxy", description: "Use a proxy for Wox network requests", value: boolValue(data.HttpProxyEnabled), choices: boolChoices},
			{key: "HttpProxyUrl", title: "Proxy URL", description: "HTTP, HTTPS, or SOCKS proxy address", value: data.HttpProxyURL, text: true},
		}
	case "runtime":
		return []settingItem{
			{key: "LogLevel", title: "Log level", description: "Diagnostic detail written by Wox core", value: strings.ToUpper(data.LogLevel), choices: []settingChoice{{"INFO", "Info"}, {"DEBUG", "Debug"}}},
			{key: "CustomPythonPath", title: "Python executable", description: "Optional Python 3.10 or newer executable", value: data.CustomPythonPath, text: true, browseFile: true},
			{key: "CustomNodejsPath", title: "Node.js executable", description: "Optional Node.js 20 or newer executable", value: data.CustomNodejsPath, text: true, browseFile: true},
		}
	case "debug":
		performanceDisabled := !data.ShowPerformanceTail
		return []settingItem{
			{key: "CloudSyncServerUrl", title: "Cloud Sync server", description: "Switching endpoints logs out the current cloud account", value: normalizedCloudSyncServerURL(data.CloudSyncServerURL), choices: []settingChoice{{"https://sync.woxlauncher.com", "Production"}, {"http://127.0.0.1:8787", "Local"}}},
			{key: "ShowScoreTail", title: "Score tails", description: "Show ranking scores on query results", value: boolValue(data.ShowScoreTail), choices: boolChoices},
			{key: "ShowPerformanceTail", title: "Performance tails", description: "Show query timing diagnostics on results", value: boolValue(data.ShowPerformanceTail), choices: boolChoices},
			{key: "ShowPerformanceTailBatch", title: "Batch timing", description: "Show the result batch and queue timing", value: boolValue(data.ShowPerformanceTailBatch), choices: boolChoices, disabled: performanceDisabled},
			{key: "ShowPerformanceTailPluginQuery", title: "Plugin query timing", description: "Show time spent querying each plugin", value: boolValue(data.ShowPerformanceTailPluginQuery), choices: boolChoices, disabled: performanceDisabled},
			{key: "ShowPerformanceTailBackendPrepared", title: "Backend prepared timing", description: "Show time until core prepared the response", value: boolValue(data.ShowPerformanceTailBackendPrepared), choices: boolChoices, disabled: performanceDisabled},
			{key: "ShowPerformanceTailUiReceived", title: "UI received timing", description: "Show time until the Go UI received the result", value: boolValue(data.ShowPerformanceTailUIReceived), choices: boolChoices, disabled: performanceDisabled},
		}
	case "updates":
		return []settingItem{
			{key: "EnableAutoUpdate", title: "Automatic updates", description: "Check for and install Wox updates", value: boolValue(data.EnableAutoUpdate), choices: boolChoices},
			{key: "ReleaseChannel", title: "Release channel", description: "Choose stable or beta releases", value: data.ReleaseChannel, choices: []settingChoice{{"stable", "Stable"}, {"beta", "Beta"}}},
		}
	case "privacy":
		return []settingItem{
			{key: "EnableAnonymousUsageStats", title: "Anonymous usage stats", description: "Help improve Wox with anonymous telemetry", value: boolValue(data.EnableAnonymousUsageStats), choices: boolChoices},
			{key: "PrivacySample", title: "Telemetry sample", description: "Inspect the exact payload shape without sending data", value: "View"},
		}
	default:
		return []settingItem{
			{key: "EnableAutostart", title: "Start at login", description: "Launch Wox when the desktop session starts", value: boolValue(data.EnableAutostart), choices: boolChoices},
			{key: "HideOnStart", title: "Start hidden", description: "Keep Wox hidden after startup", value: boolValue(data.HideOnStart), choices: boolChoices},
			{key: "HideOnLostFocus", title: "Hide on focus loss", description: "Dismiss the launcher when focus moves away", value: boolValue(data.HideOnLostFocus), choices: boolChoices},
			{key: "ShowTray", title: "Tray icon", description: "Show Wox in the system tray or menu bar", value: boolValue(data.ShowTray), choices: boolChoices},
			{key: "UsePinYin", title: "Pinyin search", description: "Match Chinese text with Pinyin", value: boolValue(data.UsePinYin), choices: boolChoices},
			{key: "SwitchInputMethodABC", title: "Switch input method", description: "Use the Latin input source when Wox opens", value: boolValue(data.SwitchInputMethodABC), choices: boolChoices},
			{key: "EnableQueryCompletionHint", title: "Query completion hints", description: "Show completion text while typing", value: boolValue(data.EnableQueryCompletionHint), choices: boolChoices},
		}
	}
}

func normalizedCloudSyncServerURL(value string) string {
	if strings.TrimSpace(value) == "http://127.0.0.1:8787" {
		return "http://127.0.0.1:8787"
	}
	return "https://sync.woxlauncher.com"
}

func nextSettingChoice(item settingItem, direction int) (settingChoice, bool) {
	if len(item.choices) == 0 {
		return settingChoice{}, false
	}
	index := 0
	for candidate, choice := range item.choices {
		if choice.value == item.value {
			index = candidate
			break
		}
	}
	if direction < 0 {
		index = (index - 1 + len(item.choices)) % len(item.choices)
	} else {
		index = (index + 1) % len(item.choices)
	}
	return item.choices[index], true
}

func settingValueLabel(item settingItem) string {
	for _, choice := range item.choices {
		if choice.value == item.value {
			return choice.label
		}
	}
	return item.value
}
