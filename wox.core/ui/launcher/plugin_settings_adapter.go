package launcher

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"wox/network"

	woxplugin "wox/plugin"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// buildPluginSettingsPage maps plugin state into the shared catalog and detail views.
func (c *pluginSettingsController) buildPluginSettingsPage(snapshot settingsSnapshot, width, height, imageScale float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-40)
	innerHeight := max(float32(0), height-40)
	listWidth := woxcomponent.SettingsCatalogListWidth(innerWidth)
	detailWidth := max(float32(0), innerWidth-listWidth-woxcomponent.SettingsCatalogDividerGutter)
	return launcherview.PluginSettingsPage(launcherview.PluginSettingsPageProps{
		Width:       width,
		Height:      height,
		List:        c.pluginListProps(snapshot, listWidth, innerHeight, imageScale),
		Detail:      c.pluginDetailProps(snapshot, detailWidth, innerHeight, imageScale),
		FilterPanel: c.pluginFilterPanelProps(snapshot),
		Theme:       snapshot.palette,
	})
}

// pluginListProps resolves localized catalog labels, images, selection, and callbacks.
func (c *pluginSettingsController) pluginListProps(snapshot settingsSnapshot, width, height, imageScale float32) launcherview.PluginListProps {
	plugins := snapshot.plugins
	iconTint := snapshot.palette.Text
	selectedIconTint := snapshot.palette.SelectionText
	installedTint := woxui.Color{R: 56, G: 176, B: 92, A: 255}
	props := launcherview.PluginListProps{
		Width: width, Height: height,
		Placeholder:           fmt.Sprintf(c.language.translate("i18n:ui_search_plugins"), len(plugins.Plugins)),
		Search:                plugins.PluginSearch,
		Focused:               plugins.PluginSearchFocused,
		Window:                c.settings.settingsNativeWindow(),
		FilterIcon:            c.imageForTint(settingControlIconSource("filter"), &iconTint, physicalImageSize(16, imageScale)),
		InstalledIcon:         c.imageForTint(settingControlIconSource("check-circle"), &installedTint, physicalImageSize(20, imageScale)),
		InstalledSelectedIcon: c.imageForTint(settingControlIconSource("check-circle"), &selectedIconTint, physicalImageSize(20, imageScale)),
		FilterLabel:           c.language.translate("i18n:ui_filter_placeholder"),
		FilterActive:          plugins.PluginFilters.applied(plugins.PluginsStore),
		Theme:                 snapshot.palette,
		OnClear:               c.clearPluginSearch,
		OnSearchKey:           c.onPluginSearchKey, OnSearchFocusChange: c.setPluginSearchFocused,
		OnSearchChanged: func(value string) { _ = c.setPluginSearchValue(value) }, OnSetSearchValue: c.setPluginSearchValue,
		OnFilter: c.togglePluginFilterPanel,
	}
	if plugins.PluginsLoading && len(plugins.Plugins) == 0 {
		props.Message = c.language.translate("i18n:ui_cloud_sync_plugin_exclusions_loading")
		return props
	}
	if plugins.PluginsError != "" && len(plugins.Plugins) == 0 {
		props.Message = plugins.PluginsError
		props.MessageError = true
		return props
	}

	filtered := filterPlugins(plugins.Plugins, plugins.PluginSearch.Text, plugins.PluginFilters, plugins.PluginsStore, snapshot.general.Data.UsePinYin)
	props.Placeholder = fmt.Sprintf(c.language.translate("i18n:ui_search_plugins"), len(filtered))
	c.applyPluginCatalogEmptyState(&props, plugins, filtered, iconTint, imageScale)
	props.Entries = c.pluginListEntries(snapshot, filtered, imageScale)
	props.ResolveIcon = c.pluginListIconResolver(filtered, imageScale)
	return props
}

// pluginListIconResolver decodes a catalog icon at the row's physical size.
// The list asks for it only while the row is on screen. Decoding every store
// icon at 256px overflows the shared image cache, and visible rows then
// flicker between artwork and the fallback color.
func (c *pluginSettingsController) pluginListIconResolver(filtered []filteredPlugin, imageScale float32) func(string) *woxui.Image {
	icons := make(map[string]woxImage, len(filtered))
	for _, entry := range filtered {
		icons[entry.plugin.ID] = entry.plugin.Icon
	}
	size := physicalImageSize(launcherview.PluginListIconSize, imageScale)
	background := settingsPalette().Background
	return func(id string) *woxui.Image {
		source, ok := icons[id]
		if !ok {
			return nil
		}
		return c.imageForSurface(source, size, background)
	}
}

