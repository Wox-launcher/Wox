package launcher

import (
	"fmt"
	"sort"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildPluginSettingsPage maps plugin state into the shared catalog and detail views.
func (a *App) buildPluginSettingsPage(snapshot settingsSnapshot, width, height, imageScale float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-40)
	innerHeight := max(float32(0), height-40)
	listWidth := min(float32(250), max(float32(220), innerWidth*0.30))
	detailWidth := max(float32(0), innerWidth-listWidth-21)
	return launcherview.PluginSettingsPage(launcherview.PluginSettingsPageProps{
		Width:       width,
		Height:      height,
		List:        a.pluginListProps(snapshot, listWidth, innerHeight, imageScale),
		Detail:      a.pluginDetailProps(snapshot, detailWidth, innerHeight, imageScale),
		FilterPanel: a.pluginFilterPanelProps(snapshot),
		Theme:       snapshot.palette.componentTheme(),
	})
}

// pluginListProps resolves localized catalog labels, images, selection, and callbacks.
func (a *App) pluginListProps(snapshot settingsSnapshot, width, height, imageScale float32) launcherview.PluginListProps {
	plugins := snapshot.plugins
	iconTint := snapshot.palette.resultSubtitle
	selectedIconTint := snapshot.palette.selectedTitle
	installedTint := woxui.Color{R: 56, G: 176, B: 92, A: 255}
	props := launcherview.PluginListProps{
		Width: width, Height: height,
		Placeholder:           fmt.Sprintf(a.translate("i18n:ui_search_plugins"), len(plugins.Plugins)),
		Search:                plugins.PluginSearch,
		Focused:               plugins.PluginSearchFocused,
		Window:                a.settingsNativeWindow(),
		FilterIcon:            a.imageForTint(settingControlIconSource("filter"), &iconTint, physicalImageSize(16, imageScale)),
		RefreshIcon:           a.imageForTint(settingControlIconSource("refresh"), &iconTint, physicalImageSize(16, imageScale)),
		InstalledIcon:         a.imageForTint(settingControlIconSource("check-circle"), &installedTint, physicalImageSize(20, imageScale)),
		InstalledSelectedIcon: a.imageForTint(settingControlIconSource("check-circle"), &selectedIconTint, physicalImageSize(20, imageScale)),
		FilterLabel:           a.translate("i18n:ui_filter_placeholder"),
		RefreshLabel:          a.translate("i18n:ui_refresh"),
		FilterActive:          plugins.PluginFilters.applied(plugins.PluginsStore),
		Refreshing:            plugins.PluginsLoading,
		Theme:                 snapshot.palette.componentTheme(),
		OnClear:               a.clearPluginSearch,
		OnSearchKey:           a.onPluginSearchKey, OnSearchFocusChange: a.setPluginSearchFocused,
		OnSearchChanged: func(value string) { _ = a.setPluginSearchValue(value) }, OnSetSearchValue: a.setPluginSearchValue,
		OnFilter: a.togglePluginFilterPanel, OnRefresh: a.refreshPluginCatalog,
	}
	if plugins.PluginsLoading && len(plugins.Plugins) == 0 {
		props.Message = a.translate("i18n:ui_cloud_sync_plugin_exclusions_loading")
		return props
	}
	if plugins.PluginsError != "" && len(plugins.Plugins) == 0 {
		props.Message = plugins.PluginsError
		props.MessageError = true
		return props
	}

	filtered := filterPlugins(plugins.Plugins, plugins.PluginSearch.Text, plugins.PluginFilters, plugins.PluginsStore)
	props.Placeholder = fmt.Sprintf(a.translate("i18n:ui_search_plugins"), len(filtered))
	a.applyPluginCatalogEmptyState(&props, plugins, filtered, iconTint, imageScale)
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
			ID: plugin.ID, Name: plugin.Name, Status: status, Badge: badge, ShowInstalledIcon: plugins.PluginsStore && plugin.IsInstalled,
			Icon: a.imageFor(plugin.Icon), FallbackColor: resultColors[visibleIndex%len(resultColors)], Selected: index == plugins.PluginSelected,
			Highlighted: snapshot.highlight == "plugin:"+plugin.ID,
			OnSelect:    func() { a.selectPlugin(index) },
		})
	}
	return props
}

