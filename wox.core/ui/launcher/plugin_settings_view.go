package launcher

import (
	"fmt"
	"sort"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildPluginSettingsPage lays out the shared installed/store catalog and definition editor.
func (a *App) buildPluginSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	innerHeight := max(float32(0), height-24)
	listWidth := min(float32(300), max(float32(250), innerWidth*0.31))
	detailWidth := max(float32(0), innerWidth-listWidth-1)
	content := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		a.buildInstalledPluginList(snapshot, listWidth, innerHeight),
		woxwidget.Container{Width: 1, Height: innerHeight, Color: snapshot.palette.previewSplit},
		a.buildPluginSettingsEditor(snapshot, detailWidth, innerHeight),
	}}
	return woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 12}, Child: content,
	}
}

func (a *App) buildInstalledPluginList(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	headerHeight := float32(58)
	viewportHeight := max(float32(0), height-headerHeight)
	a.setPluginListViewport(viewportHeight)
	if snapshot.pluginsLoading && len(snapshot.plugins) == 0 {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(16), Child: woxwidget.Text{
			Value: a.translate("i18n:ui_cloud_sync_plugin_exclusions_loading"), Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle,
		}}
	}
	if snapshot.pluginsError != "" && len(snapshot.plugins) == 0 {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(16), Child: woxwidget.TextBlock{
			Value: snapshot.pluginsError, Width: max(float32(0), width-32), Height: max(float32(0), height-32), Style: woxui.TextStyle{Size: 12}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255},
		}}
	}
	filtered := filterPlugins(snapshot.plugins, snapshot.pluginSearch.Text)
	rows := make([]woxwidget.Widget, 0, len(filtered))
	for visibleIndex, entry := range filtered {
		index := entry.index
		plugin := entry.plugin
		background := woxui.Color{}
		titleColor := snapshot.palette.resultTitle
		if index == snapshot.pluginSelected {
			background = snapshot.palette.selectedBackground
			titleColor = snapshot.palette.selectedTitle
		}
		var icon woxwidget.Widget = woxwidget.Container{Width: 36, Height: 36, Radius: 8, Color: resultColors[visibleIndex%len(resultColors)]}
		if image := a.imageFor(plugin.Icon); image != nil {
			icon = woxwidget.Image{Source: image, Width: 36, Height: 36}
		}
		status := strings.TrimSpace(plugin.Version + "  " + plugin.Author)
		if snapshot.pluginsStore && !plugin.IsInstalled {
			status = a.translate("i18n:ui_cloud_sync_key_available") + "  " + status
		} else if plugin.IsUpgradable {
			status = a.translate("i18n:ui_update") + "  " + status
		} else if plugin.IsDisable {
			status = a.translate("i18n:ui_disabled") + "  " + status
		}
		badgeLabel := ""
		if plugin.IsSystem {
			badgeLabel = a.translate("i18n:ui_setting_plugin_system_tag")
		} else if plugin.IsDev {
			badgeLabel = a.translate("i18n:ui_plugin_dev_tag")
		} else if strings.EqualFold(plugin.Runtime, "script") {
			badgeLabel = a.translate("i18n:ui_setting_plugin_script_tag")
		}
		textWidth := max(float32(0), width-80)
		var badge woxwidget.Widget
		if badgeLabel != "" {
			textWidth = max(float32(0), width-134)
			badgeColor := snapshot.palette.toolbarBackground
			badgeColor.A = 210
			badge = woxwidget.Container{Width: 44, Height: 22, Radius: 5, Color: badgeColor, Padding: woxwidget.Insets{Left: 7, Top: 4}, Child: woxwidget.Text{Value: badgeLabel, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle}}
		}
		rowChildren := []woxwidget.Widget{
			icon,
			woxwidget.Container{Width: textWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
				woxwidget.Text{Value: plugin.Name, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: titleColor},
				woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle},
			}}},
		}
		if badge != nil {
			rowChildren = append(rowChildren, badge)
		}
		rows = append(rows, woxwidget.Gesture{
			ID:    "plugin-list-" + plugin.ID,
			OnTap: func() { a.selectPlugin(index) },
			Child: woxwidget.Container{Width: width - 16, Height: pluginSettingsListRowHeight, Radius: 6, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 9, Right: 8, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: rowChildren}},
		})
	}
	contentHeight := max(viewportHeight, float32(len(rows))*pluginSettingsListRowHeight)
	var list woxwidget.Widget = woxwidget.Gesture{
		ID: "plugin-list-scroll",
		OnScroll: func(delta woxui.Point) {
			a.scrollPluginList(-delta.Y)
		},
		Child: woxwidget.ScrollView{
			Width: width - 16, Height: viewportHeight, ContentHeight: contentHeight, Offset: snapshot.pluginListScroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		},
	}
	searchStyle := woxui.TextStyle{Size: 13}
	searchWidth := max(float32(40), width-70)
	search := woxwidget.Gesture{ID: "plugin-search", OnTapAt: func(position woxui.Point) {
		offset := formTextOffsetAt(snapshot.pluginSearch, a.settingsNativeWindow(), searchStyle, 1, searchWidth-8, woxui.Point{X: max(float32(0), position.X-2), Y: position.Y})
		a.focusPluginSearch(offset)
	}, Child: woxwidget.Painter{Width: searchWidth, Height: 40, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		if snapshot.pluginSearch.Text == "" {
			displayList.DrawText(fmt.Sprintf(a.translate("i18n:ui_search_plugins"), len(snapshot.plugins)), bounds, searchStyle, snapshot.palette.resultSubtitle)
		}
		drawFormEditor(displayList, bounds, snapshot.pluginSearch, searchStyle, snapshot.palette, snapshot.pluginSearchFocused, 1, a.settingsNativeWindow())
	}}}
	searchChildren := []woxwidget.Widget{
		woxwidget.Container{Width: 30, Height: 42, Padding: woxwidget.Insets{Left: 9, Top: 11}, Child: woxwidget.Text{Value: "⌕", Style: woxui.TextStyle{Size: 17}, Color: snapshot.palette.resultSubtitle}},
		search,
	}
	if snapshot.pluginSearch.Text != "" {
		searchChildren = append(searchChildren, woxwidget.Gesture{ID: "plugin-search-clear", OnTap: a.clearPluginSearch, Child: woxwidget.Container{Width: 24, Height: 42, Padding: woxwidget.Insets{Left: 5, Top: 10}, Child: woxwidget.Text{Value: "×", Style: woxui.TextStyle{Size: 16}, Color: snapshot.palette.resultSubtitle}}})
	}
	searchField := woxwidget.Container{Width: width - 16, Height: 44, Radius: 6, Color: snapshot.palette.queryBackground, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: searchChildren}}
	if len(rows) == 0 && !snapshot.pluginsLoading && snapshot.pluginsError == "" {
		list = woxwidget.Container{Width: width - 16, Height: viewportHeight, Padding: woxwidget.Insets{Left: 10, Top: 18}, Child: woxwidget.Text{Value: a.translate("i18n:ui_setting_plugin_empty_data"), Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.resultSubtitle}}
	}
	return woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 8, Right: 8},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 14, Children: []woxwidget.Widget{
			searchField,
			list,
		}},
	}
}