// pluginListEntries builds catalog rows. Installed plugins group by enabled state.
// Store plugins group by catalog, with the official store first. A single visible
// catalog stays a flat list so the header is not repeated above every row.
func (c *pluginSettingsController) pluginListEntries(snapshot settingsSnapshot, filtered []filteredPlugin, imageScale float32) []launcherview.PluginListEntry {
	plugins := snapshot.plugins
	visibleIndex := 0
	appendItem := func(entries []launcherview.PluginListEntry, entry filteredPlugin) []launcherview.PluginListEntry {
		index := entry.index
		plugin := entry.plugin
		status := strings.TrimSpace(plugin.Version + "  " + plugin.Author)
		if plugin.IsUpgradable {
			status = c.language.translate("i18n:ui_update") + "  " + status
		}
		badge := ""
		if plugin.IsSystem {
			badge = c.language.translate("i18n:ui_setting_plugin_system_tag")
		} else if plugin.IsDev {
			badge = c.language.translate("i18n:ui_plugin_dev_tag")
		}
		if network.IsOffline() && !plugin.IsSystem {
			badge = c.language.translate("i18n:ui_plugin_offline_paused")
		}
		itemIndex := visibleIndex
		visibleIndex++
		return append(entries, launcherview.PluginListEntry{
			ID: plugin.ID,
			Item: launcherview.PluginListItem{
				ID: plugin.ID, Name: plugin.Name, Status: status, Badge: badge, ShowInstalledIcon: plugins.PluginsStore && plugin.IsInstalled,
				FallbackColor: resultColors[itemIndex%len(resultColors)], Selected: index == plugins.PluginSelected,
				Highlighted: snapshot.highlight == "plugin:"+plugin.ID, Disabled: plugin.IsDisable,
				OnSelect: func() { c.selectPlugin(index) },
			},
		})
	}
	if plugins.PluginsStore {
		sections := groupStorePlugins(filtered)
		showHeaders := len(sections) > 1
		entries := make([]launcherview.PluginListEntry, 0, len(filtered)+len(sections))
		for _, section := range sections {
			if showHeaders {
				label, icon := c.pluginStoreSection(section.ID, imageScale)
				entries = append(entries, launcherview.PluginListEntry{ID: section.ID, Header: label, HasHeaderIcon: true, HeaderIcon: icon})
			}
			for _, entry := range section.Plugins {
				entries = appendItem(entries, entry)
			}
		}
		return entries
	}
	sections := groupInstalledPlugins(filtered)
	entries := make([]launcherview.PluginListEntry, 0, len(filtered)+len(sections))
	for _, section := range sections {
		label := c.language.translate("i18n:ui_setting_plugin_section_enabled")
		if section.ID == pluginSectionDisabled {
			label = c.language.translate("i18n:ui_setting_plugin_section_disabled")
		}
		entries = append(entries, launcherview.PluginListEntry{ID: section.ID, Header: label})
		for _, entry := range section.Plugins {
			entries = appendItem(entries, entry)
		}
	}
	return entries
}

// pluginStoreSection resolves one catalog title and its brand mark.
func (c *pluginSettingsController) pluginStoreSection(storeID string, imageScale float32) (string, *woxui.Image) {
	label, source := woxplugin.PluginStorePresentation(storeID)
	label = c.language.translate(label)
	if !c.imagesReady() || source.IsEmpty() {
		return label, nil
	}
	size := physicalImageSize(launcherview.PluginListSectionIconSize, imageScale)
	return label, c.imageForSurface(fromCoreImage(source), size, settingsPalette().Background)
}

func (c *pluginSettingsController) applyPluginCatalogEmptyState(props *launcherview.PluginListProps, plugins pluginSettingsSnapshot, filtered []filteredPlugin, iconTint woxui.Color, imageScale float32) {
	if len(filtered) > 0 {
		return
	}
	emptyIconTint := iconTint
	emptyIconTint.A = 160
	props.EmptyIcon = c.imageForTint(settingControlIconSource("search"), &emptyIconTint, physicalImageSize(24, imageScale))
	if len(plugins.Plugins) > 0 {
		props.EmptyTitle = c.language.translate("i18n:ui_no_matches")
		props.EmptyDescription = c.language.translate("i18n:ui_setting_catalog_search_empty_subtitle")
		return
	}
	props.EmptyTitle = c.language.translate("i18n:ui_setting_plugin_empty_data")
	props.EmptyDescription = c.language.translate("i18n:ui_setting_plugin_empty_subtitle")
}