func (a *App) applyPluginCatalogEmptyState(props *launcherview.PluginListProps, plugins pluginSettingsSnapshot, filtered []filteredPlugin, iconTint woxui.Color, imageScale float32) {
	if len(filtered) > 0 {
		return
	}
	emptyIconTint := iconTint
	emptyIconTint.A = 160
	props.EmptyIcon = a.imageForTint(settingControlIconSource("search"), &emptyIconTint, physicalImageSize(24, imageScale))
	if len(plugins.Plugins) > 0 {
		props.EmptyTitle = a.translate("i18n:ui_no_matches")
		props.EmptyDescription = a.translate("i18n:ui_setting_catalog_search_empty_subtitle")
		return
	}
	props.EmptyTitle = a.translate("i18n:ui_setting_plugin_empty_data")
	props.EmptyDescription = a.translate("i18n:ui_setting_plugin_empty_subtitle")
}

// pluginDetailProps maps the selected plugin into an empty, store, or editable detail view.
func (a *App) pluginDetailProps(snapshot settingsSnapshot, width, height, imageScale float32) launcherview.PluginDetailProps {
	plugins := snapshot.plugins
	emptyIconTint := snapshot.palette.resultSubtitle
	emptyIconTint.A = 160
	props := launcherview.PluginDetailProps{
		Width: width, Height: height, EmptyLabel: a.translate("i18n:ui_setting_plugin_empty_data"),
		EmptyTitle: a.translate("i18n:ui_setting_plugin_empty_data"), EmptyDescription: a.translate("i18n:ui_setting_plugin_empty_subtitle"),
		EmptyIcon: a.imageForTint(settingControlIconSource("search"), &emptyIconTint, physicalImageSize(24, imageScale)), Window: a.settingsNativeWindow(),
		Theme: snapshot.palette.componentTheme(),
	}
	if plugins.PluginSelected < 0 || plugins.PluginSelected >= len(plugins.Plugins) {
		return props
	}
	plugin := plugins.Plugins[plugins.PluginSelected]
	if plugins.PluginForm == nil {
		props.Store = a.pluginStoreDetailProps(snapshot, plugin, width, imageScale)
		return props
	}

	form := plugins.PluginForm
	detailTab := plugins.PluginDetailTab
	if detailTab == "" {
		detailTab = "settings"
	}
	editor := &launcherview.PluginEditorProps{
		Header:      a.pluginHeaderProps(snapshot, plugin, imageScale),
		ActiveTab:   detailTab,
		Tabs:        a.pluginDetailTabs(),
		OnSelectTab: a.selectPluginDetailTab,
	}
	callbacks := formFieldCallbacks{
		idPrefix:          "plugin-settings",
		labelWidth:        a.pluginFormLabelWidth(form.definitions[1:]),
		imageScale:        imageScale,
		focus:             a.focusPluginFormField,
		change:            a.changePluginFormChoice,
		setText:           a.setPluginFormText,
		onKey:             a.onPluginSettingsKey,
		openTable:         a.openPluginFormTable,
		openChoice:        a.openPluginFormChoice,
		openAIModelChoice: a.openPluginAIModelChoice,
		setAIModelName:    a.setPluginAIModelName,
		finishAIModelEdit: a.finishPluginAIModelEdit,
		openModel:         a.openPluginModelManager,
		recordKey:         a.recordPluginFormHotkey,
	}
	if detailTab == "description" {
		editor.DescriptionDetail = a.pluginStoreDetailProps(snapshot, plugin, max(float32(0), width-80), imageScale)
		props.Editor = editor
		return props
	}
	if detailTab == "keywords" {
		keywordDefinition := form.definitions[0]
		innerWidth := max(float32(0), width-32)
		keywordTable := a.formTableFieldProps(form.formFieldsSnapshot, callbacks, snapshot.palette, 0, keywordDefinition, innerWidth, 0)
		keywordTable.Title = ""
		for index := range keywordTable.Rows {
			if len(keywordTable.Rows[index].Cells) > 0 && keywordTable.Rows[index].Cells[0].Text == "*" {
				keywordTable.Rows[index].Cells[0].Text = a.translate("i18n:ui_plugin_trigger_keyword_global")
			}
		}
		accent := a.pluginDetailIntroAccent(snapshot.palette.background)
		editor.Form = a.pluginDetailIntroFormProps(snapshot, imageScale, a.translate("i18n:ui_plugin_trigger_keywords_tip"), []woxwidget.Widget{
			woxwidget.Keyed{Key: pluginSettingRowKey(0), Child: launcherview.FormTableField(keywordTable)},
		}, accent)
		editor.Form.KeepVisibleKey = pluginSettingKeepVisibleKey(form.formFieldsSnapshot, 0)
		props.Editor = editor
		return props
	}
	if detailTab == "commands" {
		innerWidth := max(float32(0), width-32)
		editor.Form = a.pluginCommandsFormProps(snapshot, plugin, innerWidth, imageScale, true)
		props.Editor = editor
		return props
	}
	if detailTab != "settings" {
		metadata := a.pluginMetadataProps(plugin, detailTab)
		editor.Metadata = &metadata
		props.Editor = editor
		return props
	}

	innerWidth := max(float32(0), width-32)
	settingDefinitions := form.definitions[1:]
	rows := make([]woxwidget.Widget, 0, len(settingDefinitions))
	for index, definition := range settingDefinitions {
		formIndex := index + 1
		field := a.buildFormField(form.formFieldsSnapshot, callbacks, snapshot.palette, formIndex, definition, innerWidth, 0)
		target := woxcomponent.WoxSettingTarget(woxcomponent.SettingTargetProps{
			Width: innerWidth, Highlighted: snapshot.highlight == "plugin-setting:"+plugin.ID+"\x00"+definition.Value.Key, Child: field, Theme: snapshot.palette.componentTheme(),
		})
		rows = append(rows, woxwidget.Keyed{Key: pluginSettingRowKey(formIndex), Child: target})
	}
	editor.Form = &launcherview.PluginFormProps{
		Rows: rows, KeepVisibleKey: pluginSettingKeepVisibleKey(form.formFieldsSnapshot, 1),
		EmptyTitle: a.translate("i18n:ui_plugin_no_settings"), EmptyDescription: a.translate("i18n:ui_plugin_no_settings_subtitle"),
	}
	props.Editor = editor
	return props
}