// buildPluginSettingsEditor renders the selected plugin with the same fields used by query forms.
func (a *App) buildPluginSettingsEditor(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	if snapshot.pluginSelected < 0 || snapshot.pluginSelected >= len(snapshot.plugins) {
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.UniformInsets(24), Child: woxwidget.Text{
			Value: a.translate("i18n:ui_setting_plugin_empty_data"), Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle,
		}}
	}
	plugin := snapshot.plugins[snapshot.pluginSelected]
	if snapshot.pluginForm == nil {
		return a.buildPluginStoreDetail(snapshot, plugin, width, height)
	}
	form := snapshot.pluginForm
	innerWidth := max(float32(0), width-48)
	innerHeight := max(float32(0), height-24)
	headerHeight := float32(104)
	tabHeight := float32(46)
	footerHeight := float32(48)
	detailTab := snapshot.pluginDetailTab
	if detailTab == "" {
		detailTab = "settings"
	}
	header := a.buildPluginDetailHeader(snapshot, plugin, innerWidth, headerHeight)
	tabs := a.buildPluginDetailTabs(snapshot, detailTab, innerWidth, tabHeight)
	if detailTab != "settings" {
		bodyHeight := max(float32(0), innerHeight-headerHeight-tabHeight)
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 24, Top: 12, Right: 24, Bottom: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			header,
			tabs,
			a.buildPluginMetadataTab(snapshot, plugin, detailTab, innerWidth, bodyHeight),
		}}}
	}
	statusHeight := float32(0)
	statusText := form.status
	statusError := form.statusError
	if snapshot.pluginOperationError != "" {
		statusText = snapshot.pluginOperationError
		statusError = true
	}
	if strings.TrimSpace(statusText) != "" {
		statusHeight = 28
	}
	bodyHeight := max(float32(48), innerHeight-headerHeight-tabHeight-footerHeight-statusHeight)
	a.setPluginFormViewport(bodyHeight)
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
	contentHeight := max(bodyHeight, formDefinitionsContentHeight(form.definitions))
	body := woxwidget.Gesture{
		ID: "plugin-settings-scroll",
		OnScroll: func(delta woxui.Point) {
			a.scrollPluginForm(-delta.Y)
		},
		Child: woxwidget.ScrollView{
			Width: innerWidth, Height: bodyHeight, ContentHeight: contentHeight, Offset: form.scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		},
	}
	statusColor := snapshot.palette.resultSubtitle
	if statusError {
		statusColor = woxui.Color{R: 232, G: 95, B: 95, A: 255}
	}
	children := []woxwidget.Widget{header, tabs, body}
	if statusHeight > 0 {
		children = append(children, woxwidget.Container{Width: innerWidth, Height: statusHeight, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{
			Value: statusText, Style: woxui.TextStyle{Size: 11}, Color: statusColor,
		}})
	}
	saveLabel := a.translate("i18n:ui_save")
	if form.saving {
		saveLabel += "…"
	}
	buttonColor := snapshot.palette.selectedBackground
	if form.dirty && !form.saving {
		buttonColor = snapshot.palette.actionSelected
	}
	footerChildren := []woxwidget.Widget{woxwidget.Painter{Width: max(float32(0), innerWidth-128), Height: footerHeight}}
	footerChildren = append(footerChildren, woxwidget.Gesture{ID: "plugin-settings-save", OnTap: a.submitPluginSettings, Child: woxwidget.Container{
		Width: 128, Height: 36, Radius: 8, Color: buttonColor, Padding: woxwidget.Insets{Left: 24, Top: 10, Right: 20},
		Child: woxwidget.Text{Value: saveLabel, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.actionSelectedText},
	}})
	footer := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: footerChildren}
	children = append(children, footer)
	return woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 24, Top: 12, Right: 24, Bottom: 12},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
	}
}

