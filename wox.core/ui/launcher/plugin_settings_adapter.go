package launcher

import (
	"fmt"
	"sort"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildPluginSettingsPage maps plugin state into the shared catalog and detail views.
func (a *App) buildPluginSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	innerHeight := max(float32(0), height-24)
	listWidth := min(float32(300), max(float32(250), innerWidth*0.31))
	detailWidth := max(float32(0), innerWidth-listWidth-1)
	return launcherview.PluginSettingsPage(launcherview.PluginSettingsPageProps{
		Width:       width,
		Height:      height,
		List:        a.pluginListProps(snapshot, listWidth, innerHeight),
		Detail:      a.pluginDetailProps(snapshot, detailWidth, innerHeight),
		FilterPanel: a.pluginFilterPanelProps(snapshot),
		Theme:       snapshot.palette.componentTheme(),
	})
}

// pluginListProps resolves localized catalog labels, images, selection, and callbacks.
func (a *App) pluginListProps(snapshot settingsSnapshot, width, height float32) launcherview.PluginListProps {
	iconTint := snapshot.palette.resultSubtitle
	props := launcherview.PluginListProps{
		Width: width, Height: height, Scroll: snapshot.pluginListScroll,
		Placeholder:  fmt.Sprintf(a.translate("i18n:ui_search_plugins"), len(snapshot.plugins)),
		Search:       snapshot.pluginSearch,
		Focused:      snapshot.pluginSearchFocused,
		Window:       a.settingsNativeWindow(),
		FilterIcon:   a.imageForTint(settingControlIconSource("filter"), &iconTint, 18),
		RefreshIcon:  a.imageForTint(settingControlIconSource("refresh"), &iconTint, 18),
		FilterActive: snapshot.pluginFilters.applied(snapshot.pluginsStore),
		Refreshing:   snapshot.pluginsLoading,
		EmptyLabel:   a.translate("i18n:ui_setting_plugin_empty_data"), Theme: snapshot.palette.componentTheme(),
		OnViewport: a.setPluginListViewport, OnScroll: a.scrollPluginList, OnCaret: a.focusPluginSearch, OnClear: a.clearPluginSearch,
		OnSearchKey: a.onPluginSearchKey, OnSearchTextInput: a.onPluginSearchTextInput, OnSearchFocusChange: a.setPluginSearchFocused, OnSetSearchValue: a.setPluginSearchValue,
		OnFilter: a.togglePluginFilterPanel, OnRefresh: a.refreshPluginCatalog,
	}
	if snapshot.pluginsLoading && len(snapshot.plugins) == 0 {
		props.Message = a.translate("i18n:ui_cloud_sync_plugin_exclusions_loading")
		return props
	}
	if snapshot.pluginsError != "" && len(snapshot.plugins) == 0 {
		props.Message = snapshot.pluginsError
		props.MessageError = true
		return props
	}

	filtered := filterPlugins(snapshot.plugins, snapshot.pluginSearch.Text, snapshot.pluginFilters, snapshot.pluginsStore)
	props.Placeholder = fmt.Sprintf(a.translate("i18n:ui_search_plugins"), len(filtered))
	props.Items = make([]launcherview.PluginListItem, 0, len(filtered))
	for visibleIndex, entry := range filtered {
		index := entry.index
		plugin := entry.plugin
		status := strings.TrimSpace(plugin.Version + "  " + plugin.Author)
		if plugin.IsUpgradable {
			status = a.translate("i18n:ui_update") + "  " + status
		} else if plugin.IsDisable {
			status = a.translate("i18n:ui_disabled") + "  " + status
		}
		badge := ""
		if plugin.IsSystem {
			badge = a.translate("i18n:ui_setting_plugin_system_tag")
		} else if plugin.IsDev {
			badge = a.translate("i18n:ui_plugin_dev_tag")
		} else if strings.EqualFold(plugin.Runtime, "script") {
			badge = a.translate("i18n:ui_setting_plugin_script_tag")
		}
		props.Items = append(props.Items, launcherview.PluginListItem{
			ID: plugin.ID, Name: plugin.Name, Status: status, Badge: badge,
			Icon: a.imageFor(plugin.Icon), FallbackColor: resultColors[visibleIndex%len(resultColors)], Selected: index == snapshot.pluginSelected,
			OnSelect: func() { a.selectPlugin(index) },
		})
	}
	return props
}