// pluginFormLabelWidth mirrors Flutter's shared measured label column for each plugin.
func (a *App) pluginFormLabelWidth(definitions []formDefinition) float32 {
	window := a.settingsNativeWindow()
	if window == nil {
		return 120
	}
	return a.measureFormLabelWidth(definitions, window, 0, 200)
}

func pluginSettingRowKey(index int) woxwidget.Key {
	return woxwidget.Key(fmt.Sprintf("plugin-setting-row-%d", index))
}

// pluginSettingKeepVisibleKey maps form focus to a measured row on the active plugin tab.
func pluginSettingKeepVisibleKey(fields formFieldsSnapshot, firstVisible int) woxwidget.Key {
	if fields.focused < firstVisible || fields.focused >= len(fields.definitions) {
		return ""
	}
	return pluginSettingRowKey(fields.focused)
}

func (a *App) pluginHeaderProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin, imageScale float32) launcherview.PluginHeaderProps {
	return launcherview.PluginHeaderProps{
		Name: plugin.Name, Version: plugin.Version, Author: plugin.Author,
		Icon: a.imageFor(plugin.Icon), FallbackColor: resultColors[snapshot.plugins.PluginSelected%len(resultColors)],
		MetadataActions: a.pluginMetadataActions(snapshot, plugin, imageScale), Management: a.pluginManagementActions(snapshot, plugin),
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
	width := float32(56)
	if window := a.settingsNativeWindow(); window != nil {
		if metrics, err := window.MeasureText(label, woxui.TextStyle{Size: 13}); err == nil {
			width = max(width, metrics.Size.Width+24)
		}
	}
	return launcherview.PluginTab{ID: id, Label: label, Width: width}
}

func (a *App) pluginDetailIntroAccent(background woxui.Color) woxui.Color {
	accent := woxui.Color{R: 33, G: 150, B: 243, A: 255}
	if themeColorIsDark(background) {
		accent = woxui.Color{R: 64, G: 196, B: 255, A: 255}
	}
	return accent
}

func (a *App) pluginDetailIntroFormProps(snapshot settingsSnapshot, imageScale float32, intro string, rows []woxwidget.Widget, accent woxui.Color) *launcherview.PluginFormProps {
	return &launcherview.PluginFormProps{
		Rows:        rows,
		Intro:       intro,
		IntroIcon:   a.imageForTint(settingNavIconSource("about"), &accent, physicalImageSize(16, imageScale)),
		IntroAccent: accent,
	}
}

// pluginDetailTabFormProps builds the shared hint box and readonly table used by store and installed plugin tabs.
func (a *App) pluginDetailTabFormProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin, tab string, width, imageScale float32, readOnly bool) *launcherview.PluginFormProps {
	switch tab {
	case "keywords":
		return a.pluginKeywordsFormProps(snapshot, plugin, width, imageScale, readOnly)
	case "commands":
		return a.pluginCommandsFormProps(snapshot, plugin, width, imageScale, readOnly)
	default:
		return nil
	}
}