// pluginDetailProps maps the selected plugin into an empty, store, or editable detail view.
func (c *pluginSettingsController) pluginDetailProps(snapshot settingsSnapshot, width, height, imageScale float32) launcherview.PluginDetailProps {
	plugins := snapshot.plugins
	emptyIconTint := snapshot.palette.Text
	emptyIconTint.A = 160
	props := launcherview.PluginDetailProps{
		Width: width, Height: height, EmptyLabel: c.language.translate("i18n:ui_setting_plugin_empty_data"),
		EmptyTitle: c.language.translate("i18n:ui_setting_plugin_empty_data"), EmptyDescription: c.language.translate("i18n:ui_setting_plugin_empty_subtitle"),
		EmptyIcon: c.imageForTint(settingControlIconSource("search"), &emptyIconTint, physicalImageSize(24, imageScale)), Window: c.settings.settingsNativeWindow(),
		Theme: snapshot.palette,
	}
	if plugins.PluginSelected < 0 || plugins.PluginSelected >= len(plugins.Plugins) {
		return props
	}
	plugin := plugins.Plugins[plugins.PluginSelected]
	if plugins.PluginForm == nil {
		props.Store = c.pluginStoreDetailProps(snapshot, plugin, width, imageScale)
		return props
	}

	form := plugins.PluginForm
	editor := &launcherview.PluginEditorProps{
		Header:   c.pluginHeaderProps(snapshot, plugin, imageScale),
		ScrollID: "plugin-detail-" + plugin.ID,
		Error:    plugins.PluginOperationError,
	}
	labelWidth := c.pluginFormLabelWidth(form.definitions[1:])
	if plugin.HasNativeSettings {
		labelWidth = max(labelWidth, c.pluginFormLabelWidth([]formDefinition{{Value: formDefinitionValue{Label: "i18n:ui_plugin_native_settings_title"}}}))
	}
	callbacks := formFieldCallbacks{
		idPrefix:          "plugin-settings",
		labelWidth:        labelWidth,
		imageScale:        imageScale,
		focus:             c.focusPluginFormField,
		blur:              c.blurPluginFormField,
		change:            c.changePluginFormChoice,
		setText:           c.setPluginFormText,
		pickDir:           c.pickPluginFormDirectory,
		onKey:             c.onPluginSettingsKey,
		openTable:         c.openFormTable,
		openChoice:        c.openPluginFormChoice,
		openAIModelChoice: c.openPluginAIModelChoice,
		setAIModelName:    c.setPluginAIModelName,
		finishAIModelEdit: c.finishPluginAIModelEdit,
		openAISettings:    c.openPluginAISettings,
		openModel:         c.openPluginModelManager,
		recordKey:         c.recordPluginFormHotkey,
		runServiceAction:  c.runPluginServiceAction,
		openLink:          c.openPluginSettingLink,
		serviceBusy:       plugins.PluginSettingAction.actionID != "",
		fieldErrors:       form.fieldErrors,
	}
	if plugins.PluginSettingAction.pluginID == plugin.ID {
		callbacks.serviceError = plugins.PluginSettingAction.errorText
	}
	editor.DescriptionDetail = c.pluginStoreDetailProps(snapshot, plugin, width, imageScale)
	metadata := c.pluginMetadataProps(plugin, "privacy")
	editor.Metadata = &metadata
	keywordDefinition := form.definitions[0]
	innerWidth := max(float32(0), width-32)
	keywordTable := c.formTables.formTableFieldProps(form.formFieldsSnapshot, callbacks, snapshot.palette, 0, keywordDefinition, innerWidth, 0)
	keywordTable.Title = ""
	keywordTable.Description = c.language.translate("i18n:ui_plugin_trigger_keywords_tip")
	for index := range keywordTable.Rows {
		if len(keywordTable.Rows[index].Cells) > 0 && keywordTable.Rows[index].Cells[0].Text == "*" {
			keywordTable.Rows[index].Cells[0].Text = c.language.translate("i18n:ui_plugin_trigger_keyword_global")
		}
	}
	c.addPluginKeywordQueryTests(&keywordTable, plugin.ID, form.values["TriggerKeywords"], imageScale)
	accent := snapshot.palette.Info
	editor.Keywords = c.pluginDetailIntroFormProps(snapshot, imageScale, "", []woxwidget.Widget{
		woxwidget.Keyed{Key: pluginSettingRowKey(0), Child: launcherview.FormTableField(keywordTable)},
	}, accent)
	editor.Keywords.SectionLabel = c.language.translate("i18n:ui_plugin_tab_trigger_keywords")
	editor.Commands = c.pluginCommandsFormProps(snapshot, plugin, innerWidth, imageScale, true, true, pluginFormTriggerKeywords(plugin, form.values))
	editor.Tools = c.pluginToolsFormProps(snapshot, plugin, innerWidth, imageScale)

	keepVisibleKey := pluginSettingKeepVisibleKey(form.formFieldsSnapshot, 0)
	settingDefinitions := form.definitions[1:]
	rows := make([]woxwidget.Widget, 0, len(settingDefinitions))
	for index, definition := range settingDefinitions {
		formIndex := index + 1
		if snapshot.highlight == "plugin-setting:"+plugin.ID+"\x00"+definition.Value.Key {
			keepVisibleKey = pluginSettingRowKey(formIndex)
		}
		field := c.buildFormField(form.formFieldsSnapshot, callbacks, snapshot.palette, formIndex, definition, innerWidth, 0)
		target := woxcomponent.WoxSettingTarget(woxcomponent.SettingTargetProps{
			Width: innerWidth, Highlighted: snapshot.highlight == "plugin-setting:"+plugin.ID+"\x00"+definition.Value.Key, Child: field, Theme: snapshot.palette,
		})
		rows = append(rows, woxwidget.Keyed{Key: pluginSettingRowKey(formIndex), Child: target})
	}
	if plugin.HasNativeSettings {
		pluginID := plugin.ID
		field := launcherview.FormButtonField(launcherview.FormButtonFieldProps{
			ID: "plugin-native-settings-field", Label: c.language.translate("i18n:ui_plugin_native_settings_title"),
			Description: c.language.translate("i18n:ui_plugin_native_settings_description"),
			ButtonID:    "plugin-native-settings", ButtonLabel: c.language.translate("i18n:ui_plugin_native_settings_open"),
			Disabled: c.nativeSettingsPluginID == pluginID, Width: innerWidth, LabelWidth: callbacks.labelWidth, Theme: snapshot.palette,
			OnTap: func() { c.openNativePluginSettings(pluginID) },
		})
		rows = append([]woxwidget.Widget{woxwidget.Keyed{Key: "plugin-native-settings-row", Child: field}}, rows...)
	}
	if form.statusError && strings.TrimSpace(form.status) != "" {
		status := woxwidget.TextBlock{
			Value: form.status, Width: innerWidth, LineHeight: 18,
			Style: woxui.TextStyle{Size: snapshot.palette.Scaled(woxcomponent.SettingsHelpFontSize)}, Color: snapshot.palette.Error,
		}
		rows = append([]woxwidget.Widget{woxwidget.Keyed{Key: "plugin-settings-status", Child: status}}, rows...)
	}
	editor.Form = &launcherview.PluginFormProps{
		SectionLabel: c.language.translate("i18n:ui_plugin_tab_settings"),
		Rows:         rows, KeepVisibleKey: keepVisibleKey,
		EmptyTitle: c.language.translate("i18n:ui_plugin_no_settings"), EmptyDescription: c.language.translate("i18n:ui_plugin_no_settings_subtitle"),
	}
	props.Editor = editor
	return props
}

// openPluginSettingLink opens Markdown links from plugin setting help text.
func (c *pluginSettingsController) openPluginSettingLink(target string) {
	window := c.settings.settingsNativeWindow()
	if window == nil {
		return
	}
	if err := window.OpenExternalURL(target); err != nil {
		util.GetLogger().Error(c.lifecycleCtx, fmt.Sprintf("open plugin setting link: %v", err))
	}
}

// pluginFormLabelWidth mirrors Flutter's shared measured label column for each plugin.
func (c *pluginSettingsController) pluginFormLabelWidth(definitions []formDefinition) float32 {
	window := c.settings.settingsNativeWindow()
	if window == nil {
		return 120
	}
	return c.measureFormLabelWidth(definitions, window, 0, 200)
}

func pluginSettingRowKey(index int) woxwidget.Key {
	return woxwidget.Key(fmt.Sprintf("plugin-setting-row-%d", index))
}

// pluginSettingKeepVisibleKey maps form focus to a measured row in the plugin detail page.
func pluginSettingKeepVisibleKey(fields formFieldsSnapshot, firstVisible int) woxwidget.Key {
	// A newly selected plugin preselects a field without activating it. Scrolling to
	// that field would skip the description and jump again when screenshots load.
	if !fields.active || fields.focused < firstVisible || fields.focused >= len(fields.definitions) {
		return ""
	}
	return pluginSettingRowKey(fields.focused)
}