// pluginDetailProps maps the selected plugin into an empty, store, or editable detail view.
func (a *App) pluginDetailProps(snapshot settingsSnapshot, width, height float32) launcherview.PluginDetailProps {
	props := launcherview.PluginDetailProps{
		Width: width, Height: height, EmptyLabel: a.translate("i18n:ui_setting_plugin_empty_data"), Theme: snapshot.palette.componentTheme(),
	}
	if snapshot.pluginSelected < 0 || snapshot.pluginSelected >= len(snapshot.plugins) {
		return props
	}
	plugin := snapshot.plugins[snapshot.pluginSelected]
	if snapshot.pluginForm == nil {
		props.Store = a.pluginStoreDetailProps(snapshot, plugin, width)
		return props
	}

	form := snapshot.pluginForm
	detailTab := snapshot.pluginDetailTab
	if detailTab == "" {
		detailTab = "settings"
	}
	editor := &launcherview.PluginEditorProps{
		Header:      a.pluginHeaderProps(snapshot, plugin),
		ActiveTab:   detailTab,
		Tabs:        a.pluginDetailTabs(),
		OnSelectTab: a.selectPluginDetailTab,
	}
	if detailTab != "settings" {
		metadata := a.pluginMetadataProps(plugin, detailTab)
		editor.Metadata = &metadata
		props.Editor = editor
		return props
	}

	innerWidth := max(float32(0), width-48)
	callbacks := formFieldCallbacks{
		idPrefix:  "plugin-settings",
		focus:     a.focusPluginFormField,
		change:    a.changePluginFormChoice,
		setCaret:  a.setPluginFormCaret,
		openTable: a.openPluginFormTable,
		openModel: a.openPluginModelManager,
		recordKey: a.recordPluginFormHotkey,
	}
	rows := make([]woxwidget.Widget, 0, len(form.definitions))
	for index, definition := range form.definitions {
		rows = append(rows, a.buildFormField(form.formFieldsSnapshot, callbacks, snapshot.palette, index, definition, innerWidth, formDefinitionHeight(definition, form.values)))
	}
	editor.Form = &launcherview.PluginFormProps{
		Rows: rows, ContentHeight: formDefinitionsContentHeight(form.definitions, form.values), Scroll: form.scroll,
		OnViewport: a.setPluginFormViewport, OnScroll: a.scrollPluginForm,
	}
	editor.Status = form.status
	editor.StatusError = form.statusError
	if snapshot.pluginOperationError != "" {
		editor.Status = snapshot.pluginOperationError
		editor.StatusError = true
	}
	editor.SaveLabel = a.translate("i18n:ui_save")
	if form.saving {
		editor.SaveLabel += "…"
	}
	editor.SaveHighlight = form.dirty && !form.saving
	editor.OnSave = a.submitPluginSettings
	props.Editor = editor
	return props
}

func (a *App) pluginHeaderProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin) launcherview.PluginHeaderProps {
	return launcherview.PluginHeaderProps{
		Title: strings.TrimSpace(plugin.Name + "  " + plugin.Version), Author: plugin.Author,
		Icon: a.imageFor(plugin.Icon), FallbackColor: resultColors[snapshot.pluginSelected%len(resultColors)],
		MetadataActions: a.pluginMetadataActions(plugin), Management: a.pluginManagementActions(snapshot, plugin),
	}
}