func (a *App) pluginDetailEmptyFormProps(titleKey, subtitleKey string) *launcherview.PluginFormProps {
	return &launcherview.PluginFormProps{
		EmptyTitle:       a.translate(titleKey),
		EmptyDescription: a.translate(subtitleKey),
	}
}

// pluginKeywordsFormProps builds the shared hint box and keyword table used by store and installed plugin tabs.
func (a *App) pluginKeywordsFormProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin, width, imageScale float32, readOnly bool) *launcherview.PluginFormProps {
	if len(plugin.TriggerKeywords) == 0 {
		return a.pluginDetailEmptyFormProps("i18n:ui_plugin_no_trigger_keywords", "i18n:ui_plugin_no_trigger_keywords_subtitle")
	}
	rows := make([]launcherview.FormTableRow, 0, len(plugin.TriggerKeywords))
	for index, keyword := range plugin.TriggerKeywords {
		text := keyword
		if text == "*" {
			text = a.translate("i18n:ui_plugin_trigger_keyword_global")
		}
		rows = append(rows, launcherview.FormTableRow{Index: index, Cells: []launcherview.FormTableCell{{Text: text}}})
	}
	table := launcherview.FormTableFieldProps{
		ID: "plugin-keywords", Width: width, MaxHeight: 300, InlineTitle: true, ReadOnly: readOnly,
		Columns: []launcherview.FormTableColumn{
			{Label: a.translate("i18n:ui_plugin_trigger_keyword_column"), Tooltip: a.translate("i18n:ui_plugin_trigger_keyword_tooltip")},
		},
		Rows: rows, EmptyLabel: a.translate("i18n:ui_plugin_no_trigger_keywords"), Theme: snapshot.palette.componentTheme(),
	}
	accent := a.pluginDetailIntroAccent(snapshot.palette.background)
	return a.pluginDetailIntroFormProps(snapshot, imageScale, a.translate("i18n:ui_plugin_trigger_keywords_tip"), []woxwidget.Widget{
		woxwidget.Keyed{Key: "plugin-keyword-table", Child: launcherview.FormTableField(table)},
	}, accent)
}

