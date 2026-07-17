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
	key          string
	title        string
	description  string
	value        string
	choices      []settingChoice
	trailers     map[string]string
	filterable   bool
	text         bool
	controlWidth float32
	browseFile   bool
	disabled     bool
}

type settingsSnapshot struct {
	isDev                 bool
	titleBarHover         string
	tab                   string
	row                   int
	note                  string
	saving                bool
	editKey               string
	editing               woxui.TextEditingState
	searchQuery           woxui.TextEditingState
	searchFocused         bool
	searchPanel           bool
	searchSelected        int
	searchScroll          float32
	searchPlugins         []pluginSettingsPlugin
	searchLoading         bool
	searchError           string
	choicePicker          *settingChoicePickerSnapshot
	pageScroll            scrollController
	railScroll            scrollController
	languages             []settingChoice
	updateChannelVersions []updateChannelVersion
	data                  settingsData
	palette               uiPalette
	plugins               []pluginSettingsPlugin
	pluginsLoading        bool
	pluginsError          string
	pluginSelected        int
	pluginListScroll      float32
	pluginSearch          woxui.TextEditingState
	pluginSearchFocused   bool
	pluginFilters         pluginFilterState
	pluginFilterOpen      bool
	pluginDetailTab       string
	pluginForm            *pluginSettingsFormSnapshot
	pluginsStore          bool
	pluginOperation       string
	pluginOperationError  string
	pluginUninstallArmed  string
	hotkeyForm            *formFieldsSnapshot
	hotkeyFocused         bool
	glanceCatalog         []glanceCatalogItem
	glanceCatalogLoading  bool
	glanceCatalogError    string
	systemFontFamilies    []string
	systemFontsLoading    bool
	systemFontsError      string
	themes                []themeSettingsTheme
	themesMode            string
	themesLoading         bool
	themesError           string
	themeSelected         int
	themeListScroll       float32
	themeSearch           woxui.TextEditingState
	themeSearchFocused    bool
	themeDetailTab        string
	themeOperation        string
	themeUninstallArmed   string
	aiForm                *formFieldsSnapshot
	aiProvidersLoading    bool
	aiProvidersError      string
	tableEditor           *formTableEditorSnapshot
	modelManager          *modelManagerSnapshot
	usage                 usageStatsData
	usagePeriod           string
	usageLoading          bool
	usageError            string
	aboutVersion          string
	aboutLoading          bool
	aboutError            string
	privacySample         string
	privacyError          string
	dataBackups           []backupInfo
	dataLocation          string
	dataLoading           bool
	dataBusy              string
	dataError             string
	dataRestoreArmed      string
	dataPendingLocation   string
	dataClearLogsArmed    bool
	dataListScroll        float32
	runtimeStatuses       []runtimeStatus
	runtimeLoading        bool
	runtimeError          string
	runtimeRestarting     string
	runtimePageScroll     float32
	cloudAccount          cloudAccountStatus
	cloudSync             cloudSyncStatus
	cloudBillingPlan      cloudBillingPlan
	cloudBillingLoaded    bool
	cloudDevices          cloudDeviceList
	cloudLoading          bool
	cloudBusy             string
	cloudError            string
	cloudPageScroll       float32
	cloudForm             *cloudFormSnapshot
	cloudActionMenu       string
	cloudPlugins          []pluginSettingsPlugin
	cloudPluginScroll     float32
}

type settingTab struct {
	id    string
	label string
}