func (a *App) buildPluginDetailHeader(snapshot settingsSnapshot, plugin pluginSettingsPlugin, width, height float32) woxwidget.Widget {
	var icon woxwidget.Widget = woxwidget.Container{Width: 44, Height: 44, Radius: 10, Color: resultColors[snapshot.pluginSelected%len(resultColors)]}
	if image := a.imageFor(plugin.Icon); image != nil {
		icon = woxwidget.Image{Source: image, Width: 44, Height: 44}
	}
	actionsWidth := float32(224)
	identityWidth := max(float32(120), width-44-14-actionsWidth)
	identity := woxwidget.Container{Width: identityWidth, Height: 58, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
		woxwidget.Text{Value: strings.TrimSpace(plugin.Name + "  " + plugin.Version), Style: woxui.TextStyle{Size: 19, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
		woxwidget.Text{Value: plugin.Author, Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.resultSubtitle},
	}}}
	management, _ := a.buildPluginManagementButtons(snapshot, plugin)
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: []woxwidget.Widget{
			icon,
			identity,
			woxwidget.Container{Width: actionsWidth, Height: 58, Padding: woxwidget.Insets{Top: 4}, Child: a.buildPluginMetadataActions(snapshot, plugin)},
		}},
		woxwidget.Container{Width: width, Height: 46, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: management}},
	}}}
}

func (a *App) buildPluginDetailTabs(snapshot settingsSnapshot, active string, width, height float32) woxwidget.Widget {
	tabs := []struct {
		id    string
		label string
		width float32
	}{
		{"settings", a.translate("i18n:ui_plugin_tab_settings"), 82},
		{"description", a.translate("i18n:ui_plugin_tab_description"), 92},
		{"keywords", a.translate("i18n:ui_plugin_tab_trigger_keywords"), 126},
		{"commands", a.translate("i18n:ui_plugin_tab_commands"), 88},
		{"privacy", a.translate("i18n:ui_plugin_tab_privacy"), 76},
	}
	children := make([]woxwidget.Widget, 0, len(tabs))
	for _, tab := range tabs {
		tab := tab
		underline := woxui.Color{}
		color := snapshot.palette.resultSubtitle
		if tab.id == active {
			underline = snapshot.palette.cursor
			color = snapshot.palette.queryText
		}
		children = append(children, woxwidget.Gesture{ID: "plugin-detail-tab-" + tab.id, OnTap: func() { a.selectPluginDetailTab(tab.id) }, Child: woxwidget.Container{Width: tab.width, Height: height - 1, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Container{Width: tab.width, Height: height - 3, Padding: woxwidget.Insets{Top: 13}, Child: woxwidget.Text{Value: tab.label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: color}},
			woxwidget.Container{Width: tab.width, Height: 2, Color: underline},
		}}}})
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children},
		woxwidget.Container{Width: width, Height: 1, Color: snapshot.palette.previewSplit},
	}}}
}