func (a *App) pluginDetailTabs() []launcherview.PluginTab {
	return []launcherview.PluginTab{
		a.resolvedPluginTab("settings", a.translate("i18n:ui_plugin_tab_settings")),
		a.resolvedPluginTab("description", a.translate("i18n:ui_plugin_tab_description")),
		a.resolvedPluginTab("keywords", a.translate("i18n:ui_plugin_tab_trigger_keywords")),
		a.resolvedPluginTab("commands", a.translate("i18n:ui_plugin_tab_commands")),
		a.resolvedPluginTab("privacy", a.translate("i18n:ui_plugin_tab_privacy")),
	}
}

func (a *App) pluginStoreDetailTabs() []launcherview.PluginTab {
	return []launcherview.PluginTab{
		a.resolvedPluginTab("description", a.translate("i18n:ui_plugin_tab_description")),
		a.resolvedPluginTab("keywords", a.translate("i18n:ui_plugin_tab_trigger_keywords")),
		a.resolvedPluginTab("commands", a.translate("i18n:ui_plugin_tab_commands")),
		a.resolvedPluginTab("privacy", a.translate("i18n:ui_plugin_tab_privacy")),
	}
}

// resolvedPluginTab sizes localized labels like Flutter's scrollable content-width tabs.
func (a *App) resolvedPluginTab(id, label string) launcherview.PluginTab {
	width := float32(72)
	if window := a.settingsNativeWindow(); window != nil {
		if metrics, err := window.MeasureText(label, woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}); err == nil {
			width = max(width, metrics.Size.Width+32)
		}
	}
	return launcherview.PluginTab{ID: id, Label: label, Width: width}
}

// pluginMetadataProps restores Flutter's non-editing plugin detail tabs from core metadata.
func (a *App) pluginMetadataProps(plugin pluginSettingsPlugin, tab string) launcherview.PluginMetadataProps {
	props := launcherview.PluginMetadataProps{}
	switch tab {
	case "description":
		props.DescriptionOnly = true
		props.Description = plugin.Description
	case "keywords":
		if len(plugin.TriggerKeywords) == 0 {
			props.EmptyLabel = a.translate("i18n:ui_plugin_no_trigger_keywords")
			break
		}
		for _, keyword := range plugin.TriggerKeywords {
			props.Items = append(props.Items, launcherview.PluginMetadataItem{Title: keyword, Description: a.translate("i18n:ui_plugin_trigger_keywords_tip")})
		}
	case "commands":
		if len(plugin.Commands) == 0 {
			props.EmptyLabel = a.translate("i18n:ui_plugin_no_commands")
			break
		}
		props.Items = append(props.Items, launcherview.PluginMetadataItem{Title: a.translate("i18n:ui_plugin_command_name_column"), Description: a.translate("i18n:ui_plugin_command_desc_column")})
		for _, command := range plugin.Commands {
			props.Items = append(props.Items, launcherview.PluginMetadataItem{Title: command.Command, Description: command.Description})
		}
	case "privacy":
		accesses := pluginPrivacyAccesses(plugin.Features)
		if len(accesses) == 0 {
			props.EmptyLabel = a.translate("i18n:ui_plugin_no_data_access")
			break
		}
		props.Header = a.translate("i18n:ui_plugin_data_access_title")
		for _, access := range accesses {
			props.Items = append(props.Items, launcherview.PluginMetadataItem{Title: pluginPrivacyTitle(a, access), Description: pluginPrivacyDescription(a, access)})
		}
	}
	return props
}