var baseSettingTabs = []settingTab{
	{id: "general", label: "General"},
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

type settingNavSpec struct {
	id       string
	tab      string
	labelKey string
	fallback string
	icon     string
	mode     string
	depth    int
	parent   bool
}

func settingNavSpecs(isDev bool) []settingNavSpec {
	specs := []settingNavSpec{
		{id: "general", tab: "general", labelKey: "ui_general", fallback: "General", icon: "⚙"},
		{id: "ui", tab: "appearance", labelKey: "ui_ui", fallback: "Interface", icon: "◉"},
		{id: "ai", tab: "ai", labelKey: "ui_ai", fallback: "AI", icon: "◇"},
		{id: "network", tab: "network", labelKey: "ui_network", fallback: "Network", icon: "●"},
		{id: "data", labelKey: "ui_data", fallback: "Data", icon: "□", parent: true},
		{id: "data.backup", tab: "data", labelKey: "ui_data_backup_restore_nav", fallback: "Backup & Logs", icon: "☁", depth: 1},
		{id: "data.cloudsync", tab: "cloud", labelKey: "ui_cloud_sync", fallback: "Cloud Sync", icon: "☁", depth: 1},
		{id: "plugins", labelKey: "ui_plugins", fallback: "Plugins", icon: "♧", parent: true},
		{id: "plugins.store", tab: "plugins", labelKey: "ui_store_plugins", fallback: "Plugin Store", icon: "▢", mode: "store", depth: 1},
		{id: "plugins.installed", tab: "plugins", labelKey: "ui_installed_plugins", fallback: "Installed Plugins", icon: "▦", mode: "installed", depth: 1},
		{id: "plugins.runtime", tab: "runtime", labelKey: "ui_runtime_settings", fallback: "Runtime Settings", icon: "▣", depth: 1},
		{id: "themes", labelKey: "ui_themes", fallback: "Themes", icon: "◉", parent: true},
		{id: "themes.store", tab: "theme", labelKey: "ui_store_themes", fallback: "Theme Store", icon: "▢", mode: "store", depth: 1},
		{id: "themes.installed", tab: "theme", labelKey: "ui_installed_themes", fallback: "Installed Themes", icon: "⌁", mode: "installed", depth: 1},
		{id: "themes.edit", tab: "theme", labelKey: "ui_theme_editor_title", fallback: "Theme Editor", icon: "⚑", mode: "editor", depth: 1},
		{id: "usage", tab: "usage", labelKey: "ui_usage", fallback: "Usage", icon: "⌁"},
	}
	if isDev {
		specs = append(specs, settingNavSpec{id: "debug", tab: "debug", labelKey: "ui_debug", fallback: "Debug", icon: "!"})
	}
	return append(specs,
		settingNavSpec{id: "update", tab: "updates", labelKey: "ui_update", fallback: "Updates", icon: "↻"},
		settingNavSpec{id: "privacy", tab: "privacy", labelKey: "ui_privacy", fallback: "Privacy", icon: "◇"},
		settingNavSpec{id: "about", tab: "about", labelKey: "ui_about", fallback: "About", icon: "ⓘ"},
	)
}

func activeSettingNavID(tab string, pluginsStore bool, themesMode string) string {
	switch tab {
	case "appearance":
		return "ui"
	case "data":
		return "data.backup"
	case "cloud":
		return "data.cloudsync"
	case "plugins":
		if pluginsStore {
			return "plugins.store"
		}
		return "plugins.installed"
	case "runtime":
		return "plugins.runtime"
	case "theme":
		switch themesMode {
		case "store":
			return "themes.store"
		case "editor":
			return "themes.edit"
		default:
			return "themes.installed"
		}
	case "updates":
		return "update"
	default:
		return tab
	}
}

// openSettings creates or focuses the independent settings window at one platform-neutral route.
func (a *App) openSettings(windowContext settingWindowContext) error {
	if err := a.reloadSettings(); err != nil {
		return err
	}
	if err := a.hideWindow(true); err != nil {
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
	a.settingsOpen = true
	a.settingsCtx = windowContext
	a.settingTab = tab
	a.settingRow = 0
	a.settingNote = note
	a.settingSaving = false
	a.settingEditKey = ""
	a.settingEditor = nil
	a.settingSearchEditor = woxui.NewTextEditor("")
	a.settingSearchFocused = tab != "plugins"
	a.settingSearchPanel = false
	a.settingSearchSelected = 0
	a.settingSearchScroll = 0
	if tab == "plugins" {
		if a.pluginSearchEditor == nil {
			a.pluginSearchEditor = woxui.NewTextEditor("")
		}
		a.pluginSearchFocused = true
	} else {
		a.pluginSearchFocused = false
	}
	a.settingPageScroll.reset()
	a.settingsHotkeyFocus = false
	a.settingChoicePicker = nil
	a.modelManager = nil
	a.runtimePageScroll = 0
	a.cloudPageScroll = 0
	a.cloudForm = nil
	a.cloudActionMenu = ""
	a.form = nil
	a.requirementForm = nil
	a.triggerConflict = nil
	a.themeEditor = nil
	if a.hotkeySettingsForm != nil {
		a.hotkeySettingsForm.active = tab == "general"
	}
	if a.aiSettingsForm != nil {
		a.aiSettingsForm.active = tab == "ai"
	}
	if tab == "theme" {
		a.themesMode = themeMode
		a.themes = nil
		a.themesLoaded = false
		a.themesLoading = false
		a.themesError = ""
		a.themeSelected = -1
		a.themeListScroll = 0
		a.themeSearchEditor = woxui.NewTextEditor("")
		a.themeSearchFocused = false
		a.themeDetailTab = "preview"
		a.themeOperation = ""
		a.themeUninstallArmed = ""
	}
	if a.pluginForm != nil {
		a.pluginForm.active = false
	}
	a.ensureSettingTabVisibleLocked(tab)
	a.mu.Unlock()
	a.preloadThemeEditorWallpaper()
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
	if tab == "general" {
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
	go a.reloadUpdateChannelVersions()

	settingsView, err := a.ensureSettingsWindow()
	if err != nil {
		a.mu.Lock()
		a.settingsOpen = false
		a.releaseThemeEditorWallpaperLocked()
		a.mu.Unlock()
		return err
	}
	settingsWindow := settingsView.Window()
	if err := settingsWindow.SetHideOnBlur(false); err != nil {
		return err
	}
	if err := settingsWindow.SetTextInputState(woxui.TextInputState{}); err != nil {
		return err
	}
	if settingsView.Lifecycle() == woxui.WindowLifecycleCreated {
		if err := settingsWindow.Center(woxui.Size{Width: settingsWindowWidth, Height: settingsWindowHeight}); err != nil {
			return err
		}
	}
	if err := a.notifySettingViewChanged(true); err != nil {
		return err
	}
	if _, err := settingsView.Show(); err != nil {
		_ = settingsView.Close()
		return err
	}
	a.updateSettingsTextInput(false)
	go a.loadSettingsSearchPlugins()
	return settingsWindow.Invalidate()
}

// reloadSettings refreshes the shared DTO without coupling the widget layer to Wox core packages.
func (a *App) reloadSettings() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var data settingsData
	if err := a.client.Post(ctx, "/setting/wox", map[string]any{}, &data); err != nil {
		return fmt.Errorf("load Wox settings: %w", err)
	}
	var languages []struct {
		Code string `json:"Code"`
		Name string `json:"Name"`
	}
	_ = a.client.Post(ctx, "/lang/available", map[string]any{}, &languages)
	languageChoices := make([]settingChoice, 0, len(languages))
	for _, language := range languages {
		if strings.TrimSpace(language.Code) != "" {
			languageChoices = append(languageChoices, settingChoice{value: language.Code, label: firstNonEmpty(language.Name, language.Code)})
		}
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
	aiForm.active = a.settingsOpen && a.settingTab == "ai"
	hotkeyForm.active = a.settingsOpen && a.settingTab == "general"
	a.settings = data
	a.settingLanguages = languageChoices
	a.aiSettingsForm = &aiForm
	a.hotkeySettingsForm = &hotkeyForm
	a.mu.Unlock()
	if a.window != nil {
		if err := a.window.SetFontFamily(data.AppFontFamily); err != nil {
			return fmt.Errorf("apply Wox UI font: %w", err)
		}
		_ = a.window.Invalidate()
	}
	if settingsWindow := a.settingsNativeWindow(); settingsWindow != nil {
		if err := settingsWindow.SetFontFamily(data.AppFontFamily); err != nil {
			return fmt.Errorf("apply Wox settings UI font: %w", err)
		}
	}
	return nil
}

func (a *App) closeSettings() error {
	a.stopHotkeyRecording()
	a.mu.RLock()
	settingsView := a.settingsView
	a.mu.RUnlock()
	if settingsView == nil {
		return nil
	}
	return settingsView.Close()
}

func (a *App) onSettingsKey(event woxui.KeyEvent) bool {
	if a.onPrivacySettingsKey(event) {
		return true
	}
	if a.onModelManagerKey(event) {
		return true
	}
	if a.onCloudSettingsKey(event) {
		return true
	}
	if a.onSettingChoicePickerKey(event) {
		return true
	}
	if a.onSettingsSearchKey(event) {
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
	var searchQuery woxui.TextEditingState
	if a.settingSearchEditor != nil {
		searchQuery = a.settingSearchEditor.State()
	}
	var pluginSearch woxui.TextEditingState
	if a.pluginSearchEditor != nil {
		pluginSearch = a.pluginSearchEditor.State()
	}
	var themeSearch woxui.TextEditingState
	if a.themeSearchEditor != nil {
		themeSearch = a.themeSearchEditor.State()
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
		isDev:                 a.isDev,
		titleBarHover:         a.settingsTitleBarHover,
		tab:                   a.settingTab,
		row:                   a.settingRow,
		note:                  a.settingNote,
		saving:                a.settingSaving,
		editKey:               a.settingEditKey,
		editing:               editing,
		searchQuery:           searchQuery,
		searchFocused:         a.settingSearchFocused,
		searchPanel:           a.settingSearchPanel,
		searchSelected:        a.settingSearchSelected,
		searchScroll:          a.settingSearchScroll,
		searchPlugins:         append([]pluginSettingsPlugin(nil), a.settingSearchPlugins...),
		searchLoading:         a.settingSearchLoading,
		searchError:           a.settingSearchError,
		choicePicker:          choicePicker,
		pageScroll:            a.settingPageScroll,
		railScroll:            a.settingRailScroll,
		languages:             append([]settingChoice(nil), a.settingLanguages...),
		updateChannelVersions: append([]updateChannelVersion(nil), a.updateChannelVersions...),
		data:                  a.settings,
		palette:               a.palette,
		plugins:               append([]pluginSettingsPlugin(nil), a.plugins...),
		pluginsLoading:        a.pluginsLoading,
		pluginsError:          a.pluginsError,
		pluginSelected:        a.pluginSelected,
		pluginListScroll:      a.pluginListScroll,
		pluginSearch:          pluginSearch,
		pluginSearchFocused:   a.pluginSearchFocused,
		pluginFilters:         a.pluginFilters,
		pluginFilterOpen:      a.pluginFilterOpen,
		pluginDetailTab:       a.pluginDetailTab,
		pluginForm:            snapshotPluginSettingsFormLocked(a.pluginForm),
		pluginsStore:          a.pluginsStore,
		pluginOperation:       a.pluginOperation,
		pluginOperationError:  a.pluginOperationError,
		pluginUninstallArmed:  a.pluginUninstallArmed,
		hotkeyForm:            hotkeyForm,
		hotkeyFocused:         a.settingsHotkeyFocus,
		glanceCatalog:         append([]glanceCatalogItem(nil), a.glanceCatalog...),
		glanceCatalogLoading:  a.glanceCatalogLoading,
		glanceCatalogError:    a.glanceCatalogError,
		systemFontFamilies:    append([]string(nil), a.systemFontFamilies...),
		systemFontsLoading:    a.systemFontsLoading,
		systemFontsError:      a.systemFontsError,
		themes:                append([]themeSettingsTheme(nil), a.themes...),
		themesMode:            a.themesMode,
		themesLoading:         a.themesLoading,
		themesError:           a.themesError,
		themeSelected:         a.themeSelected,
		themeListScroll:       a.themeListScroll,
		themeSearch:           themeSearch,
		themeSearchFocused:    a.themeSearchFocused,
		themeDetailTab:        a.themeDetailTab,
		themeOperation:        a.themeOperation,
		themeUninstallArmed:   a.themeUninstallArmed,
		aiForm:                aiForm,
		aiProvidersLoading:    a.aiProvidersLoading,
		aiProvidersError:      a.aiProvidersError,
		tableEditor:           tableEditor,
		modelManager:          modelManager,
		usage:                 cloneUsageStats(a.usageStats),
		usagePeriod:           a.usagePeriod,
		usageLoading:          a.usageLoading,
		usageError:            a.usageError,
		aboutVersion:          a.aboutVersion,
		aboutLoading:          a.aboutLoading,
		aboutError:            a.aboutError,
		privacySample:         a.privacySample,
		privacyError:          a.privacyError,
		dataBackups:           append([]backupInfo(nil), a.dataBackups...),
		dataLocation:          a.dataLocation,
		dataLoading:           a.dataLoading,
		dataBusy:              a.dataBusy,
		dataError:             a.dataError,
		dataRestoreArmed:      a.dataRestoreArmed,
		dataPendingLocation:   a.dataPendingLocation,
		dataClearLogsArmed:    a.dataClearLogsArmed,
		dataListScroll:        a.dataListScroll,
		runtimeStatuses:       cloneRuntimeStatuses(a.runtimeStatuses),
		runtimeLoading:        a.runtimeLoading,
		runtimeError:          a.runtimeError,
		runtimeRestarting:     a.runtimeRestarting,
		runtimePageScroll:     a.runtimePageScroll,
		cloudAccount:          a.cloudAccount,
		cloudSync:             a.cloudSync,
		cloudBillingPlan:      a.cloudBillingPlan,
		cloudBillingLoaded:    a.cloudBillingLoaded,
		cloudDevices:          cloneCloudDeviceList(a.cloudDevices),
		cloudLoading:          a.cloudLoading,
		cloudBusy:             a.cloudBusy,
		cloudError:            a.cloudError,
		cloudPageScroll:       a.cloudPageScroll,
		cloudForm:             cloudForm,
		cloudActionMenu:       a.cloudActionMenu,
		cloudPlugins:          append([]pluginSettingsPlugin(nil), a.cloudPlugins...),
		cloudPluginScroll:     a.cloudPluginScroll,
	}
}

func (a *App) selectSettingTab(tab string) {
	if tab == "debug" && !a.isDev {
		return
	}
	a.blurSettingsSearch()
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
	loadUpdateChannels := false
	a.mu.Lock()
	a.settingChoicePicker = nil
	if tab == "plugins" {
		if a.pluginSearchEditor == nil {
			a.pluginSearchEditor = woxui.NewTextEditor("")
		}
		a.pluginSearchFocused = true
		a.settingSearchFocused = false
		a.settingSearchPanel = false
	} else {
		a.pluginSearchFocused = false
	}
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
		a.settingPageScroll.reset()
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
		if tab != "theme" {
			a.themeSearchFocused = false
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
		a.hotkeySettingsForm.active = tab == "general"
		if tab == "general" {
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
	loadHotkeyApps = tab == "general" && !a.hotkeyAppsLoaded && !a.hotkeyAppsLoading
	loadGlanceCatalog = tab == "appearance" && !a.glanceCatalogLoaded && !a.glanceCatalogLoading
	loadSystemFonts = tab == "appearance" && !a.systemFontsLoaded && !a.systemFontsLoading
	loadData = tab == "data" && !a.dataLoaded && !a.dataLoading
	loadRuntime = tab == "runtime" && !a.runtimeLoaded && !a.runtimeLoading
	loadCloud = tab == "cloud" && !a.cloudLoaded && !a.cloudLoading
	loadUpdateChannels = tab == "updates" && len(a.updateChannelVersions) == 0 && !a.updateChannelsLoading
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
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
				a.invalidateSettingsWindow()
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
	if loadUpdateChannels {
		go a.reloadUpdateChannelVersions()
	}
	a.invalidateSettingsWindow()
}

// selectSettingsNavItem keeps hierarchical Flutter routes mapped onto the existing page and catalog state.
func (a *App) selectSettingsNavItem(item settingNavSpec) {
	if item.parent || item.tab == "" {
		return
	}
	a.mu.RLock()
	currentTab := a.settingTab
	a.mu.RUnlock()
	if item.tab == "plugins" {
		store := item.mode == "store"
		if currentTab == "plugins" {
			a.switchPluginList(store)
			return
		}
		a.mu.Lock()
		if a.pluginsStore != store {
			a.pluginsStore = store
			a.plugins = nil
			a.pluginsLoaded = false
			a.pluginsLoading = false
			a.pluginSelected = -1
			a.pluginForm = nil
			a.pluginListScroll = 0
		}
		a.mu.Unlock()
		a.selectSettingTab("plugins")
		return
	}
	if item.tab == "theme" {
		mode := item.mode
		if currentTab == "theme" {
			a.switchThemeSettingsMode(mode)
			return
		}
		a.mu.Lock()
		if a.themesMode != mode {
			a.themesMode = mode
			a.themes = nil
			a.themesLoaded = false
			a.themesLoading = false
			a.themeSelected = -1
			a.themeListScroll = 0
			a.themeSearchEditor = woxui.NewTextEditor("")
			a.themeSearchFocused = false
			a.themeDetailTab = "preview"
		}
		a.mu.Unlock()
		a.selectSettingTab("theme")
		return
	}
	a.selectSettingTab(item.tab)
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

func (a *App) scrollSettingsRail(delta float32) {
	a.mu.Lock()
	contentHeight := settingsRailContentHeight(len(settingNavSpecs(a.isDev)))
	a.settingRailScroll.setGeometry(max(float32(1), a.settingRailScroll.viewport), contentHeight)
	a.settingRailScroll.scrollBy(delta)
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) ensureSettingTabVisibleLocked(tabID string) {
	items := settingNavSpecs(a.isDev)
	activeID := activeSettingNavID(tabID, a.pluginsStore, a.themesMode)
	viewport := max(float32(1), a.settingRailScroll.viewport)
	a.settingRailScroll = resolveSettingsRailScroll(items, activeID, a.settingRailScroll, viewport, true)
}

// resolveSettingsRailScroll preserves manual scrolling unless a selection or viewport change must be revealed.
func resolveSettingsRailScroll(items []settingNavSpec, activeID string, current scrollController, viewport float32, followSelection bool) scrollController {
	scroll := current.withGeometry(viewport, settingsRailContentHeight(len(items)))
	if !followSelection {
		return scroll
	}
	for index, item := range items {
		if item.id != activeID {
			continue
		}
		top := float32(index * 50)
		bottom := top + 46
		scroll.ensureVisible(top, bottom)
		break
	}
	return scroll
}

// rememberSettingsRailGeometry keeps wheel input aligned with the offset rendered for the current settings route.
func (a *App) rememberSettingsRailGeometry(snapshot settingsSnapshot, scroll scrollController) {
	if scroll == snapshot.railScroll {
		return
	}
	a.mu.Lock()
	if a.settingTab == snapshot.tab && a.pluginsStore == snapshot.pluginsStore && a.themesMode == snapshot.themesMode && a.settingRailScroll == snapshot.railScroll {
		a.settingRailScroll = scroll
	}
	a.mu.Unlock()
}

func settingsRailContentHeight(tabCount int) float32 {
	return float32(tabCount * 50)
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
	a.invalidateSettingsWindow()
}

func (a *App) selectSettingRow(index int) {
	a.blurSettingsSearch()
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
		a.invalidateSettingsWindow()
		return
	}
	a.settingRow = index
	a.settingsHotkeyFocus = false
	// Pointer-selected rows are already visible; moving the viewport here would invalidate popup anchors captured by the same click.
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

// setSettingsPageGeometry records the measured page without taking scroll ownership from pointer input.
func (a *App) setSettingsPageGeometry(height, contentHeight float32) {
	a.mu.Lock()
	a.settingPageScroll.setGeometry(max(float32(1), height), contentHeight)
	a.mu.Unlock()
}

// rememberSettingsPageGeometry adopts render-time measurements only while the page snapshot is current.
func (a *App) rememberSettingsPageGeometry(snapshot settingsSnapshot, scroll scrollController) {
	if scroll == snapshot.pageScroll {
		return
	}
	a.mu.Lock()
	if a.settingTab == snapshot.tab && a.settingRow == snapshot.row && a.settingPageScroll == snapshot.pageScroll {
		a.settingPageScroll = scroll
	}
	a.mu.Unlock()
}

func (a *App) scrollSettingsPage(delta float32) {
	a.mu.Lock()
	a.settingPageScroll.scrollBy(delta)
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) ensureSettingRowVisibleLocked(itemCount int) {
	if a.settingRow < 0 || a.settingRow >= itemCount {
		return
	}
	if a.settingTab == "runtime" {
		a.ensureRuntimeSettingRowVisibleLocked()
		return
	}
	viewport := max(float32(1), a.settingPageScroll.viewport)
	top := float32(74 + a.settingRow*79)
	bottom := top + 70
	a.settingPageScroll.setGeometry(viewport, a.settingPageScroll.content)
	a.settingPageScroll.ensureVisible(top, bottom)
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
	a.invalidateSettingsWindow()
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
	a.updateSettingsTextInput(true)
	a.invalidateSettingsWindow()
}

// cancelBuiltInSettingEdit discards an unsaved text value without mutating the loaded settings DTO.
func (a *App) cancelBuiltInSettingEdit() {
	a.mu.Lock()
	a.settingEditKey = ""
	a.settingEditor = nil
	a.settingNote = ""
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
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
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	go a.saveSetting(item, settingChoice{value: value, label: value})
}

// onBuiltInSettingsEditorKey keeps text editing separate from rail and choice navigation.
func (a *App) onBuiltInSettingsEditorKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	active := a.settingsOpen && a.settingEditKey != "" && a.settingEditor != nil
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
	a.invalidateSettingsWindow()
	return true
}

// onBuiltInSettingsTextInput commits native text and IME events into the active settings editor.
func (a *App) onBuiltInSettingsTextInput(event woxui.TextInputEvent) bool {
	if a.onSettingChoicePickerTextInput(event) {
		return true
	}
	a.mu.Lock()
	if !a.settingsOpen || a.settingSaving || a.settingEditKey == "" || a.settingEditor == nil {
		a.mu.Unlock()
		return false
	}
	a.settingEditor.HandleTextInput(event)
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	return true
}

// browseBuiltInSettingFile uses the common Window picker and leaves persistence on explicit Enter.
func (a *App) browseBuiltInSettingFile(item settingItem) {
	if !item.text || !item.browseFile {
		return
	}
	settingsWindow := a.settingsNativeWindow()
	if settingsWindow == nil {
		return
	}
	path, err := settingsWindow.PickFile(woxui.FileDialogOptions{})
	if err != nil {
		a.mu.Lock()
		a.settingNote = "Could not select " + item.title + ": " + err.Error()
		a.mu.Unlock()
		a.invalidateSettingsWindow()
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
	a.invalidateSettingsWindow()
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
	if err == nil && item.key == "LangCode" {
		err = a.reloadTranslations()
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
		a.settingNote = ""
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
		a.updateSettingsTextInput(true)
	} else {
		a.updateSettingsTextInput(false)
	}
	if refreshGlance {
		go a.refreshGlance("settingsChanged", "", nil)
	}
	if err == nil && (item.key == "CustomPythonPath" || item.key == "CustomNodejsPath") {
		go a.reloadRuntimeStatuses()
	}
	a.publishSettingsChanged(item.key)
}

func settingTabForPath(path string) (string, string) {
	switch strings.TrimSpace(path) {
	case "", "/", "/general":
		return "general", ""
	case "/ui", "/appearance":
		return "appearance", ""
	case "/hotkeys", "hotkeys", "/query/hotkeys":
		return "general", ""
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
	if snapshot.tab == "ai" || snapshot.tab == "data" || snapshot.tab == "cloud" || snapshot.tab == "plugins" || snapshot.tab == "theme" || snapshot.tab == "about" {
		return nil
	}
	items := settingItems(snapshot.tab, snapshot.data)
	if snapshot.tab == "updates" {
		for index := range items {
			if items[index].key == "ReleaseChannel" {
				items[index].trailers = updateChannelVersionTrailers(snapshot.updateChannelVersions)
				break
			}
		}
	}
	if snapshot.tab == "general" && len(snapshot.languages) > 0 {
		for index := range items {
			if items[index].key == "LangCode" {
				items[index].choices = append([]settingChoice(nil), snapshot.languages...)
				break
			}
		}
	}
	if snapshot.tab == "appearance" {
		font := systemFontSettingItem(snapshot)
		insertAt := min(4, len(items))
		items = append(items[:insertAt], append([]settingItem{font}, items[insertAt:]...)...)
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
			{key: "ShowPosition", title: "Window position", description: "Display used when Wox opens", value: data.ShowPosition, choices: []settingChoice{{"mouse_screen", "Mouse display"}, {"active_screen", "Active display"}, {"last_location", "Last location"}}},
			{key: "ShowTray", title: "Tray icon", description: "Show Wox in the system tray or menu bar", value: boolValue(data.ShowTray), choices: boolChoices},
			{key: "AppWidth", title: "Launcher width", description: "Logical width of the query and result window", value: fmt.Sprintf("%d", data.AppWidth), choices: widthChoices},
			{key: "UiDensity", title: "UI density", description: "Spacing and row size across the launcher", value: data.UIDensity, choices: []settingChoice{{"compact", "Compact"}, {"normal", "Normal"}, {"comfortable", "Comfortable"}}},
			{key: "EnableQueryCompletionHint", title: "Query completion hints", description: "Show completion text while typing", value: boolValue(data.EnableQueryCompletionHint), choices: boolChoices},
			{key: "MaxResultCount", title: "Maximum results", description: "Number of result rows visible before scrolling", value: fmt.Sprintf("%d", data.MaxResultCount), choices: resultChoices},
			{key: "EnableGlance", title: "Glance", description: "Show glance content beside the query", value: boolValue(data.EnableGlance), choices: boolChoices},
			{key: "HideGlanceIcon", title: "Hide glance icon", description: "Keep the query box visually minimal", value: boolValue(data.HideGlanceIcon), choices: boolChoices},
		}
	case "network":
		return []settingItem{
			{key: "HttpProxyEnabled", title: "HTTP proxy", value: boolValue(data.HttpProxyEnabled), choices: boolChoices},
			{key: "HttpProxyUrl", title: "Proxy URL", value: data.HttpProxyURL, text: true, controlWidth: 300, disabled: !data.HttpProxyEnabled},
		}
	case "runtime":
		return []settingItem{
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
			{key: "EnableAutoUpdate", title: "Enable auto update", description: "Download updates in the background and wait for confirmation before installing", value: boolValue(data.EnableAutoUpdate), choices: boolChoices},
			{key: "ReleaseChannel", title: "Update channel", description: "Choose whether Wox checks the stable update channel or the beta update channel", value: data.ReleaseChannel, choices: []settingChoice{{"stable", "Stable channel"}, {"beta", "Beta channel"}}},
		}
	case "privacy":
		return []settingItem{
			{key: "EnableAnonymousUsageStats", title: "Anonymous usage stats", description: "Help improve Wox with anonymous telemetry", value: boolValue(data.EnableAnonymousUsageStats), choices: boolChoices},
		}
	default:
		return []settingItem{
			{key: "EnableAutostart", title: "Start at login", description: "Launch Wox when the desktop session starts", value: boolValue(data.EnableAutostart), choices: boolChoices},
			{key: "HideOnStart", title: "Start hidden", description: "Keep Wox hidden after startup", value: boolValue(data.HideOnStart), choices: boolChoices},
			{key: "LaunchMode", title: "Launch mode", description: "Start fresh or continue the previous query", value: data.LaunchMode, choices: []settingChoice{{"fresh", "Fresh"}, {"continue", "Continue"}}},
			{key: "StartPage", title: "Start page", description: "Content shown for an empty query", value: data.StartPage, choices: []settingChoice{{"blank", "Blank"}, {"mru", "Recent"}}},
			{key: "HideOnLostFocus", title: "Hide on focus loss", description: "Dismiss the launcher when focus moves away", value: boolValue(data.HideOnLostFocus), choices: boolChoices},
			{key: "UsePinYin", title: "Pinyin search", description: "Match Chinese text with Pinyin", value: boolValue(data.UsePinYin), choices: boolChoices},
			{key: "SwitchInputMethodABC", title: "Switch input method", description: "Use the Latin input source when Wox opens", value: boolValue(data.SwitchInputMethodABC), choices: boolChoices},
			{key: "LangCode", title: "Language", description: "Language used by Wox", value: data.LangCode, choices: []settingChoice{{data.LangCode, data.LangCode}}},
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