func (a *App) pluginCommandsFormProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin, width, imageScale float32, readOnly bool) *launcherview.PluginFormProps {
	if len(plugin.Commands) == 0 {
		return a.pluginDetailEmptyFormProps("i18n:ui_plugin_no_commands", "i18n:ui_plugin_no_commands_subtitle")
	}
	commands := append([]pluginCommand(nil), plugin.Commands...)
	sort.SliceStable(commands, func(i, j int) bool { return commands[i].Command < commands[j].Command })
	rows := make([]launcherview.FormTableRow, 0, len(commands))
	for index, command := range commands {
		rows = append(rows, launcherview.FormTableRow{Index: index, Cells: []launcherview.FormTableCell{{Text: command.Command}, {Text: command.Description}}})
	}
	table := launcherview.FormTableFieldProps{
		ID: "plugin-commands", Width: width, MaxHeight: 300, InlineTitle: true, ReadOnly: readOnly,
		Columns: []launcherview.FormTableColumn{
			{Label: a.translate("i18n:ui_plugin_command_name_column"), Width: 120},
			{Label: a.translate("i18n:ui_plugin_command_desc_column")},
		},
		Rows: rows, EmptyLabel: a.translate("i18n:ui_plugin_no_commands"), Theme: snapshot.palette.componentTheme(),
	}
	accent := a.pluginDetailIntroAccent(snapshot.palette.background)
	return a.pluginDetailIntroFormProps(snapshot, imageScale, a.translate("i18n:ui_plugin_commands_tip"), []woxwidget.Widget{
		woxwidget.Keyed{Key: "plugin-command-table", Child: launcherview.FormTableField(table)},
	}, accent)
}