// buildPluginMetadataTab restores Flutter's non-editing plugin detail tabs from metadata already returned by core.
func (a *App) buildPluginMetadataTab(snapshot settingsSnapshot, plugin pluginSettingsPlugin, tab string, width, height float32) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0)
	switch tab {
	case "description":
		rows = append(rows, woxwidget.TextBlock{Value: plugin.Description, Width: width, Height: max(float32(100), height-30), MaxLines: 20, Style: woxui.TextStyle{Size: 13}, LineHeight: 21, Color: snapshot.palette.resultSubtitle})
	case "keywords":
		if len(plugin.TriggerKeywords) == 0 {
			rows = append(rows, a.pluginDetailEmptyState(snapshot, a.translate("i18n:ui_plugin_no_trigger_keywords"), width))
		} else {
			for _, keyword := range plugin.TriggerKeywords {
				rows = append(rows, a.buildPluginMetadataRow(snapshot, keyword, a.translate("i18n:ui_plugin_trigger_keywords_tip"), width))
			}
		}
	case "commands":
		if len(plugin.Commands) == 0 {
			rows = append(rows, a.pluginDetailEmptyState(snapshot, a.translate("i18n:ui_plugin_no_commands"), width))
		} else {
			rows = append(rows, a.buildPluginMetadataRow(snapshot, a.translate("i18n:ui_plugin_command_name_column"), a.translate("i18n:ui_plugin_command_desc_column"), width))
			for _, command := range plugin.Commands {
				rows = append(rows, a.buildPluginMetadataRow(snapshot, command.Command, command.Description, width))
			}
		}
	case "privacy":
		accesses := pluginPrivacyAccesses(plugin.Features)
		if len(accesses) == 0 {
			rows = append(rows, a.pluginDetailEmptyState(snapshot, a.translate("i18n:ui_plugin_no_data_access"), width))
		} else {
			rows = append(rows, woxwidget.Container{Width: width, Height: 46, Padding: woxwidget.Insets{Top: 16}, Child: woxwidget.Text{Value: a.translate("i18n:ui_plugin_data_access_title"), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle}})
			for _, access := range accesses {
				rows = append(rows, a.buildPluginMetadataRow(snapshot, pluginPrivacyTitle(a, access), pluginPrivacyDescription(a, access), width))
			}
		}
	}
	contentHeight := max(height-24, float32(len(rows))*62)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 18}, Child: woxwidget.ScrollView{Width: width, Height: max(float32(1), height-18), ContentHeight: contentHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}}
}

func (a *App) buildPluginMetadataRow(snapshot settingsSnapshot, title, description string, width float32) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: width * 0.32, Height: 61, Padding: woxwidget.Insets{Left: 8, Top: 18, Right: 8}, Child: woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle}},
			woxwidget.Container{Width: width * 0.68, Height: 61, Padding: woxwidget.Insets{Left: 8, Top: 18, Right: 8}, Child: woxwidget.Text{Value: description, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle}},
		}},
		woxwidget.Container{Width: width, Height: 1, Color: snapshot.palette.previewSplit},
	}}}
}

func (a *App) pluginDetailEmptyState(snapshot settingsSnapshot, message string, width float32) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: 100, Padding: woxwidget.Insets{Top: 26}, Child: woxwidget.Text{Value: message, Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle}}
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
	accesses = append(accesses, unknown...)
	return accesses
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