func pluginPrivacyAccesses(features []pluginFeature) []string {
	accessSet := map[string]bool{}
	for _, feature := range features {
		if feature.Name == "queryEnv" {
			for key, value := range feature.Params {
				enabled, ok := value.(bool)
				if ok && enabled {
					accessSet[key] = true
					continue
				}
				if text, ok := value.(string); ok && strings.EqualFold(strings.TrimSpace(text), "true") {
					accessSet[key] = true
				}
			}
		}
		if feature.Name == "llm" || feature.Name == "ai" {
			accessSet["llm"] = true
		}
	}
	order := []string{"requireActiveWindowName", "requireActiveWindowPid", "requireActiveWindowId", "requireActiveWindowIcon", "requireActiveWindowIsOpenSaveDialog", "requireActiveBrowserUrl", "llm"}
	accesses := make([]string, 0, len(accessSet))
	for _, access := range order {
		if accessSet[access] {
			accesses = append(accesses, access)
			delete(accessSet, access)
		}
	}
	unknown := make([]string, 0, len(accessSet))
	for access := range accessSet {
		unknown = append(unknown, access)
	}
	sort.Strings(unknown)
	return append(accesses, unknown...)
}

func pluginPrivacyDescription(a *App, access string) string {
	switch access {
	case "requireActiveWindowName":
		return a.translate("i18n:ui_plugin_privacy_window_name_desc")
	case "requireActiveWindowPid":
		return a.translate("i18n:ui_plugin_privacy_window_pid_desc")
	case "requireActiveWindowId":
		return a.translate("i18n:ui_plugin_privacy_window_id_desc")
	case "requireActiveWindowIcon":
		return a.translate("i18n:ui_plugin_privacy_window_icon_desc")
	case "requireActiveWindowIsOpenSaveDialog":
		return a.translate("i18n:ui_plugin_privacy_open_save_dialog_desc")
	case "requireActiveBrowserUrl":
		return a.translate("i18n:ui_plugin_privacy_browser_url_desc")
	case "llm":
		return a.translate("i18n:ui_plugin_privacy_llm_desc")
	default:
		return ""
	}
}

func pluginPrivacyTitle(a *App, access string) string {
	switch access {
	case "requireActiveWindowName":
		return a.translate("i18n:ui_plugin_privacy_window_name")
	case "requireActiveWindowPid":
		return a.translate("i18n:ui_plugin_privacy_window_pid")
	case "requireActiveWindowId":
		return a.translate("i18n:ui_plugin_privacy_window_id")
	case "requireActiveWindowIcon":
		return a.translate("i18n:ui_plugin_privacy_window_icon")
	case "requireActiveWindowIsOpenSaveDialog":
		return a.translate("i18n:ui_plugin_privacy_open_save_dialog")
	case "requireActiveBrowserUrl":
		return a.translate("i18n:ui_plugin_privacy_browser_url")
	case "llm":
		return a.translate("i18n:ui_plugin_privacy_llm")
	default:
		return access
	}
}

