package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
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
	OnboardingFinished                 bool
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
	EnablePrivacyMode                  bool
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
	key               string
	title             string
	description       string
	value             string
	choices           []settingChoice
	trailers          map[string]string
	icons             map[string]woxImage
	preserveIconColor bool
	filterable        bool
	text              bool
	controlWidth      float32
	browseFile        bool
	disabled          bool
}

type settingsSnapshot struct {
	isDev       bool
	tab         string
	row         int
	note        string
	saving      bool
	highlight   string
	search      settingsSearchSnapshot
	update      updateSettingsSnapshot
	palette     uiPalette
	plugins     pluginSettingsSnapshot
	hotkey      hotkeySettingsSnapshot
	appearance  appearanceSettingsSnapshot
	theme       themeSettingsSnapshot
	ai          aiSettingsSnapshot
	tableEditor *formTableEditorSnapshot
	usage       usageSettingsSnapshot
	about       aboutSettingsSnapshot
	privacy     privacySettingsSnapshot
	dataState   dataSettingsSnapshot
	network     networkSettingsSnapshot
	runtime     runtimeSettingsSnapshot
	cloud       cloudSettingsSnapshot
	general     generalSettingsSnapshot
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
	wasOnboarding := false
	var onboardingView *woxui.ManagedWindow
	if err := a.runOnUI("leave onboarding for settings", func() {
		wasOnboarding = a.onboardingOpen
		a.onboardingOpen = false
		a.onboardingChoice = ""
		a.onboardingChoiceAnchor = woxui.Rect{}
		if wasOnboarding {
			a.releaseDemoWallpaperLocked()
		}
		onboardingView = a.onboardingView
	}); err != nil {
		return err
	}
	if wasOnboarding {
		if onboardingView != nil {
			_ = onboardingView.Hide()
		}
		if err := a.notifyOnboardingViewChanged(false); err != nil {
			return err
		}
	}
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
		if err := a.runOnUI("prepare plugin settings route", func() {
			a.pluginSettings.SetPluginsStore(store)
		}); err != nil {
			return err
		}
		if err := a.reloadPlugins(store, windowContext.Param); err != nil {
			note = "Could not load plugins: " + err.Error()
		}
	}
	if err := a.runOnUI("open settings state", func() {
		a.settingsOpen = true
		a.settingsCtx = windowContext
		a.settingTab = tab
		a.settingRow = 0
		a.settingNote = note
		a.settingSaving = false
		a.settingsSearch.SetEditor(woxwidget.NewTextEditingController(""))
		a.settingsSearch.SetFocused(tab != "plugins")
		a.settingsSearch.SetPanel(false)
		a.settingsSearch.SetSelected(0)
		if tab == "plugins" {
			if a.pluginSettings.SearchEditor() == nil {
				a.pluginSettings.SetSearchEditor(woxui.NewTextEditor(""))
			}
			a.pluginSettings.SetSearchFocused(true)
		} else {
			a.pluginSettings.SetSearchFocused(false)
		}
		a.hotkeySettings.SetFocused(false)
		a.aiSettings.SetModelManager(nil)
		a.cloudSettings.SetForm(nil)
		a.cloudPlanTooltip = nil
		a.settingsDemo = nil
		a.settingsDemoRevision.Add(1)
		a.cloudSettings.SetActionMenu("")
		a.form = nil
		a.requirementForm = nil
		a.launcherTableEditor = nil
		a.triggerConflict = nil
		a.themeSettings.SetThemeEditor(nil)
		if form := a.hotkeySettings.Form(); form != nil {
			form.active = tab == "general"
		}
		if form := a.aiSettings.Form(); form != nil {
			form.active = tab == "ai"
		}
		if tab == "theme" {
			a.themeSettings.SetThemesMode(themeMode)
			a.themeSettings.SetThemes(nil)
			a.themeSettings.SetThemesLoaded(false)
			a.themeSettings.SetThemesLoading(false)
			a.themeSettings.SetThemesError("")
			a.themeSettings.SetThemeSelected(-1)
			a.themeSettings.SetThemeSearchEditor(woxui.NewTextEditor(""))
			a.themeSettings.SetThemeSearchFocused(false)
			a.themeSettings.SetThemeDetailTab("preview")
			a.themeSettings.SetThemeOperation("")
			a.themeSettings.SetThemeUninstallArmed("")
		}
		if form := a.pluginSettings.Form(); form != nil {
			form.active = false
		}
		// Reset the shared built-in editor and any open choice picker on settings open.
		a.generalSettings.EndEdit()
		a.generalSettings.SetChoicePicker(nil)
		a.deactivateTerminalPreview()
		a.resetChatPreview()
	}); err != nil {
		return err
	}
	if tab == "theme" && themeMode == "editor" {
		if err := a.loadSettingsThemeEditor(); err != nil {
			_ = a.runOnUI("apply theme editor load error", func() {
				a.settingNote = "Could not load theme editor: " + err.Error()
			})
		}
	}
	if tab == "theme" && themeMode != "editor" {
		if err := a.reloadThemes(themeMode, ""); err != nil {
			_ = a.runOnUI("apply theme catalog load error", func() {
				a.settingNote = "Could not load themes: " + err.Error()
			})
		}
	}
	if tab == "usage" {
		util.Go(a.lifecycleCtx, "reload usage stats", func() {
			a.reloadUsageStats(a.currentUsagePeriod())
		})
	}
	if tab == "ai" {
		util.Go(a.lifecycleCtx, "load AI provider catalog", a.loadAIProviderCatalog)
	}
	if tab == "general" {
		util.Go(a.lifecycleCtx, "load hotkey app candidates", a.loadHotkeyAppCandidates)
	}
	if tab == "appearance" {
		util.Go(a.lifecycleCtx, "load glance catalog", a.loadGlanceCatalog)
		util.Go(a.lifecycleCtx, "load system font families", a.loadSystemFontFamilies)
	}
	if tab == "data" {
		util.Go(a.lifecycleCtx, "reload data settings", a.reloadDataSettings)
	}
	if tab == "cloud" {
		util.Go(a.lifecycleCtx, "reload cloud sync", a.reloadCloudSync)
	}
	if tab == "runtime" {
		util.Go(a.lifecycleCtx, "reload runtime statuses", a.reloadRuntimeStatuses)
	}
	if tab == "about" {
		util.Go(a.lifecycleCtx, "reload about version", a.reloadAboutVersion)
	}
	if tab == "privacy" {
		util.Go(a.lifecycleCtx, "reload privacy version", a.reloadAboutVersion)
	}
	util.Go(a.lifecycleCtx, "reload update channel versions", a.reloadUpdateChannelVersions)

	settingsView, err := a.ensureSettingsWindow()
	if err != nil {
		_ = a.runOnUI("rollback settings open", func() {
			a.settingsOpen = false
			a.releaseDemoWallpaperLocked()
		})
		return err
	}
	settingsWindow := settingsView.Window()
	if err := settingsWindow.SetHideOnBlur(false); err != nil {
		return err
	}
	if err := settingsWindow.SetTextInputState(woxui.TextInputState{}); err != nil {
		return err
	}
	if err := settingsWindow.Center(woxui.Size{Width: settingsWindowWidth, Height: settingsWindowHeight}); err != nil {
		return err
	}
	if err := a.notifySettingViewChanged(true); err != nil {
		return err
	}
	if _, err := settingsView.Show(); err != nil {
		_ = settingsView.Close()
		return err
	}
	a.updateSettingsTextInput(false)
	util.Go(a.lifecycleCtx, "load settings search plugins", a.loadSettingsSearchPlugins)
	if _, loaded := a.pluginSettings.CachedPlugins(true); !loaded {
		util.Go(a.lifecycleCtx, "preload plugin store", func() {
			if err := a.pluginSettings.PreloadPlugins(a.lifecycleCtx, a.services, a.sessionID, true); err != nil {
				log.Printf("preload plugin store: %v", err)
			}
		})
	}
	return settingsWindow.Invalidate()
}

