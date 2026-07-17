package launcher

import (
	"fmt"
	"sort"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
)

// buildPluginSettingsPage maps plugin state into the shared catalog and detail views.
func (a *App) buildPluginSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	innerHeight := max(float32(0), height-24)
	listWidth := min(float32(300), max(float32(250), innerWidth*0.31))
	detailWidth := max(float32(0), innerWidth-listWidth-1)
	return launcherview.PluginSettingsPage(launcherview.PluginSettingsPageProps{
		Width:  width,
		Height: height,
		List:   a.pluginListProps(snapshot, listWidth, innerHeight),
		Detail: a.pluginDetailProps(snapshot, detailWidth, innerHeight),
		Theme:  snapshot.palette.componentTheme(),
	})
}

// pluginListProps resolves localized catalog labels, images, selection, and callbacks.
func (a *App) pluginListProps(snapshot settingsSnapshot, width, height float32) launcherview.PluginListProps {
	props := launcherview.PluginListProps{
		Width: width, Height: height, Scroll: snapshot.pluginListScroll,
		Placeholder: fmt.Sprintf(a.translate("i18n:ui_search_plugins"), len(snapshot.plugins)),
		Search:      snapshot.pluginSearch, Focused: snapshot.pluginSearchFocused, Window: a.settingsNativeWindow(),
		EmptyLabel: a.translate("i18n:ui_setting_plugin_empty_data"), Theme: snapshot.palette.componentTheme(),
		OnViewport: a.setPluginListViewport, OnScroll: a.scrollPluginList, OnCaret: a.focusPluginSearch, OnClear: a.clearPluginSearch,
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

	filtered := filterPlugins(snapshot.plugins, snapshot.pluginSearch.Text)
	props.Items = make([]launcherview.PluginListItem, 0, len(filtered))
	for visibleIndex, entry := range filtered {
		index := entry.index
		plugin := entry.plugin
		status := strings.TrimSpace(plugin.Version + "  " + plugin.Author)
		if snapshot.pluginsStore && !plugin.IsInstalled {
			status = a.translate("i18n:ui_cloud_sync_key_available") + "  " + status
		} else if plugin.IsUpgradable {
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
		props.Store = a.pluginStoreDetailProps(snapshot, plugin)
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
		rows = append(rows, a.buildFormField(form.formFieldsSnapshot, callbacks, snapshot.palette, index, definition, innerWidth, formDefinitionHeight(definition)))
	}
	editor.Form = &launcherview.PluginFormProps{
		Rows: rows, ContentHeight: formDefinitionsContentHeight(form.definitions), Scroll: form.scroll,
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
		{ID: "settings", Label: a.translate("i18n:ui_plugin_tab_settings"), Width: 82},
		{ID: "description", Label: a.translate("i18n:ui_plugin_tab_description"), Width: 92},
		{ID: "keywords", Label: a.translate("i18n:ui_plugin_tab_trigger_keywords"), Width: 126},
		{ID: "commands", Label: a.translate("i18n:ui_plugin_tab_commands"), Width: 88},
		{ID: "privacy", Label: a.translate("i18n:ui_plugin_tab_privacy"), Width: 76},
	}
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

func (a *App) pluginStoreDetailProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin) *launcherview.PluginStoreDetailProps {
	return &launcherview.PluginStoreDetailProps{
		Name: plugin.Name, Subtitle: plugin.Author + " · " + plugin.Version + " · " + plugin.Runtime, Description: plugin.Description,
		Icon: a.imageFor(plugin.Icon), FallbackColor: resultColors[snapshot.pluginSelected%len(resultColors)],
		MetadataActions: a.pluginMetadataActions(plugin), Management: a.pluginManagementActions(snapshot, plugin), Error: snapshot.pluginOperationError,
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