func (a *App) pluginStoreDetailProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin, width float32) *launcherview.PluginStoreDetailProps {
	activeTab := snapshot.pluginDetailTab
	if activeTab == "" || activeTab == "settings" {
		activeTab = "description"
	}
	websiteLabel := ""
	websiteChipLabel := ""
	var onWebsite func()
	var externalIcon *woxui.Image
	var websiteIcon *woxui.Image
	if strings.TrimSpace(plugin.Website) != "" {
		websiteLabel = a.translate("i18n:ui_plugin_website")
		websiteChipLabel = websiteLabel + " ↗"
		onWebsite = a.openSelectedPluginWebsite
		iconTint := snapshot.palette.resultTitle
		externalIcon = a.imageForTint(settingControlIconSource("external"), &iconTint, 16)
		if strings.Contains(strings.ToLower(plugin.Website), "github.com") {
			websiteChipLabel = "GitHub ↗"
			websiteIcon = a.imageForTint(pluginMetadataIconSource("github"), &iconTint, 18)
		}
	}
	runtimeLabel := pluginRuntimeLabel(plugin.Runtime)
	var runtimeIcon *woxui.Image
	if source := pluginMetadataIconSource(strings.ToLower(plugin.Runtime)); source.ImageData != "" {
		runtimeIcon = a.imageFor(source)
	}
	var screenshot *woxui.Image
	var screenshotHeight float32
	var onScreenshot func()
	if activeTab == "description" && len(plugin.ScreenshotURLs) > 0 {
		source := woxImage{ImageType: "url", ImageData: plugin.ScreenshotURLs[0]}
		screenshotWidth := max(float32(1), width-48)
		requestSize := int(min(float32(2048), max(float32(512), screenshotWidth*2)))
		screenshot = a.imageForSize(source, requestSize)
		if screenshot != nil && screenshot.Width > 0 {
			screenshotHeight = screenshotWidth * float32(screenshot.Height) / float32(screenshot.Width)
		}
		onScreenshot = func() { a.openPreviewImageOverlay(source) }
	}
	return &launcherview.PluginStoreDetailProps{
		Name: plugin.Name, Version: plugin.Version, Author: plugin.Author, Description: plugin.Description, Runtime: runtimeLabel,
		WebsiteLabel: websiteLabel, WebsiteChipLabel: websiteChipLabel,
		Icon: a.imageFor(plugin.Icon), ExternalIcon: externalIcon, RuntimeIcon: runtimeIcon, WebsiteIcon: websiteIcon,
		FallbackColor: resultColors[snapshot.pluginSelected%len(resultColors)], Management: a.pluginManagementActions(snapshot, plugin),
		ActiveTab: activeTab, Tabs: a.pluginStoreDetailTabs(), Metadata: a.pluginMetadataProps(plugin, activeTab),
		Screenshot: screenshot, ScreenshotHeight: screenshotHeight, Error: snapshot.pluginOperationError, OnWebsite: onWebsite, OnScreenshot: onScreenshot, OnSelectTab: a.selectPluginDetailTab,
	}
}

func (a *App) pluginFilterPanelProps(snapshot settingsSnapshot) *launcherview.PluginFilterPanelProps {
	if !snapshot.pluginFilterOpen {
		return nil
	}
	filters := snapshot.pluginFilters
	options := make([]launcherview.PluginFilterOption, 0, 4)
	if snapshot.pluginsStore {
		options = append(options, launcherview.PluginFilterOption{ID: "uninstalled", Label: a.translate("i18n:ui_not_installed"), Value: filters.uninstalledOnly})
	} else {
		options = append(options,
			launcherview.PluginFilterOption{ID: "disabled", Label: a.translate("i18n:ui_plugin_filter_disabled_only"), Value: filters.disabledOnly},
			launcherview.PluginFilterOption{ID: "enabled", Label: a.translate("i18n:ui_plugin_filter_enabled_only"), Value: filters.enabledOnly},
			launcherview.PluginFilterOption{ID: "upgradable", Label: a.translate("i18n:ui_plugin_filter_upgradable"), Value: filters.upgradableOnly},
		)
	}
	options = append(options, launcherview.PluginFilterOption{ID: "third-party", Label: a.translate("i18n:ui_plugin_filter_third_party_only"), Value: filters.thirdPartyOnly})
	runtimes := []launcherview.PluginFilterOption{
		{ID: "runtime-nodejs", Label: a.translate("i18n:ui_runtime_name_nodejs"), Value: filters.runtimeNodeJSOnly},
		{ID: "runtime-python", Label: a.translate("i18n:ui_runtime_name_python"), Value: filters.runtimePythonOnly},
	}
	if snapshot.pluginsStore {
		runtimes = append(runtimes, launcherview.PluginFilterOption{ID: "runtime-script", Label: a.translate("i18n:ui_runtime_name_script"), Value: filters.runtimeScriptOnly})
	} else {
		runtimes = append(runtimes,
			launcherview.PluginFilterOption{ID: "runtime-script-nodejs", Label: a.translate("i18n:plugin_wpm_script_template_nodejs"), Value: filters.runtimeScriptNodeJSOnly},
			launcherview.PluginFilterOption{ID: "runtime-script-python", Label: a.translate("i18n:plugin_wpm_script_template_python"), Value: filters.runtimeScriptPythonOnly},
		)
	}
	return &launcherview.PluginFilterPanelProps{
		Width: 360, Title: a.translate("i18n:ui_filter_placeholder"), RuntimeTitle: a.translate("i18n:ui_runtime_status"),
		Options: options, Runtimes: runtimes, Theme: snapshot.palette.componentTheme(), OnToggle: a.togglePluginFilter, OnDismiss: a.closePluginFilterPanel,
	}
}