func (c *pluginSettingsController) pluginHeaderProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin, imageScale float32) launcherview.PluginHeaderProps {
	props := launcherview.PluginHeaderProps{
		Name: plugin.Name, Version: plugin.Version, Author: plugin.Author,
		Icon: c.imageForSurface(plugin.Icon, 256, settingsPalette().Background), FallbackColor: resultColors[snapshot.plugins.PluginSelected%len(resultColors)],
		MetadataActions: c.pluginMetadataActions(snapshot, plugin, imageScale), Management: c.pluginManagementActions(snapshot, plugin),
	}
	if plugin.IsDisable {
		props.DisabledLabel = c.language.translate("i18n:ui_plugin_filter_disabled")
	}
	return props
}

func (c *pluginSettingsController) pluginDetailIntroFormProps(snapshot settingsSnapshot, imageScale float32, intro string, rows []woxwidget.Widget, accent woxui.Color) *launcherview.PluginFormProps {
	return &launcherview.PluginFormProps{
		Rows:        rows,
		Intro:       intro,
		IntroIcon:   c.imageForTint(settingNavIconSource("about"), &accent, physicalImageSize(16, imageScale)),
		IntroAccent: accent,
	}
}

func (c *pluginSettingsController) pluginDetailEmptyFormProps(titleKey, subtitleKey string) *launcherview.PluginFormProps {
	return &launcherview.PluginFormProps{
		EmptyTitle:       c.language.translate(titleKey),
		EmptyDescription: c.language.translate(subtitleKey),
	}
}

// pluginKeywordsFormProps builds the keyword table with its shared title and help text.
func (c *pluginSettingsController) pluginKeywordsFormProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin, width, imageScale float32, readOnly bool) *launcherview.PluginFormProps {
	if len(plugin.TriggerKeywords) == 0 {
		return c.pluginDetailEmptyFormProps("i18n:ui_plugin_no_trigger_keywords", "i18n:ui_plugin_no_trigger_keywords_subtitle")
	}
	rows := make([]launcherview.FormTableRow, 0, len(plugin.TriggerKeywords))
	for index, keyword := range plugin.TriggerKeywords {
		text := keyword
		if text == "*" {
			text = c.language.translate("i18n:ui_plugin_trigger_keyword_global")
		}
		rows = append(rows, launcherview.FormTableRow{Index: index, Cells: []launcherview.FormTableCell{{Text: text}}})
	}
	table := launcherview.FormTableFieldProps{
		ID: "plugin-keywords", Width: width, MaxHeight: 300, InlineTitle: true, ReadOnly: readOnly,
		Description: c.language.translate("i18n:ui_plugin_trigger_keywords_tip"),
		Columns: []launcherview.FormTableColumn{
			{Label: c.language.translate("i18n:ui_plugin_trigger_keyword_column"), Tooltip: c.language.translate("i18n:ui_plugin_trigger_keyword_tooltip")},
		},
		Rows: rows, EmptyLabel: c.language.translate("i18n:ui_plugin_no_trigger_keywords"), Theme: snapshot.palette,
	}
	accent := snapshot.palette.Info
	form := c.pluginDetailIntroFormProps(snapshot, imageScale, "", []woxwidget.Widget{
		woxwidget.Keyed{Key: "plugin-keyword-table", Child: launcherview.FormTableField(table)},
	}, accent)
	form.SectionLabel = c.language.translate("i18n:ui_plugin_tab_trigger_keywords")
	return form
}

func (c *pluginSettingsController) pluginCommandsFormProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin, width, imageScale float32, readOnly, testable bool, testTriggers []string) *launcherview.PluginFormProps {
	if len(plugin.Commands) == 0 {
		return c.pluginDetailEmptyFormProps("i18n:ui_plugin_no_commands", "i18n:ui_plugin_no_commands_subtitle")
	}
	commands := append([]pluginCommand(nil), plugin.Commands...)
	sort.SliceStable(commands, func(i, j int) bool { return commands[i].Command < commands[j].Command })
	rows := make([]launcherview.FormTableRow, 0, len(commands))
	for index, command := range commands {
		rows = append(rows, launcherview.FormTableRow{Index: index, Cells: []launcherview.FormTableCell{{Text: command.Command}, {Text: command.Description}}})
	}
	table := launcherview.FormTableFieldProps{
		ID: "plugin-commands", Width: width, MaxHeight: 300, InlineTitle: true, ReadOnly: readOnly,
		Description: c.language.translate("i18n:ui_plugin_commands_tip"),
		Columns: []launcherview.FormTableColumn{
			{Label: c.language.translate("i18n:ui_plugin_command_name_column"), Width: 120},
			{Label: c.language.translate("i18n:ui_plugin_command_desc_column")},
		},
		Rows: rows, EmptyLabel: c.language.translate("i18n:ui_plugin_no_commands"), OperationLabel: c.language.translate("i18n:ui_operation"),
		OnTooltip: c.setSettingChoiceTooltip, Theme: snapshot.palette,
	}
	if testable {
		c.addPluginCommandQueryTests(&table, commands, testTriggers, imageScale)
	}
	accent := snapshot.palette.Info
	form := c.pluginDetailIntroFormProps(snapshot, imageScale, "", []woxwidget.Widget{
		woxwidget.Keyed{Key: "plugin-command-table", Child: launcherview.FormTableField(table)},
	}, accent)
	form.SectionLabel = c.language.translate("i18n:ui_plugin_tab_commands")
	return form
}