// reloadSettings refreshes the shared settings snapshot and language catalog.
func (a *App) reloadSettings() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	loaded, err := a.services.GeneralSettings(ctx, a.sessionID)
	if err != nil {
		return fmt.Errorf("load Wox settings: %w", err)
	}
	data, err := settingsDataFromContract(loaded)
	if err != nil {
		return err
	}
	languages, _ := a.services.AvailableLanguages(ctx, a.sessionID)
	languageChoices := make([]settingChoice, 0, len(languages))
	for _, language := range languages {
		code := string(language.Code)
		if strings.TrimSpace(code) != "" {
			languageChoices = append(languageChoices, settingChoice{value: code, label: firstNonEmpty(language.Name, code)})
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
	var applyErr error
	densityChanged := false
	if err := a.runOnUI("apply general settings snapshot", func() {
		aiForm := newAISettingsForm(data)
		hotkeyForm := newHotkeySettingsForm(data)
		applyAIProviderCatalogLocked(&aiForm, a.aiSettings.ProviderCatalog())
		aiForm.active = a.settingsOpen && a.settingTab == "ai"
		hotkeyForm.active = a.settingsOpen && a.settingTab == "general"
		a.aiSettings.SetForm(&aiForm)
		a.hotkeySettings.SetForm(&hotkeyForm)

		nextDensityMetrics := launcherDensityMetricsFor(data.UIDensity)
		densityChanged = a.densityMetrics != nextDensityMetrics
		a.densityMetrics = nextDensityMetrics
		// Domain controllers receive the same authoritative payload in one UI transaction.
		a.generalSettings.ApplyData(data)
		a.generalSettings.SetLanguages(languageChoices)
		a.networkSettings.ApplyData(data.HttpProxyEnabled, data.HttpProxyURL)
		if a.window != nil {
			if err := a.window.SetFontFamily(data.AppFontFamily); err != nil {
				applyErr = fmt.Errorf("apply Wox UI font: %w", err)
				return
			}
			_ = a.window.Invalidate()
		}
		if a.settingsView != nil {
			if err := a.settingsView.Window().SetFontFamily(data.AppFontFamily); err != nil {
				applyErr = fmt.Errorf("apply Wox settings UI font: %w", err)
			}
		}
		if a.onboardingView != nil {
			if err := a.onboardingView.Window().SetFontFamily(data.AppFontFamily); err != nil {
				applyErr = fmt.Errorf("apply Wox onboarding UI font: %w", err)
			}
		}
	}); err != nil {
		return err
	}
	if applyErr != nil {
		return applyErr
	}
	if densityChanged && a.window != nil {
		return a.applyWindowBounds()
	}
	return nil
}

// settingsDataFromContract adapts core domain types to launcher-owned form state.
func settingsDataFromContract(loaded contract.GeneralSettings) (settingsData, error) {
	ignoredHotkeyApps, err := json.Marshal(loaded.IgnoredHotkeyApps)
	if err != nil {
		return settingsData{}, fmt.Errorf("encode ignored hotkey apps: %w", err)
	}
	trayQueries, err := json.Marshal(loaded.TrayQueries)
	if err != nil {
		return settingsData{}, fmt.Errorf("encode tray queries: %w", err)
	}
	aiProviders, err := json.Marshal(loaded.AIProviders)
	if err != nil {
		return settingsData{}, fmt.Errorf("encode AI providers: %w", err)
	}
	mcpServers, err := json.Marshal(loaded.AIMCPServers)
	if err != nil {
		return settingsData{}, fmt.Errorf("encode AI MCP servers: %w", err)
	}
	aiSkills, err := json.Marshal(loaded.AISkills)
	if err != nil {
		return settingsData{}, fmt.Errorf("encode AI skills: %w", err)
	}

	queryHotkeys := make([]queryHotkeySetting, len(loaded.QueryHotkeys))
	for index, item := range loaded.QueryHotkeys {
		queryHotkeys[index] = queryHotkeySetting{
			Name: item.Name, Hotkey: item.Hotkey, Query: item.Query, IsSilentExecution: item.IsSilentExecution,
			HideQueryBox: item.HideQueryBox, HideToolbar: item.HideToolbar, Width: item.Width,
			MaxResultCount: item.MaxResultCount, Position: string(item.Position), Disabled: item.Disabled,
		}
	}
	queryShortcuts := make([]queryShortcutSetting, len(loaded.QueryShortcuts))
	for index, item := range loaded.QueryShortcuts {
		queryShortcuts[index] = queryShortcutSetting{Shortcut: item.Shortcut, Query: item.Query, Disabled: item.Disabled}
	}
	return settingsData{
		EnableAutostart:                    loaded.EnableAutostart,
		LogLevel:                           loaded.LogLevel,
		MainHotkey:                         loaded.MainHotkey,
		SelectionHotkey:                    loaded.SelectionHotkey,
		IgnoredHotkeyApps:                  ignoredHotkeyApps,
		QueryHotkeys:                       queryHotkeys,
		QueryShortcuts:                     queryShortcuts,
		TrayQueries:                        trayQueries,
		IsLinuxWaylandSession:              loaded.IsLinuxWaylandSession,
		UsePinYin:                          loaded.UsePinYin,
		SwitchInputMethodABC:               loaded.SwitchInputMethodABC,
		HideOnStart:                        loaded.HideOnStart,
		OnboardingFinished:                 loaded.OnboardingFinished,
		HideOnLostFocus:                    loaded.HideOnLostFocus,
		ShowTray:                           loaded.ShowTray,
		LangCode:                           string(loaded.LangCode),
		LaunchMode:                         string(loaded.LaunchMode),
		StartPage:                          string(loaded.StartPage),
		HttpProxyEnabled:                   loaded.HTTPProxyEnabled,
		HttpProxyURL:                       loaded.HTTPProxyURL,
		ShowPosition:                       string(loaded.ShowPosition),
		EnableAutoBackup:                   loaded.EnableAutoBackup,
		EnableAutoUpdate:                   loaded.EnableAutoUpdate,
		ReleaseChannel:                     string(loaded.ReleaseChannel),
		EnableAnonymousUsageStats:          loaded.EnableAnonymousUsageStats,
		EnablePrivacyMode:                  loaded.EnablePrivacyMode,
		CustomPythonPath:                   loaded.CustomPythonPath,
		CustomNodejsPath:                   loaded.CustomNodejsPath,
		CloudSyncServerURL:                 loaded.CloudSyncServerURL,
		AppWidth:                           loaded.AppWidth,
		MaxResultCount:                     loaded.MaxResultCount,
		UIDensity:                          string(loaded.UIDensity),
		ThemeID:                            loaded.ThemeID,
		AppFontFamily:                      loaded.AppFontFamily,
		EnableQueryCompletionHint:          loaded.EnableQueryCompletionHint,
		EnableGlance:                       loaded.EnableGlance,
		PrimaryGlance:                      glanceRef{PluginID: loaded.PrimaryGlance.PluginId, GlanceID: loaded.PrimaryGlance.GlanceId},
		HideGlanceIcon:                     loaded.HideGlanceIcon,
		AIProviders:                        aiProviders,
		AIMCPServers:                       mcpServers,
		AISkills:                           aiSkills,
		CloudSyncDisabledPlugins:           append([]string(nil), loaded.CloudSyncDisabledPlugins...),
		ShowScoreTail:                      loaded.ShowScoreTail,
		ShowPerformanceTail:                loaded.ShowPerformanceTail,
		ShowPerformanceTailBatch:           loaded.ShowPerformanceTailBatch,
		ShowPerformanceTailPluginQuery:     loaded.ShowPerformanceTailPluginQuery,
		ShowPerformanceTailBackendPrepared: loaded.ShowPerformanceTailBackendPrepared,
		ShowPerformanceTailUIReceived:      loaded.ShowPerformanceTailUIReceived,
	}, nil
}

func (a *App) closeSettings() error {
	var settingsView *woxui.ManagedWindow
	if err := a.runOnUI("prepare settings close", func() {
		a.stopHotkeyRecording()
		if form := a.pluginSettings.Form(); form != nil {
			syncFormFieldsEditorLocked(&form.formFieldsState)
			if pluginFormDirty(form.definitions, form.values, form.initial) {
				a.submitPluginSettings()
			}
		}
		settingsView = a.settingsView
	}); err != nil {
		return err
	}
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
	choicePicker := a.generalSettings.ChoicePicker()
	if choicePicker != nil {
		if event.Key == woxui.KeyEscape {
			a.closeSettingChoicePicker()
			return true
		}
		// Printable keys must reach the native text-input path while the filter field owns focus.
		return !choicePicker.item.filterable
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
	themeTab := a.settingTab == "theme"
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
	search := a.settingsSearch.Snapshot()
	update := a.updateSettings.Snapshot()
	plugins := a.pluginSettings.Snapshot()
	hotkey := a.hotkeySettings.Snapshot()
	appearance := a.appearanceSettings.Snapshot()
	theme := a.themeSettings.Snapshot()
	ai := a.aiSettings.Snapshot()
	usage := a.usageSettings.Snapshot()
	about := a.aboutSettings.Snapshot()
	privacy := a.privacySettings.Snapshot()
	dataState := a.dataSettings.Snapshot()
	network := a.networkSettings.Snapshot()
	runtime := a.runtimeSettings.Snapshot()
	cloud := a.cloudSettings.Snapshot()
	general := a.generalSettings.Snapshot()

	// Resolve controller-owned form pointers in the same UI-thread snapshot transaction.
	pluginForm := a.pluginSettings.Form()
	aiForm := a.aiSettings.Form()
	hotkeyForm := a.hotkeySettings.Form()

	var tableEditor *formTableEditorSnapshot
	if a.settingsTableEditor != nil && a.formTableTargetCurrentWithFormsLocked(a.settingsTableEditor.target, pluginForm, aiForm, hotkeyForm) {
		tableEditor = snapshotFormTableEditorLocked(a.settingsTableEditor)
	}
	return settingsSnapshot{
		isDev:       a.isDev,
		tab:         a.settingTab,
		row:         a.settingRow,
		note:        a.settingNote,
		saving:      a.settingSaving,
		highlight:   a.settingFlash,
		search:      search,
		update:      update,
		palette:     a.palette,
		plugins:     plugins,
		hotkey:      hotkey,
		appearance:  appearance,
		theme:       theme,
		ai:          ai,
		tableEditor: tableEditor,
		usage:       usage,
		about:       about,
		privacy:     privacy,
		dataState:   dataState,
		network:     network,
		runtime:     runtime,
		cloud:       cloud,
		general:     general,
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
	a.generalSettings.SetChoicePicker(nil)
	if tab == "plugins" {
		if a.pluginSettings.SearchEditor() == nil {
			a.pluginSettings.SetSearchEditor(woxui.NewTextEditor(""))
		}
		a.pluginSettings.SetSearchFocused(true)
		a.settingsSearch.SetFocused(false)
		a.settingsSearch.SetPanel(false)
	} else {
		a.pluginSettings.SetSearchFocused(false)
	}
	if a.settingTab != tab {
		if form := a.pluginSettings.Form(); form != nil {
			syncFormFieldsEditorLocked(&form.formFieldsState)
			form.active = false
			if pluginFormDirty(form.definitions, form.values, form.initial) {
				a.submitPluginSettings()
			}
		}
		a.settingTab = tab
		a.settingRow = 0
		a.settingNote = ""
		a.generalSettings.EndEdit()
		a.cloudSettings.SetForm(nil)
		a.cloudPlanTooltip = nil
		a.settingsDemo = nil
		a.settingsDemoRevision.Add(1)
		if tab != "plugins" {
			a.aiSettings.SetModelManager(nil)
		}
		if themeEditor := a.themeSettings.ThemeEditor(); themeEditor != nil {
			themeEditor.active = false
		}
		if tab != "theme" {
			a.themeSettings.SetThemeSearchFocused(false)
		}
	}
	if form := a.aiSettings.Form(); form != nil {
		form.active = tab == "ai"
		if tab == "ai" {
			setFormFieldsFocusLocked(form, 0)
		}
	}
	if hotkeyForm := a.hotkeySettings.Form(); hotkeyForm != nil {
		hotkeyForm.active = tab == "general"
		if tab == "general" {
			setFormFieldsFocusLocked(hotkeyForm, max(0, hotkeyForm.focused))
		}
	}
	if tab == "theme" && a.themeSettings.ThemesMode() == "" {
		a.themeSettings.SetThemesMode("installed")
	}
	pluginSnap := a.pluginSettings.Snapshot()
	loadPlugins = tab == "plugins" && !pluginSnap.PluginsLoaded && !pluginSnap.PluginsLoading
	themeEditor := a.themeSettings.ThemeEditor()
	loadTheme = tab == "theme" && a.themeSettings.ThemesMode() == "editor" && (themeEditor == nil || !strings.HasPrefix(themeEditor.key, "settings-theme|"))
	loadThemes = tab == "theme" && a.themeSettings.ThemesMode() != "editor" && !a.themeSettings.ThemesLoaded() && !a.themeSettings.ThemesLoading()
	usageSnap := a.usageSettings.Snapshot()
	loadUsage = tab == "usage" && !usageSnap.Loaded && !usageSnap.Loading
	aboutSnap := a.aboutSettings.Snapshot()
	loadAbout = (tab == "about" || tab == "privacy") && !aboutSnap.Loaded && !aboutSnap.Loading
	aiSnap := a.aiSettings.Snapshot()
	loadAIProviders = tab == "ai" && !aiSnap.ProvidersLoaded && !aiSnap.ProvidersLoading
	hotkeySnap := a.hotkeySettings.Snapshot()
	loadHotkeyApps = tab == "general" && !hotkeySnap.AppsLoaded && !hotkeySnap.AppsLoading
	appearanceSnap := a.appearanceSettings.Snapshot()
	loadGlanceCatalog = tab == "appearance" && !appearanceSnap.GlanceCatalogLoaded && !appearanceSnap.GlanceCatalogLoading
	loadSystemFonts = tab == "appearance" && !appearanceSnap.FontsLoaded && !appearanceSnap.FontsLoading
	dataSnap := a.dataSettings.Snapshot()
	loadData = tab == "data" && !dataSnap.Loaded && !dataSnap.Loading
	runtimeSnap := a.runtimeSettings.Snapshot()
	loadRuntime = tab == "runtime" && !runtimeSnap.Loaded && !runtimeSnap.Loading
	loadCloud = tab == "cloud" && !a.cloudSettings.Loaded() && !a.cloudSettings.Loading()
	updateSnap := a.updateSettings.Snapshot()
	loadUpdateChannels = tab == "updates" && len(updateSnap.ChannelVersions) == 0 && !updateSnap.ChannelsLoading
	pluginStore := a.pluginSettings.PluginsStore()
	a.updateSettingsTextInput(false)
	if loadPlugins {
		util.Go(a.lifecycleCtx, "reload settings plugins", func() {
			if err := a.reloadPlugins(pluginStore, ""); err != nil {
				log.Printf("load plugins: %v", err)
			}
		})
	}
	if loadTheme {
		util.Go(a.lifecycleCtx, "load settings theme editor", func() {
			if err := a.loadSettingsThemeEditor(); err != nil {
				_ = a.runOnUI("apply theme editor load error", func() {
					a.settingNote = "Could not load theme editor: " + err.Error()
					a.invalidateSettingsWindow()
				})
			}
		})
	}
	if loadThemes {
		mode := a.themeSettings.ThemesMode()
		util.Go(a.lifecycleCtx, "reload settings themes", func() {
			if err := a.reloadThemes(mode, ""); err != nil {
				log.Printf("load themes: %v", err)
			}
		})
	}
	if loadUsage {
		util.Go(a.lifecycleCtx, "reload usage stats", func() {
			a.reloadUsageStats(a.currentUsagePeriod())
		})
	}
	if loadAbout {
		util.Go(a.lifecycleCtx, "reload about version", a.reloadAboutVersion)
	}
	if loadAIProviders {
		util.Go(a.lifecycleCtx, "load AI provider catalog", a.loadAIProviderCatalog)
	}
	if loadHotkeyApps {
		util.Go(a.lifecycleCtx, "load hotkey app candidates", a.loadHotkeyAppCandidates)
	}
	if loadGlanceCatalog {
		util.Go(a.lifecycleCtx, "load glance catalog", a.loadGlanceCatalog)
	}
	if loadSystemFonts {
		util.Go(a.lifecycleCtx, "load system font families", a.loadSystemFontFamilies)
	}
	if loadData {
		util.Go(a.lifecycleCtx, "reload data settings", a.reloadDataSettings)
	}
	if loadRuntime {
		util.Go(a.lifecycleCtx, "reload runtime statuses", a.reloadRuntimeStatuses)
	}
	if loadCloud {
		util.Go(a.lifecycleCtx, "reload cloud sync", a.reloadCloudSync)
	}
	if loadUpdateChannels {
		util.Go(a.lifecycleCtx, "reload update channel versions", a.reloadUpdateChannelVersions)
	}
	a.invalidateSettingsWindow()
}

// selectSettingsNavItem keeps hierarchical Flutter routes mapped onto the existing page and catalog state.
func (a *App) selectSettingsNavItem(item settingNavSpec) {
	if item.parent || item.tab == "" {
		return
	}
	currentTab := a.settingTab
	if item.tab == "plugins" {
		store := item.mode == "store"
		if currentTab == "plugins" {
			a.switchPluginList(store)
			return
		}
		if a.pluginSettings.PluginsStore() != store {
			a.pluginSettings.SetPluginsStore(store)
			a.pluginSettings.SetPlugins(nil)
			a.pluginSettings.SetPluginsLoaded(false)
			a.pluginSettings.SetPluginsLoading(false)
			a.pluginSettings.SetSelected(-1)
			a.pluginSettings.SetForm(nil)
		}
		a.selectSettingTab("plugins")
		return
	}
	if item.tab == "theme" {
		mode := item.mode
		if currentTab == "theme" {
			a.switchThemeSettingsMode(mode)
			return
		}
		if a.themeSettings.ThemesMode() != mode {
			a.themeSettings.SetThemesMode(mode)
			a.themeSettings.SetThemes(nil)
			a.themeSettings.SetThemesLoaded(false)
			a.themeSettings.SetThemesLoading(false)
			a.themeSettings.SetThemeSelected(-1)
			a.themeSettings.SetThemeSearchEditor(woxui.NewTextEditor(""))
			a.themeSettings.SetThemeSearchFocused(false)
			a.themeSettings.SetThemeDetailTab("preview")
		}
		a.selectSettingTab("theme")
		return
	}
	a.selectSettingTab(item.tab)
}

func (a *App) moveSettingTab(delta int) {
	current := a.settingTab
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

func (a *App) moveSettingRow(delta int) {
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	if len(items) == 0 {
		return
	}
	a.settingRow = (a.settingRow + delta + len(items)) % len(items)
	a.invalidateSettingsWindow()
}

func (a *App) selectSettingRow(index int) {
	a.blurSettingsSearch()
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	if index < 0 || index >= len(items) {
		return
	}
	if a.generalSettings.EditKey() != "" {
		if a.settingRow != index {
			return
		}
		a.invalidateSettingsWindow()
		return
	}
	a.settingRow = index
	a.hotkeySettings.SetFocused(false)
	// Pointer-selected rows are already visible; moving the viewport here would invalidate popup anchors captured by the same click.
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
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
			util.Go(a.lifecycleCtx, "reload usage stats", func() {
				a.reloadUsageStats(next.value)
			})
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
	a.beginSettingSave()
	a.invalidateSettingsWindow()
	util.Go(a.lifecycleCtx, "save setting choice", func() {
		a.saveSetting(item, next)
	})
}

// startBuiltInSettingEdit gives a core-backed text value shared editor and native IME ownership.
func (a *App) startBuiltInSettingEdit(item settingItem, caret int) {
	if !item.text {
		return
	}
	if a.settingSaving {
		return
	}
	if a.generalSettings.EditKey() != item.key || a.generalSettings.Editor() == nil {
		if !a.generalSettings.StartEdit(item.key, item.value, caret) {
			return
		}
	} else if caret >= 0 {
		a.generalSettings.Editor().SetCaret(caret)
	}
	a.settingNote = "Editing " + item.title + " · Enter saves · Esc cancels"
	a.updateSettingsTextInput(true)
	a.invalidateSettingsWindow()
}

// cancelBuiltInSettingEdit discards an unsaved text value without mutating the loaded settings snapshot.
func (a *App) cancelBuiltInSettingEdit() {
	a.generalSettings.EndEdit()
	a.settingNote = ""
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

// submitBuiltInSettingEdit persists the active text row through the shared general settings service.
func (a *App) submitBuiltInSettingEdit() {
	snapshot := a.settingsSnapshot()
	if snapshot.general.EditKey == "" || snapshot.saving {
		return
	}
	items := settingItemsForSnapshot(snapshot)
	index := -1
	for candidate, item := range items {
		if item.key == snapshot.general.EditKey && item.text {
			index = candidate
			break
		}
	}
	if index < 0 {
		a.cancelBuiltInSettingEdit()
		return
	}
	item := items[index]
	value := snapshot.general.Editing.Text
	a.beginSettingSave()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	util.Go(a.lifecycleCtx, "save setting text value", func() {
		a.saveSetting(item, settingChoice{value: value, label: value})
	})
}

// onBuiltInSettingsEditorKey keeps text editing separate from rail and choice navigation.
func (a *App) onBuiltInSettingsEditorKey(event woxui.KeyEvent) bool {
	active := a.settingsOpen && a.generalSettings.EditKey() != "" && a.generalSettings.Editor() != nil
	saving := a.settingSaving
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
	return false
}

// setBuiltInSettingEditValue keeps only the committed business value in launcher state.
func (a *App) setBuiltInSettingEditValue(item settingItem, value string) {
	if a.settingsOpen && a.generalSettings.EditKey() == item.key {
		a.generalSettings.SetEditText(item.key, value)
	}
	a.invalidateSettingsWindow()
}

// onBuiltInSettingsTextInput commits native text and IME events into the active settings editor.
func (a *App) onBuiltInSettingsTextInput(_ woxui.TextInputEvent) bool {
	choicePickerOpen := a.generalSettings.ChoicePicker() != nil
	if choicePickerOpen {
		return true
	}
	active := a.settingsOpen && !a.settingSaving && a.generalSettings.EditKey() != "" && a.generalSettings.Editor() != nil
	return active
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
		a.settingNote = "Could not select " + item.title + ": " + err.Error()
		a.invalidateSettingsWindow()
		return
	}
	if path == "" {
		return
	}
	a.startBuiltInSettingEdit(item, -1)
	a.generalSettings.SetEditText(item.key, path)
	a.invalidateSettingsWindow()
}

func (a *App) beginSettingSave() {
	a.settingSaving = true
	a.settingNote = ""
}

func (a *App) saveSetting(item settingItem, choice settingChoice) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	err := a.services.UpdateGeneralSetting(ctx, a.sessionID, item.key, choice.value)
	if err == nil && item.key == "CloudSyncServerUrl" {
		err = a.services.LogoutAccount(ctx, a.sessionID)
	}
	cancel()
	if err == nil {
		err = a.reloadSettings()
	}
	if err == nil && item.key == "LangCode" {
		err = a.reloadTranslations()
	}
	restoreTextInput := false
	refreshGlance := false
	_ = a.runOnUI("apply general setting save", func() {
		a.settingSaving = false
		if a.generalSettings.EditKey() == item.key {
			if err == nil {
				a.generalSettings.EndEdit()
			} else if item.text {
				a.generalSettings.Editor().SetText(choice.value, false)
				restoreTextInput = true
			}
		}
		if err != nil {
			a.settingNote = "Could not save " + item.title + ": " + err.Error()
		} else {
			a.settingNote = ""
		}
		if err == nil && (item.key == "EnableGlance" || item.key == "PrimaryGlance") {
			a.stopGlanceLocked(true)
			refreshGlance = a.glanceEligibleLocked()
		}
		a.updateSettingsTextInput(restoreTextInput)
		a.invalidateSettingsWindow()
	})
	if refreshGlance {
		util.Go(a.lifecycleCtx, "refresh glance after settings change", func() {
			a.refreshGlance("settingsChanged", "", nil)
		})
	}
	if err == nil && (item.key == "CustomPythonPath" || item.key == "CustomNodejsPath") {
		util.Go(a.lifecycleCtx, "reload runtime statuses", a.reloadRuntimeStatuses)
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

// settingItemsForSnapshot adds page-local controls without storing them in the core settings snapshot.
func settingItemsForSnapshot(snapshot settingsSnapshot) []settingItem {
	if snapshot.tab == "usage" {
		return []settingItem{{
			key: "UsagePeriod", title: "Reporting period", value: snapshot.usage.Period,
			choices: []settingChoice{{"7d", "7 days"}, {"30d", "30 days"}, {"365d", "365 days"}, {"all", "All time"}},
		}}
	}
	if snapshot.tab == "ai" || snapshot.tab == "data" || snapshot.tab == "cloud" || snapshot.tab == "plugins" || snapshot.tab == "theme" || snapshot.tab == "about" {
		return nil
	}
	items := settingItems(snapshot.tab, snapshot.general.Data)
	if snapshot.tab == "updates" {
		for index := range items {
			if items[index].key == "ReleaseChannel" {
				items[index].trailers = updateChannelVersionTrailers(snapshot.update.ChannelVersions)
				break
			}
		}
	}
	if snapshot.tab == "general" && len(snapshot.general.Languages) > 0 {
		for index := range items {
			if items[index].key == "LangCode" {
				items[index].choices = append([]settingChoice(nil), snapshot.general.Languages...)
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
	appearance := snapshot.appearance
	current := snapshot.general.Data.PrimaryGlance
	currentValue := glanceRefJSON(current)
	choices := make([]settingChoice, 0, len(appearance.GlanceCatalog)+1)
	trailers := make(map[string]string, len(appearance.GlanceCatalog))
	icons := make(map[string]woxImage, len(appearance.GlanceCatalog))
	found := false
	for _, glance := range appearance.GlanceCatalog {
		value := glanceRefJSON(glance.Ref)
		label := glance.Name
		if strings.TrimSpace(label) == "" {
			label = glance.Ref.GlanceID
		}
		choices = append(choices, settingChoice{value: value, label: label})
		icons[value] = glance.Icon
		if glance.Preview != nil {
			trailers[value] = glance.Preview.Text
			if glance.Preview.Icon.ImageData != "" {
				icons[value] = glance.Preview.Icon
			}
		}
		if glance.Ref == current {
			found = true
		}
	}
	if !found && current.PluginID != "" && current.GlanceID != "" {
		choices = append([]settingChoice{{value: currentValue, label: current.GlanceID}}, choices...)
	}
	description := "Select the status shown in the global query box"
	if appearance.GlanceCatalogLoading {
		description = "Loading available Glance providers…"
	} else if appearance.GlanceCatalogError != "" {
		description = "Could not load Glance providers: " + appearance.GlanceCatalogError
	}
	return settingItem{key: "PrimaryGlance", title: "Primary glance", description: description, value: currentValue, choices: choices, trailers: trailers, icons: icons}
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
			{key: "EnablePrivacyMode", title: "Private mode", description: "Clear local data after exit while retaining non-sensitive settings", value: boolValue(data.EnablePrivacyMode), choices: boolChoices},
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