// pluginRuntimeLabel normalizes manifest runtime names for the compact metadata chip.
func pluginRuntimeLabel(runtime string) string {
	switch strings.ToLower(strings.TrimSpace(runtime)) {
	case "nodejs":
		return "NodeJS"
	case "python":
		return "Python"
	case "script":
		return "Script"
	case "go":
		return "Go"
	default:
		return runtime
	}
}

// pluginMetadataIconSource reuses the brand assets from the Flutter plugin detail chips.
func pluginMetadataIconSource(kind string) woxImage {
	switch kind {
	case "nodejs":
		return woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><path fill="#8bc34a" d="M16 20.003v2h4a2 2 0 0 0 2-2v-2a2 2 0 0 0-2-2h-2v-2h4v-2h-4a2 2 0 0 0-2 2v2a2 2 0 0 0 2 2h2v2Z"/><path fill="#8bc34a" d="m16 3.003l-12 7v14l4 2h6v-13.5a.5.5 0 0 0-.5-.5h-1a.5.5 0 0 0-.5.5v11.5H8l-2-1.034V11.15l10-5.833l10 5.833v11.703l-10 5.833l-1.745-1.022L13 29.253l3 1.75l12-7v-14Z"/></svg>`}
	case "python":
		return woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="#0288d1" d="M9.86 2A2.86 2.86 0 0 0 7 4.86v1.68h4.29c.39 0 .71.57.71.96H4.86A2.86 2.86 0 0 0 2 10.36v3.781a2.86 2.86 0 0 0 2.86 2.86h1.18v-2.68a2.85 2.85 0 0 1 2.85-2.86h5.25c1.58 0 2.86-1.271 2.86-2.851V4.86A2.86 2.86 0 0 0 14.14 2zm-.72 1.61c.4 0 .72.12.72.71s-.32.891-.72.891c-.39 0-.71-.3-.71-.89s.32-.711.71-.711"/><path fill="#fdd835" d="M17.959 7v2.68a2.85 2.85 0 0 1-2.85 2.859H9.86A2.85 2.85 0 0 0 7 15.389v3.75a2.86 2.86 0 0 0 2.86 2.86h4.28A2.86 2.86 0 0 0 17 19.14v-1.68h-4.291c-.39 0-.709-.57-.709-.96h7.14A2.86 2.86 0 0 0 22 13.64V9.86A2.86 2.86 0 0 0 19.14 7zM8.32 11.513l-.004.004l.038-.004zm6.54 7.276c.39 0 .71.3.71.89a.71.71 0 0 1-.71.71c-.4 0-.72-.12-.72-.71s.32-.89.72-.89"/></svg>`}
	case "github":
		return woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 98 96"><path fill="#24292f" fill-rule="evenodd" clip-rule="evenodd" d="M48.9 0C21.9 0 0 22 0 49.1c0 21.7 14 40.1 33.4 46.6 2.4.5 3.3-1.1 3.3-2.4 0-1.2 0-4.2-.1-8.3-13.6 3-16.4-6.6-16.4-6.6-2.2-5.7-5.4-7.2-5.4-7.2-4.4-3 .3-3 .3-3 4.9.3 7.5 5.1 7.5 5.1 4.3 7.5 11.4 5.3 14.1 4.1.4-3.2 1.7-5.3 3.1-6.5-10.8-1.2-22.2-5.4-22.2-24.2 0-5.4 1.9-9.7 5-13.2-.5-1.2-2.2-6.2.5-13 0 0 4.1-1.3 13.4 5 3.9-1.1 8-1.6 12.2-1.6s8.3.6 12.2 1.6c9.3-6.3 13.4-5 13.4-5 2.7 6.8 1 11.8.5 13 3.1 3.5 5 7.8 5 13.2 0 18.8-11.4 22.9-22.3 24.1 1.8 1.5 3.3 4.5 3.3 9.1 0 6.5-.1 11.8-.1 13.4 0 1.3.9 2.9 3.4 2.4C84 89.2 98 70.8 98 49.1 97.8 22 75.9 0 48.9 0z"/></svg>`}
	default:
		return woxImage{}
	}
}