// buildPluginStoreDetail renders the same metadata card before a store plugin has settings to edit.
func (a *App) buildPluginStoreDetail(snapshot settingsSnapshot, plugin pluginSettingsPlugin, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-36)
	var icon woxwidget.Widget = woxwidget.Container{Width: 54, Height: 54, Radius: 12, Color: resultColors[snapshot.pluginSelected%len(resultColors)]}
	if image := a.imageFor(plugin.Icon); image != nil {
		icon = woxwidget.Image{Source: image, Width: 54, Height: 54}
	}
	management, _ := a.buildPluginManagementButtons(snapshot, plugin)
	errorText := snapshot.pluginOperationError
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.actionBackground, Padding: woxwidget.UniformInsets(18), Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 16, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: []woxwidget.Widget{
				icon,
				woxwidget.Container{Width: max(float32(0), innerWidth-68), Height: 60, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
					woxwidget.Text{Value: plugin.Name, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
					woxwidget.Text{Value: plugin.Author + " · " + plugin.Version + " · " + plugin.Runtime, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle},
				}}},
			}},
			woxwidget.TextBlock{Value: plugin.Description, Width: innerWidth, Height: 120, MaxLines: 6, Style: woxui.TextStyle{Size: 12}, LineHeight: 19, Color: snapshot.palette.resultSubtitle},
			a.buildPluginMetadataActions(snapshot, plugin),
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: management},
			woxwidget.TextBlock{Value: errorText, Width: innerWidth, Height: 60, MaxLines: 3, Style: woxui.TextStyle{Size: 11}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255}},
		},
	}}
}

// buildPluginManagementButtons shares install, upgrade, and uninstall actions between store and settings details.
func (a *App) buildPluginManagementButtons(snapshot settingsSnapshot, plugin pluginSettingsPlugin) ([]woxwidget.Widget, float32) {
	busy := snapshot.pluginOperation != ""
	buttons := []woxwidget.Widget{}
	width := float32(0)
	if !plugin.IsInstalled {
		buttons = append(buttons, a.buildFormTableButton("plugin-install", pluginOperationButtonLabel(snapshot, "install", plugin.ID, a.translate("i18n:ui_plugin_install")), 104, !busy, true, func() { a.runPluginOperation("install") }, snapshot.palette))
		width += 104
		return buttons, width
	}
	if plugin.IsUpgradable {
		buttons = append(buttons, a.buildFormTableButton("plugin-upgrade", pluginOperationButtonLabel(snapshot, "upgrade", plugin.ID, a.translate("i18n:ui_update")), 104, !busy, true, func() { a.runPluginOperation("upgrade") }, snapshot.palette))
		width += 104
	}
	if plugin.IsDisable {
		buttons = append(buttons, a.buildFormTableButton("plugin-enable", pluginOperationButtonLabel(snapshot, "enable", plugin.ID, a.translate("i18n:ui_plugin_enable")), 96, !busy, false, func() { a.runPluginOperation("enable") }, snapshot.palette))
		width += 96
	} else {
		buttons = append(buttons, a.buildFormTableButton("plugin-disable", pluginOperationButtonLabel(snapshot, "disable", plugin.ID, a.translate("i18n:ui_plugin_disable")), 96, !busy, false, func() { a.runPluginOperation("disable") }, snapshot.palette))
		width += 96
	}
	if !plugin.IsSystem {
		label := a.translate("i18n:ui_plugin_uninstall")
		if snapshot.pluginUninstallArmed == plugin.ID {
			label = a.translate("i18n:ui_cloud_sync_confirm") + " " + label
		}
		buttons = append(buttons, a.buildFormTableButton("plugin-uninstall", pluginOperationButtonLabel(snapshot, "uninstall", plugin.ID, label), 124, !busy, false, func() { a.runPluginOperation("uninstall") }, snapshot.palette))
		width += 124
	}
	return buttons, width
}

// buildPluginMetadataActions exposes browser and folder actions without platform-specific widget code.
func (a *App) buildPluginMetadataActions(snapshot settingsSnapshot, plugin pluginSettingsPlugin) woxwidget.Widget {
	buttons := make([]woxwidget.Widget, 0, 2)
	if strings.TrimSpace(plugin.Website) != "" {
		buttons = append(buttons, a.buildFormTableButton("plugin-website", a.translate("i18n:ui_plugin_website"), 104, true, false, a.openSelectedPluginWebsite, snapshot.palette))
	}
	if plugin.IsInstalled && strings.TrimSpace(plugin.PluginDirectory) != "" {
		buttons = append(buttons, a.buildFormTableButton("plugin-directory", a.translate("i18n:ui_plugin_open_directory"), 112, true, false, a.openSelectedPluginDirectory, snapshot.palette))
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}
}

func pluginOperationButtonLabel(snapshot settingsSnapshot, kind, pluginID, idle string) string {
	if snapshot.pluginOperation == kind+":"+pluginID {
		return idle + "…"
	}
	return idle
}