// pluginMetadataProps restores Flutter's non-editing plugin detail tabs from core metadata.
func (a *App) pluginMetadataProps(plugin pluginSettingsPlugin, tab string) launcherview.PluginMetadataProps {
	props := launcherview.PluginMetadataProps{}
	switch tab {
	case "description":
		props.DescriptionOnly = true
		props.Description = plugin.Description
	case "privacy":
		accesses := pluginPrivacyAccesses(plugin.Features)
		if len(accesses) == 0 {
			props.EmptyTitle = a.translate("i18n:ui_plugin_no_data_access")
			props.EmptyDescription = a.translate("i18n:ui_plugin_no_data_access_subtitle")
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
	order := []string{"requireActiveWindowName", "requireActiveWindowPid", "requireActiveWindowId", "requireActiveWindowIcon", "requireActiveWindowIsOpenSaveDialog", "requireActiveWindowIsOpenSaveDialogSelectFolder", "requireActiveBrowserUrl", "llm"}
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
	case "requireActiveWindowIsOpenSaveDialogSelectFolder":
		return a.translate("i18n:ui_plugin_privacy_open_save_dialog_select_folder_desc")
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
	case "requireActiveWindowIsOpenSaveDialogSelectFolder":
		return a.translate("i18n:ui_plugin_privacy_open_save_dialog_select_folder")
	case "requireActiveBrowserUrl":
		return a.translate("i18n:ui_plugin_privacy_browser_url")
	case "llm":
		return a.translate("i18n:ui_plugin_privacy_llm")
	default:
		return access
	}
}

func (a *App) pluginStoreDetailProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin, width, imageScale float32) *launcherview.PluginStoreDetailProps {
	plugins := snapshot.plugins
	activeTab := plugins.PluginDetailTab
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
		externalIcon = a.imageForTint(settingControlIconSource("external"), &iconTint, physicalImageSize(13, imageScale))
		if strings.Contains(strings.ToLower(plugin.Website), "github.com") {
			websiteChipLabel = "GitHub ↗"
			websiteIcon = a.imageForTint(pluginMetadataIconSource("github"), &iconTint, physicalImageSize(14, imageScale))
		}
	}
	runtimeLabel := pluginRuntimeLabel(plugin.Runtime)
	var runtimeIcon *woxui.Image
	if source := pluginMetadataIconSource(strings.ToLower(plugin.Runtime)); source.ImageData != "" {
		runtimeIcon = a.imageFor(source)
	}
	var screenshot *woxui.Image
	screenshotLoading := false
	var onScreenshot func()
	if activeTab == "description" && len(plugin.ScreenshotURLs) > 0 {
		source := woxImage{ImageType: "url", ImageData: plugin.ScreenshotURLs[0]}
		// Request a high-res decode from the detail width. Display height is derived later from
		// the description content width so store padding cannot stretch the aspect ratio.
		screenshotWidth := max(float32(1), width)
		requestSize := int(min(float32(2048), max(float32(512), screenshotWidth*2)))
		screenshot = a.imageForSize(source, requestSize)
		screenshotLoading = screenshot == nil
		onScreenshot = func() { a.openPreviewImageOverlay(source) }
	}
	contentWidth := max(float32(0), width-32)
	var tabForm *launcherview.PluginFormProps
	var metadata *launcherview.PluginMetadataProps
	switch activeTab {
	case "keywords", "commands":
		tabForm = a.pluginDetailTabFormProps(snapshot, plugin, activeTab, contentWidth, imageScale, true)
	case "privacy":
		meta := a.pluginMetadataProps(plugin, activeTab)
		metadata = &meta
	}
	return &launcherview.PluginStoreDetailProps{
		Name: plugin.Name, Version: plugin.Version, Author: plugin.Author, Description: plugin.Description, Runtime: runtimeLabel,
		WebsiteLabel: websiteLabel, WebsiteChipLabel: websiteChipLabel,
		Icon: a.imageFor(plugin.Icon), ExternalIcon: externalIcon, RuntimeIcon: runtimeIcon, WebsiteIcon: websiteIcon,
		FallbackColor: resultColors[plugins.PluginSelected%len(resultColors)], Management: a.pluginManagementActions(snapshot, plugin),
		ActiveTab: activeTab, Tabs: a.pluginStoreDetailTabs(), TabForm: tabForm, Metadata: metadata,
		Screenshot: screenshot, ScreenshotLoading: screenshotLoading, Error: plugins.PluginOperationError, OnWebsite: onWebsite, OnScreenshot: onScreenshot, OnSelectTab: a.selectPluginDetailTab,
	}
}