// pluginManagementActions shares install, upgrade, and uninstall actions between plugin details.
func (a *App) pluginManagementActions(snapshot settingsSnapshot, plugin pluginSettingsPlugin) []launcherview.PluginAction {
	busy := snapshot.pluginOperation != ""
	if !plugin.IsInstalled {
		return []launcherview.PluginAction{{
			ID: "plugin-install", Label: pluginOperationButtonLabel(snapshot, "install", plugin.ID, a.translate("i18n:ui_plugin_install")), Width: 104,
			Enabled: !busy, Primary: true, OnTap: func() { a.runPluginOperation("install") },
		}}
	}
	actions := make([]launcherview.PluginAction, 0, 3)
	if plugin.IsUpgradable {
		actions = append(actions, launcherview.PluginAction{ID: "plugin-upgrade", Label: pluginOperationButtonLabel(snapshot, "upgrade", plugin.ID, a.translate("i18n:ui_update")), Width: 104, Enabled: !busy, Primary: true, OnTap: func() { a.runPluginOperation("upgrade") }})
	}
	if plugin.IsDisable {
		actions = append(actions, launcherview.PluginAction{ID: "plugin-enable", Label: pluginOperationButtonLabel(snapshot, "enable", plugin.ID, a.translate("i18n:ui_plugin_enable")), Width: 96, Enabled: !busy, OnTap: func() { a.runPluginOperation("enable") }})
	} else {
		actions = append(actions, launcherview.PluginAction{ID: "plugin-disable", Label: pluginOperationButtonLabel(snapshot, "disable", plugin.ID, a.translate("i18n:ui_plugin_disable")), Width: 96, Enabled: !busy, OnTap: func() { a.runPluginOperation("disable") }})
	}
	if !plugin.IsSystem {
		label := a.translate("i18n:ui_plugin_uninstall")
		if snapshot.pluginUninstallArmed == plugin.ID {
			label = a.translate("i18n:ui_cloud_sync_confirm") + " " + label
		}
		actions = append(actions, launcherview.PluginAction{ID: "plugin-uninstall", Label: pluginOperationButtonLabel(snapshot, "uninstall", plugin.ID, label), Width: 124, Enabled: !busy, OnTap: func() { a.runPluginOperation("uninstall") }})
	}
	return actions
}

// pluginMetadataActions exposes browser and folder actions without platform-specific widgets.
func (a *App) pluginMetadataActions(plugin pluginSettingsPlugin) []launcherview.PluginAction {
	actions := make([]launcherview.PluginAction, 0, 2)
	if strings.TrimSpace(plugin.Website) != "" {
		actions = append(actions, launcherview.PluginAction{ID: "plugin-website", Label: a.translate("i18n:ui_plugin_website"), Width: 104, Enabled: true, OnTap: a.openSelectedPluginWebsite})
	}
	if plugin.IsInstalled && strings.TrimSpace(plugin.PluginDirectory) != "" {
		actions = append(actions, launcherview.PluginAction{ID: "plugin-directory", Label: a.translate("i18n:ui_plugin_open_directory"), Width: 112, Enabled: true, OnTap: a.openSelectedPluginDirectory})
	}
	return actions
}

func pluginOperationButtonLabel(snapshot settingsSnapshot, kind, pluginID, idle string) string {
	if snapshot.pluginOperation == kind+":"+pluginID {
		return idle + "…"
	}
	return idle
}