func (c *pluginSettingsController) pluginToolsFormProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin, width, imageScale float32) *launcherview.PluginFormProps {
	tools := c.pluginToolsForSettings(plugin)
	if len(tools) == 0 {
		return nil
	}
	sort.SliceStable(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	rows := make([]launcherview.FormTableRow, 0, len(tools))
	for index, tool := range tools {
		rows = append(rows, launcherview.FormTableRow{Index: index, Cells: []launcherview.FormTableCell{{Text: tool.Name}, {Text: tool.Description}}})
	}
	table := launcherview.FormTableFieldProps{
		ID: "plugin-tools", Width: width, MaxHeight: 300, InlineTitle: true, ReadOnly: true,
		Description: c.language.translate("i18n:ui_plugin_tools_tip"),
		Columns: []launcherview.FormTableColumn{
			{Label: c.language.translate("i18n:ui_plugin_tool_name_column"), Width: 160},
			{Label: c.language.translate("i18n:ui_plugin_tool_desc_column")},
		},
		Rows: rows, EmptyLabel: c.language.translate("i18n:ui_plugin_tab_tools"), Theme: snapshot.palette,
	}
	accent := snapshot.palette.Info
	form := c.pluginDetailIntroFormProps(snapshot, imageScale, "", []woxwidget.Widget{
		woxwidget.Keyed{Key: "plugin-tool-table", Child: launcherview.FormTableField(table)},
	}, accent)
	form.SectionLabel = c.language.translate("i18n:ui_plugin_tab_tools")
	return form
}

func (c *pluginSettingsController) pluginToolsForSettings(plugin pluginSettingsPlugin) []pluginTool {
	if listed := c.listInstalledPluginTools(plugin.ID); len(listed) > 0 {
		return listed
	}
	return append([]pluginTool(nil), plugin.Tools...)
}

func (c *pluginSettingsController) listInstalledPluginTools(pluginID string) []pluginTool {
	if strings.TrimSpace(pluginID) == "" {
		return nil
	}
	manager := woxplugin.GetPluginManager()
	if manager == nil {
		return nil
	}
	ctx := c.lifecycleCtx
	if ctx == nil {
		ctx = context.Background()
	}
	listed := manager.ListPluginTools(ctx, nil, woxplugin.ListPluginToolsOption{PluginId: pluginID})
	tools := make([]pluginTool, 0, len(listed.Tools))
	for _, item := range listed.Tools {
		tools = append(tools, pluginTool{Name: item.Tool.Name, Description: item.Tool.Description})
	}
	return tools
}

// addPluginKeywordQueryTests puts a launcher test action first in each keyword row's operation column.
// Global "*" keywords are skipped because they have no typed prefix to preview.
func (c *pluginSettingsController) addPluginKeywordQueryTests(table *launcherview.FormTableFieldProps, pluginID, value string, imageScale float32) {
	rows, err := decodeFormTableRows(value)
	if err != nil {
		return
	}
	for viewIndex := range table.Rows {
		sourceIndex := table.Rows[viewIndex].Index
		if sourceIndex < 0 || sourceIndex >= len(rows) {
			continue
		}
		keyword := strings.TrimSpace(fmt.Sprint(rows[sourceIndex]["keyword"]))
		if keyword == "*" {
			continue
		}
		table.Rows[viewIndex].LeadingActions = append(table.Rows[viewIndex].LeadingActions, c.pluginQueryTestAction(table.Theme, imageScale, c.language.translate("i18n:ui_plugin_test_keyword"), func() {
			c.runPluginKeywordQueryTest(pluginID, keyword)
		}))
	}
}

// addPluginCommandQueryTests puts a launcher test action first in each command row's operation column.
func (c *pluginSettingsController) addPluginCommandQueryTests(table *launcherview.FormTableFieldProps, commands []pluginCommand, triggers []string, imageScale float32) {
	for viewIndex := range table.Rows {
		sourceIndex := table.Rows[viewIndex].Index
		if sourceIndex < 0 || sourceIndex >= len(commands) {
			continue
		}
		command := commands[sourceIndex].Command
		table.Rows[viewIndex].LeadingActions = append(table.Rows[viewIndex].LeadingActions, c.pluginQueryTestAction(table.Theme, imageScale, c.language.translate("i18n:ui_plugin_test_command"), func() {
			c.runPluginCommandQueryTest(triggers, command)
		}))
	}
}

// pluginQueryTestAction is the shared bolt icon used by keyword and command test buttons.
func (c *pluginSettingsController) pluginQueryTestAction(theme woxcomponent.ControlTheme, imageScale float32, label string, onTap func()) launcherview.FormTableRowAction {
	tint := theme.TextSecondary
	return launcherview.FormTableRowAction{
		ID: "test-query", Label: label, Icon: c.imageForTint(settingControlIconSource("bolt"), &tint, physicalImageSize(16, imageScale)),
		OnTap: onTap,
		OnHover: func(inside bool, anchor woxui.Rect) {
			c.setSettingChoiceTooltip(inside, label, anchor)
		},
	}
}

// runPluginKeywordQueryTest opens the launcher as if the user typed this trigger keyword.
func (c *pluginSettingsController) runPluginKeywordQueryTest(pluginID, keyword string) {
	if queryText := pluginKeywordQueryText(keyword); queryText != "" {
		c.runQueryTest(newInputQuery(queryText))
		return
	}
	if strings.TrimSpace(keyword) != "*" || strings.TrimSpace(pluginID) == "" {
		return
	}
	query := newInputQuery("")
	query.QueryScope = queryScope{Plugins: []queryScopePlugin{{PluginID: pluginID}}}
	c.runQueryTest(query)
}

// runPluginCommandQueryTest opens the launcher as if the user entered this plugin command.
func (c *pluginSettingsController) runPluginCommandQueryTest(triggerKeywords []string, command string) {
	if queryText := pluginCommandQueryText(triggerKeywords, command); queryText != "" {
		c.runQueryTest(newInputQuery(queryText))
	}
}

// pluginMetadataProps restores Flutter's non-editing plugin detail tabs from core metadata.
func (c *pluginSettingsController) pluginMetadataProps(plugin pluginSettingsPlugin, tab string) launcherview.PluginMetadataProps {
	props := launcherview.PluginMetadataProps{}
	switch tab {
	case "description":
		props.DescriptionOnly = true
		props.Description = plugin.Description
	case "privacy":
		props.Header = c.language.translate("i18n:ui_plugin_tab_privacy")
		accesses := pluginPrivacyAccesses(plugin.Features)
		if len(accesses) == 0 {
			props.EmptyTitle = c.language.translate("i18n:ui_plugin_no_data_access")
			props.EmptyDescription = c.language.translate("i18n:ui_plugin_no_data_access_subtitle")
			break
		}
		for _, access := range accesses {
			props.Items = append(props.Items, launcherview.PluginMetadataItem{Title: pluginPrivacyTitle(c.language, access), Description: pluginPrivacyDescription(c.language, access)})
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

func pluginPrivacyDescription(language *languageCatalog, access string) string {
	switch access {
	case "requireActiveWindowName":
		return language.translate("i18n:ui_plugin_privacy_window_name_desc")
	case "requireActiveWindowPid":
		return language.translate("i18n:ui_plugin_privacy_window_pid_desc")
	case "requireActiveWindowId":
		return language.translate("i18n:ui_plugin_privacy_window_id_desc")
	case "requireActiveWindowIcon":
		return language.translate("i18n:ui_plugin_privacy_window_icon_desc")
	case "requireActiveWindowIsOpenSaveDialog":
		return language.translate("i18n:ui_plugin_privacy_open_save_dialog_desc")
	case "requireActiveWindowIsOpenSaveDialogSelectFolder":
		return language.translate("i18n:ui_plugin_privacy_open_save_dialog_select_folder_desc")
	case "requireActiveBrowserUrl":
		return language.translate("i18n:ui_plugin_privacy_browser_url_desc")
	case "llm":
		return language.translate("i18n:ui_plugin_privacy_llm_desc")
	default:
		return ""
	}
}

func pluginPrivacyTitle(language *languageCatalog, access string) string {
	switch access {
	case "requireActiveWindowName":
		return language.translate("i18n:ui_plugin_privacy_window_name")
	case "requireActiveWindowPid":
		return language.translate("i18n:ui_plugin_privacy_window_pid")
	case "requireActiveWindowId":
		return language.translate("i18n:ui_plugin_privacy_window_id")
	case "requireActiveWindowIcon":
		return language.translate("i18n:ui_plugin_privacy_window_icon")
	case "requireActiveWindowIsOpenSaveDialog":
		return language.translate("i18n:ui_plugin_privacy_open_save_dialog")
	case "requireActiveWindowIsOpenSaveDialogSelectFolder":
		return language.translate("i18n:ui_plugin_privacy_open_save_dialog_select_folder")
	case "requireActiveBrowserUrl":
		return language.translate("i18n:ui_plugin_privacy_browser_url")
	case "llm":
		return language.translate("i18n:ui_plugin_privacy_llm")
	default:
		return access
	}
}

func (c *pluginSettingsController) pluginStoreDetailProps(snapshot settingsSnapshot, plugin pluginSettingsPlugin, width, imageScale float32) *launcherview.PluginStoreDetailProps {
	plugins := snapshot.plugins
	websiteLabel := ""
	websiteChipLabel := ""
	var onWebsite func()
	var externalIcon *woxui.Image
	var websiteIcon *woxui.Image
	if strings.TrimSpace(plugin.Website) != "" {
		websiteLabel = c.language.translate("i18n:ui_plugin_website")
		websiteChipLabel = websiteLabel
		onWebsite = c.openSelectedPluginWebsite
		iconTint := snapshot.palette.Text
		externalIcon = c.imageForTint(settingControlIconSource("external"), &iconTint, physicalImageSize(13, imageScale))
		websiteIcon = externalIcon
		websiteURL, _ := url.Parse(strings.TrimSpace(plugin.Website))
		if websiteURL != nil && (strings.EqualFold(websiteURL.Hostname(), "github.com") || strings.EqualFold(websiteURL.Hostname(), "gist.github.com") || strings.EqualFold(websiteURL.Hostname(), "www.github.com")) {
			websiteChipLabel = "GitHub"
			websiteIcon = c.imageForTint(pluginMetadataIconSource("github"), &iconTint, physicalImageSize(14, imageScale))
		}
	}
	runtimeLabel := pluginRuntimeLabel(plugin.Runtime)
	var runtimeIcon *woxui.Image
	var onRuntimeHover func(bool, woxui.Rect)
	if runtimeLabel != "" {
		if source := pluginMetadataIconSource(strings.ToLower(plugin.Runtime)); source.ImageData != "" {
			runtimeIcon = c.imageForSurface(source, 256, settingsPalette().Background)
		}
		onRuntimeHover = func(inside bool, anchor woxui.Rect) {
			c.setSettingChoiceTooltip(inside, c.language.translate("i18n:ui_plugin_filter_runtime")+": "+runtimeLabel, anchor)
		}
	}
	var screenshot *woxui.Image
	screenshotLoading := false
	var onScreenshot func()
	if len(plugin.ScreenshotURLs) > 0 {
		source := woxImage{ImageType: "url", ImageData: plugin.ScreenshotURLs[0]}
		// Request a high-res decode from the detail width. Display height is derived later from
		// the description content width so store padding cannot stretch the aspect ratio.
		screenshotWidth := max(float32(1), width)
		requestSize := int(min(float32(2048), max(float32(512), screenshotWidth*2)))
		screenshot = c.imageForSurface(source, requestSize, settingsPalette().Background)
		screenshotLoading = screenshot == nil
		onScreenshot = func() { c.openPreviewImage(source) }
	}
	contentWidth := max(float32(0), width-32)
	metadata := c.pluginMetadataProps(plugin, "privacy")
	props := &launcherview.PluginStoreDetailProps{
		Name: plugin.Name, Version: plugin.Version, Author: plugin.Author, Description: plugin.Description, Runtime: runtimeLabel,
		WebsiteLabel: websiteLabel, WebsiteChipLabel: websiteChipLabel,
		Icon: c.imageForSurface(plugin.Icon, 256, settingsPalette().Background), ExternalIcon: externalIcon, RuntimeIcon: runtimeIcon, WebsiteIcon: websiteIcon,
		OnRuntimeHover: onRuntimeHover,
		FallbackColor:  resultColors[plugins.PluginSelected%len(resultColors)], Management: c.pluginManagementActions(snapshot, plugin),
		ScrollID: "plugin-detail-" + plugin.ID, Metadata: &metadata,
		Keywords:   c.pluginKeywordsFormProps(snapshot, plugin, contentWidth, imageScale, true),
		Commands:   c.pluginCommandsFormProps(snapshot, plugin, contentWidth, imageScale, true, false, nil),
		Screenshot: screenshot, ScreenshotLoading: screenshotLoading, Error: plugins.PluginOperationError, OnWebsite: onWebsite, OnScreenshot: onScreenshot,
	}
	if plugin.IsInstalled && plugin.IsDisable {
		props.DisabledLabel = c.language.translate("i18n:ui_plugin_filter_disabled")
	}
	return props
}

func (c *pluginSettingsController) pluginFilterPanelProps(snapshot settingsSnapshot) *launcherview.PluginFilterPanelProps {
	plugins := snapshot.plugins
	if !plugins.PluginFilterOpen {
		return nil
	}
	fields := c.pluginFilterFields(plugins.PluginFilters, plugins.PluginsStore)
	labelWidth := float32(80)
	if window := c.settings.settingsNativeWindow(); window != nil {
		for _, field := range fields {
			if metrics, err := window.MeasureText(field.Label, woxui.TextStyle{Size: snapshot.palette.Scaled(woxcomponent.SettingsLabelFontSize)}); err == nil {
				labelWidth = max(labelWidth, metrics.Size.Width)
			}
		}
	}
	labelWidth = min(labelWidth, float32(160))
	width := float32(32) + labelWidth + 12 + woxcomponent.SettingsChoiceControlWidth
	if width < 360 {
		width = 360
	}
	return &launcherview.PluginFilterPanelProps{
		Width: width, LabelWidth: labelWidth, Fields: fields,
		ResetLabel: c.language.translate("i18n:ui_plugin_filter_reset"), ResetEnabled: plugins.PluginFilters.applied(plugins.PluginsStore),
		Theme: snapshot.palette, OnOpen: c.openPluginFilterChoice, OnReset: c.resetPluginFilters, OnDismiss: c.closePluginFilterPanel,
	}
}

// pluginFilterFields builds the exclusive dropdown rows for the current catalog.
func (c *pluginSettingsController) pluginFilterFields(filters pluginFilterState, store bool) []launcherview.PluginFilterField {
	fields := make([]launcherview.PluginFilterField, 0, 4)
	if store {
		fields = append(fields, launcherview.PluginFilterField{
			ID: "install", Label: c.language.translate("i18n:ui_plugin_filter_install_status"), Value: c.pluginFilterChoiceLabel("install", filters.installStatus),
		})
	} else {
		fields = append(fields,
			launcherview.PluginFilterField{ID: "enabled", Label: c.language.translate("i18n:ui_plugin_filter_enabled_status"), Value: c.pluginFilterChoiceLabel("enabled", filters.enabledStatus)},
			launcherview.PluginFilterField{ID: "upgrade", Label: c.language.translate("i18n:ui_plugin_filter_upgrade_status"), Value: c.pluginFilterChoiceLabel("upgrade", filters.upgradeStatus)},
		)
	}
	fields = append(fields,
		launcherview.PluginFilterField{ID: "type", Label: c.language.translate("i18n:ui_plugin_filter_plugin_type"), Value: c.pluginFilterChoiceLabel("type", filters.pluginType)},
		launcherview.PluginFilterField{ID: "runtime", Label: c.language.translate("i18n:ui_plugin_filter_runtime"), Value: c.pluginFilterChoiceLabel("runtime", filters.runtime)},
	)
	return fields
}

// pluginFilterChoiceItem adapts one catalog filter dropdown to the shared settings picker.
func (c *pluginSettingsController) pluginFilterChoiceItem(id string) settingItem {
	filters := c.Filters()
	var value string
	var title string
	switch id {
	case "enabled":
		title, value = c.language.translate("i18n:ui_plugin_filter_enabled_status"), filters.enabledStatus
	case "upgrade":
		title, value = c.language.translate("i18n:ui_plugin_filter_upgrade_status"), filters.upgradeStatus
	case "type":
		title, value = c.language.translate("i18n:ui_plugin_filter_plugin_type"), filters.pluginType
	case "runtime":
		title, value = c.language.translate("i18n:ui_plugin_filter_runtime"), filters.runtime
	case "install":
		title, value = c.language.translate("i18n:ui_plugin_filter_install_status"), filters.installStatus
	default:
		return settingItem{}
	}
	if pluginFilterIsAll(value) {
		value = pluginFilterAll
	}
	return settingItem{key: "plugin-filter-" + id, title: title, value: value, choices: c.pluginFilterChoices(id)}
}

// pluginFilterChoiceLabel returns the visible value for one exclusive filter dropdown.
func (c *pluginSettingsController) pluginFilterChoiceLabel(id, value string) string {
	if pluginFilterIsAll(value) {
		value = pluginFilterAll
	}
	for _, choice := range c.pluginFilterChoices(id) {
		if choice.value == value {
			return choice.label
		}
	}
	return c.language.translate("i18n:ui_all")
}

// pluginFilterChoices lists the exclusive options for one catalog filter dropdown.
func (c *pluginSettingsController) pluginFilterChoices(id string) []settingChoice {
	all := settingChoice{value: pluginFilterAll, label: c.language.translate("i18n:ui_all")}
	switch id {
	case "enabled":
		return []settingChoice{all, {value: pluginFilterEnabled, label: c.language.translate("i18n:ui_plugin_filter_enabled")}, {value: pluginFilterDisabled, label: c.language.translate("i18n:ui_plugin_filter_disabled")}}
	case "upgrade":
		return []settingChoice{all, {value: pluginFilterUpgradable, label: c.language.translate("i18n:ui_plugin_filter_upgradable")}, {value: pluginFilterNotUpgradable, label: c.language.translate("i18n:ui_plugin_filter_not_upgradable")}}
	case "type":
		return []settingChoice{all, {value: pluginFilterSystem, label: c.language.translate("i18n:ui_plugin_filter_system")}, {value: pluginFilterThirdParty, label: c.language.translate("i18n:ui_plugin_filter_third_party")}}
	case "install":
		return []settingChoice{all, {value: pluginFilterInstalled, label: c.language.translate("i18n:ui_plugin_filter_installed")}, {value: pluginFilterUninstalled, label: c.language.translate("i18n:ui_not_installed")}}
	case "runtime":
		choices := []settingChoice{
			all,
			{value: pluginFilterRuntimeNodeJS, label: c.language.translate("i18n:ui_runtime_name_nodejs")},
			{value: pluginFilterRuntimePython, label: c.language.translate("i18n:ui_runtime_name_python")},
			{value: pluginFilterRuntimeScript, label: c.language.translate("i18n:ui_runtime_name_script")},
		}
		for _, runtime := range thirdPartyRuntimeFilterValues() {
			choices = append(choices, settingChoice{value: runtime, label: c.localizedRuntimeDisplayName(runtime)})
		}
		return choices
	default:
		return nil
	}
}

// pluginRuntimeLabel normalizes host runtimes for the detail chip. Wox and Flow
// prefixes keep the two plugin families distinct. Go is omitted because it is
// the native plugin host and the tag adds no useful distinction.
func pluginRuntimeLabel(runtime string) string {
	switch strings.ToLower(strings.TrimSpace(runtime)) {
	case "nodejs":
		return "Wox Node.js"
	case "python":
		return "Wox Python"
	case "script":
		return "Wox Script"
	case "flowjsonrpc":
		return "Flow JSON-RPC"
	case "flowdotnet":
		return "Flow .NET"
	case "go":
		return ""
	default:
		return runtime
	}
}

// pluginManagementActions shares install, upgrade, and uninstall actions between plugin details.
func (c *pluginSettingsController) pluginManagementActions(snapshot settingsSnapshot, plugin pluginSettingsPlugin) []launcherview.PluginAction {
	plugins := snapshot.plugins
	busy := plugins.PluginOperation != ""
	if !plugin.IsInstalled {
		return []launcherview.PluginAction{{
			ID: "plugin-install", Label: pluginOperationButtonLabel(plugins, "install", plugin.ID, c.language.translate("i18n:ui_plugin_install")), Width: 104,
			Enabled: !busy, Primary: true, OnTap: func() { c.runPluginOperation("install") },
		}}
	}
	actions := make([]launcherview.PluginAction, 0, 3)
	if plugin.IsUpgradable {
		actions = append(actions, launcherview.PluginAction{ID: "plugin-upgrade", Label: pluginOperationButtonLabel(plugins, "upgrade", plugin.ID, c.language.translate("i18n:ui_update")), Width: 104, Enabled: !busy, Primary: true, OnTap: func() { c.runPluginOperation("upgrade") }})
	}
	if !plugin.IsSystem {
		label := c.language.translate("i18n:ui_plugin_uninstall")
		if plugins.PluginUninstallArmed == plugin.ID {
			label = c.language.translate("i18n:ui_cloud_sync_confirm") + " " + label
		}
		actions = append(actions, launcherview.PluginAction{ID: "plugin-uninstall", Label: pluginOperationButtonLabel(plugins, "uninstall", plugin.ID, label), Width: 124, Enabled: !busy, OnTap: func() { c.runPluginOperation("uninstall") }})
	}
	if plugin.IsDisable {
		actions = append(actions, launcherview.PluginAction{ID: "plugin-enable", Label: pluginOperationButtonLabel(plugins, "enable", plugin.ID, c.language.translate("i18n:ui_plugin_enable")), Width: 96, Enabled: !busy, OnTap: func() { c.runPluginOperation("enable") }})
	} else {
		actions = append(actions, launcherview.PluginAction{ID: "plugin-disable", Label: pluginOperationButtonLabel(plugins, "disable", plugin.ID, c.language.translate("i18n:ui_plugin_disable")), Width: 96, Enabled: !busy, OnTap: func() { c.runPluginOperation("disable") }})
	}
	if !plugin.IsSystem && strings.TrimSpace(plugin.PluginDirectory) != "" {
		label := c.language.translate("i18n:ui_plugin_open_directory")
		width := float32(132)
		if woxplugin.IsSingleFilePluginDirectory(plugin.PluginDirectory) {
			label = c.language.translate("i18n:ui_plugin_locate_file")
			width = 160
		}
		actions = append(actions, launcherview.PluginAction{
			ID: "plugin-directory", Label: label, Width: width, Enabled: !busy, OnTap: c.openSelectedPluginDirectory,
		})
	}
	if network.IsOffline() && !plugin.IsSystem {
		for index := range actions {
			if actions[index].ID == "plugin-enable" || actions[index].ID == "plugin-disable" || actions[index].ID == "plugin-upgrade" {
				actions[index].Enabled = false
				actions[index].Label = c.language.translate("i18n:ui_plugin_offline_paused")
				actions[index].OnTap = nil
			}
		}
	}

	return actions
}

// pluginMetadataActions exposes the browser action without platform-specific widgets.
func (c *pluginSettingsController) pluginMetadataActions(snapshot settingsSnapshot, plugin pluginSettingsPlugin, imageScale float32) []launcherview.PluginAction {
	actions := make([]launcherview.PluginAction, 0, 1)
	if strings.TrimSpace(plugin.Website) != "" {
		iconTint := snapshot.palette.Text
		actions = append(actions, launcherview.PluginAction{
			ID: "plugin-website", Label: c.language.translate("i18n:ui_plugin_website"), Icon: c.imageForTint(settingControlIconSource("external"), &iconTint, physicalImageSize(14, imageScale)),
			Width: 88, Enabled: true, OnTap: c.openSelectedPluginWebsite,
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