func (a *App) pluginFilterPanelProps(snapshot settingsSnapshot) *launcherview.PluginFilterPanelProps {
	plugins := snapshot.plugins
	if !plugins.PluginFilterOpen {
		return nil
	}
	filters := plugins.PluginFilters
	options := make([]launcherview.PluginFilterOption, 0, 4)
	if plugins.PluginsStore {
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
	if plugins.PluginsStore {
		runtimes = append(runtimes, launcherview.PluginFilterOption{ID: "runtime-script", Label: a.translate("i18n:ui_runtime_name_script"), Value: filters.runtimeScriptOnly})
	} else {
		runtimes = append(runtimes,
			launcherview.PluginFilterOption{ID: "runtime-script-nodejs", Label: a.translate("i18n:plugin_wpm_script_template_nodejs"), Value: filters.runtimeScriptNodeJSOnly},
			launcherview.PluginFilterOption{ID: "runtime-script-python", Label: a.translate("i18n:plugin_wpm_script_template_python"), Value: filters.runtimeScriptPythonOnly},
		)
	}
	labelWidth := float32(50)
	if window := a.settingsNativeWindow(); window != nil {
		for _, option := range options {
			if metrics, err := window.MeasureText(option.Label, woxui.TextStyle{Size: 13}); err == nil {
				labelWidth = max(labelWidth, metrics.Size.Width)
			}
		}
		if metrics, err := window.MeasureText(a.translate("i18n:ui_runtime_status"), woxui.TextStyle{Size: 13}); err == nil {
			labelWidth = max(labelWidth, metrics.Size.Width)
		}
	}
	return &launcherview.PluginFilterPanelProps{
		Width: 660, LabelWidth: min(labelWidth, float32(180)), RuntimeTitle: a.translate("i18n:ui_runtime_status"),
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

// pluginManagementActions shares install, upgrade, and uninstall actions between plugin details.
func (a *App) pluginManagementActions(snapshot settingsSnapshot, plugin pluginSettingsPlugin) []launcherview.PluginAction {
	plugins := snapshot.plugins
	busy := plugins.PluginOperation != ""
	if !plugin.IsInstalled {
		return []launcherview.PluginAction{{
			ID: "plugin-install", Label: pluginOperationButtonLabel(plugins, "install", plugin.ID, a.translate("i18n:ui_plugin_install")), Width: 104,
			Enabled: !busy, Primary: true, OnTap: func() { a.runPluginOperation("install") },
		}}
	}
	actions := make([]launcherview.PluginAction, 0, 3)
	if plugin.IsUpgradable {
		actions = append(actions, launcherview.PluginAction{ID: "plugin-upgrade", Label: pluginOperationButtonLabel(plugins, "upgrade", plugin.ID, a.translate("i18n:ui_update")), Width: 104, Enabled: !busy, Primary: true, OnTap: func() { a.runPluginOperation("upgrade") }})
	}
	if !plugin.IsSystem {
		label := a.translate("i18n:ui_plugin_uninstall")
		if plugins.PluginUninstallArmed == plugin.ID {
			label = a.translate("i18n:ui_cloud_sync_confirm") + " " + label
		}
		actions = append(actions, launcherview.PluginAction{ID: "plugin-uninstall", Label: pluginOperationButtonLabel(plugins, "uninstall", plugin.ID, label), Width: 124, Enabled: !busy, OnTap: func() { a.runPluginOperation("uninstall") }})
	}
	if plugin.IsDisable {
		actions = append(actions, launcherview.PluginAction{ID: "plugin-enable", Label: pluginOperationButtonLabel(plugins, "enable", plugin.ID, a.translate("i18n:ui_plugin_enable")), Width: 96, Enabled: !busy, OnTap: func() { a.runPluginOperation("enable") }})
	} else {
		actions = append(actions, launcherview.PluginAction{ID: "plugin-disable", Label: pluginOperationButtonLabel(plugins, "disable", plugin.ID, a.translate("i18n:ui_plugin_disable")), Width: 96, Enabled: !busy, OnTap: func() { a.runPluginOperation("disable") }})
	}
	if !plugin.IsSystem && strings.TrimSpace(plugin.PluginDirectory) != "" {
		actions = append(actions, launcherview.PluginAction{
			ID: "plugin-directory", Label: a.translate("i18n:ui_plugin_open_directory"), Width: 132, Enabled: !busy, OnTap: a.openSelectedPluginDirectory,
		})
	}
	return actions
}

// pluginMetadataActions exposes the browser action without platform-specific widgets.
func (a *App) pluginMetadataActions(snapshot settingsSnapshot, plugin pluginSettingsPlugin, imageScale float32) []launcherview.PluginAction {
	actions := make([]launcherview.PluginAction, 0, 1)
	if strings.TrimSpace(plugin.Website) != "" {
		iconTint := snapshot.palette.resultTitle
		actions = append(actions, launcherview.PluginAction{
			ID: "plugin-website", Label: a.translate("i18n:ui_plugin_website"), Icon: a.imageForTint(settingControlIconSource("external"), &iconTint, physicalImageSize(14, imageScale)),
			Width: 88, Enabled: true, OnTap: a.openSelectedPluginWebsite,
		})
	}
	return actions
}

func pluginOperationButtonLabel(plugins pluginSettingsSnapshot, kind, pluginID, idle string) string {
	if plugins.PluginOperation == kind+":"+pluginID {
		return idle + "…"
	}
	return idle
}
