package launcher

import (
	"fmt"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

// buildPluginSettingsPage lays out the shared installed/store catalog and definition editor.
func (a *App) buildPluginSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-48)
	innerHeight := max(float32(0), height-48)
	headerHeight := float32(64)
	bodyHeight := max(float32(0), innerHeight-headerHeight)
	listWidth := min(float32(300), max(float32(240), innerWidth*0.32))
	detailWidth := max(float32(0), innerWidth-listWidth-16)
	modeLabel := "Installed plugins and their core-backed settings"
	if snapshot.pluginsStore {
		modeLabel = "Browse, install, and upgrade plugins from the Wox store"
	}
	header := woxwidget.Container{Width: innerWidth, Height: headerHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), innerWidth-212), Height: headerHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "Plugins", Style: woxui.TextStyle{Size: 24, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
			woxwidget.Text{Value: modeLabel, Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle},
		}}},
		a.buildFormTableButton("plugins-mode-installed", "Installed", 98, !snapshot.pluginsLoading && snapshot.pluginOperation == "", !snapshot.pluginsStore, func() { a.switchPluginList(false) }, snapshot.palette),
		a.buildFormTableButton("plugins-mode-store", "Store", 98, !snapshot.pluginsLoading && snapshot.pluginOperation == "", snapshot.pluginsStore, func() { a.switchPluginList(true) }, snapshot.palette),
	}}}
	content := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: []woxwidget.Widget{
		a.buildInstalledPluginList(snapshot, listWidth, bodyHeight),
		a.buildPluginSettingsEditor(snapshot, detailWidth, bodyHeight),
	}}
	return woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 24, Top: 24, Right: 24, Bottom: 24},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, content}},
	}
}

func (a *App) buildInstalledPluginList(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	headerHeight := float32(42)
	viewportHeight := max(float32(0), height-headerHeight-20)
	a.setPluginListViewport(viewportHeight)
	if snapshot.pluginsLoading && len(snapshot.plugins) == 0 {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(16), Child: woxwidget.Text{
			Value: "Loading plugins…", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle,
		}}
	}
	if snapshot.pluginsError != "" && len(snapshot.plugins) == 0 {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(16), Child: woxwidget.TextBlock{
			Value: snapshot.pluginsError, Width: max(float32(0), width-32), Height: max(float32(0), height-32), Style: woxui.TextStyle{Size: 12}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255},
		}}
	}
	rows := make([]woxwidget.Widget, 0, len(snapshot.plugins))
	for index, plugin := range snapshot.plugins {
		index := index
		plugin := plugin
		background := woxui.Color{}
		titleColor := snapshot.palette.resultTitle
		if index == snapshot.pluginSelected {
			background = snapshot.palette.selectedBackground
			titleColor = snapshot.palette.selectedTitle
		}
		var icon woxwidget.Widget = woxwidget.Container{Width: 34, Height: 34, Radius: 8, Color: resultColors[index%len(resultColors)]}
		if image := a.imageFor(plugin.Icon); image != nil {
			icon = woxwidget.Image{Source: image, Width: 34, Height: 34}
		}
		status := plugin.Runtime + " · " + plugin.Version
		if snapshot.pluginsStore && !plugin.IsInstalled {
			status = "Available · " + status
		} else if plugin.IsUpgradable {
			status = "Upgrade available · " + status
		} else if plugin.IsDisable {
			status = "Disabled · " + status
		}
		rows = append(rows, woxwidget.Gesture{
			ID:    "plugin-list-" + plugin.ID,
			OnTap: func() { a.selectPlugin(index) },
			Child: woxwidget.Container{Width: width - 16, Height: pluginSettingsListRowHeight, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 10, Right: 8, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
				icon,
				woxwidget.Container{Width: max(float32(0), width-78), Height: 42, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
					woxwidget.Text{Value: plugin.Name, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: titleColor},
					woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle},
				}}},
			}}},
		})
	}
	contentHeight := max(viewportHeight, float32(len(rows))*pluginSettingsListRowHeight)
	listLabel := "Installed"
	if snapshot.pluginsStore {
		listLabel = "Store"
	}
	list := woxwidget.Gesture{
		ID: "plugin-list-scroll",
		OnScroll: func(delta woxui.Point) {
			a.scrollPluginList(-delta.Y)
		},
		Child: woxwidget.ScrollView{
			Width: width - 16, Height: viewportHeight, ContentHeight: contentHeight, Offset: snapshot.pluginListScroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		},
	}
	return woxwidget.Container{
		Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 8, Top: 8, Right: 8, Bottom: 8},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Container{Width: width - 16, Height: headerHeight, Padding: woxwidget.Insets{Left: 10, Top: 10}, Child: woxwidget.Text{
				Value: fmt.Sprintf("%s · %d", listLabel, len(snapshot.plugins)), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle,
			}},
			list,
		}},
	}
}

// buildPluginSettingsEditor renders the selected plugin with the same fields used by query forms.
func (a *App) buildPluginSettingsEditor(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	if snapshot.pluginSelected < 0 || snapshot.pluginSelected >= len(snapshot.plugins) {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(18), Child: woxwidget.Text{
			Value: "No plugin selected", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle,
		}}
	}
	plugin := snapshot.plugins[snapshot.pluginSelected]
	if snapshot.pluginForm == nil {
		return a.buildPluginStoreDetail(snapshot, plugin, width, height)
	}
	form := snapshot.pluginForm
	innerWidth := max(float32(0), width-32)
	headerHeight := float32(78)
	footerHeight := float32(86)
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
	bodyHeight := max(float32(48), height-32-headerHeight-footerHeight-statusHeight)
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
	var icon woxwidget.Widget = woxwidget.Container{Width: 44, Height: 44, Radius: 10, Color: resultColors[snapshot.pluginSelected%len(resultColors)]}
	if image := a.imageFor(plugin.Icon); image != nil {
		icon = woxwidget.Image{Source: image, Width: 44, Height: 44}
	}
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: []woxwidget.Widget{
		icon,
		woxwidget.Container{Width: max(float32(0), innerWidth-58), Height: headerHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Text{Value: plugin.Name, Style: woxui.TextStyle{Size: 19, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
			woxwidget.Text{Value: plugin.Description, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle},
		}}},
	}}
	statusColor := snapshot.palette.resultSubtitle
	if statusError {
		statusColor = woxui.Color{R: 232, G: 95, B: 95, A: 255}
	}
	children := []woxwidget.Widget{header, body}
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
	management, managementWidth := a.buildPluginManagementButtons(snapshot, plugin)
	footerChildren := append([]woxwidget.Widget(nil), management...)
	footerChildren = append(footerChildren, woxwidget.Painter{Width: max(float32(0), innerWidth-managementWidth-128-float32(len(management)+1)*8), Height: footerHeight})
	footerChildren = append(footerChildren, woxwidget.Gesture{ID: "plugin-settings-save", OnTap: a.submitPluginSettings, Child: woxwidget.Container{
		Width: 128, Height: 36, Radius: 8, Color: buttonColor, Padding: woxwidget.Insets{Left: 24, Top: 10, Right: 20},
		Child: woxwidget.Text{Value: saveLabel, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.actionSelectedText},
	}})
	footer := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
		a.buildPluginMetadataActions(snapshot, plugin),
		woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: footerChildren},
	}}
	children = append(children, footer)
	return woxwidget.Container{
		Width: width, Height: height, Radius: 10, Color: snapshot.palette.actionBackground, Padding: woxwidget.Insets{Left: 16, Top: 16, Right: 16, Bottom: 16},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
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
		buttons = append(buttons, a.buildFormTableButton("plugin-install", pluginOperationButtonLabel(snapshot, "install", plugin.ID, "Install"), 104, !busy, true, func() { a.runPluginOperation("install") }, snapshot.palette))
		width += 104
		return buttons, width
	}
	if plugin.IsUpgradable {
		buttons = append(buttons, a.buildFormTableButton("plugin-upgrade", pluginOperationButtonLabel(snapshot, "upgrade", plugin.ID, "Upgrade"), 104, !busy, true, func() { a.runPluginOperation("upgrade") }, snapshot.palette))
		width += 104
	}
	if plugin.IsDisable {
		buttons = append(buttons, a.buildFormTableButton("plugin-enable", pluginOperationButtonLabel(snapshot, "enable", plugin.ID, "Enable"), 96, !busy, false, func() { a.runPluginOperation("enable") }, snapshot.palette))
		width += 96
	} else {
		buttons = append(buttons, a.buildFormTableButton("plugin-disable", pluginOperationButtonLabel(snapshot, "disable", plugin.ID, "Disable"), 96, !busy, false, func() { a.runPluginOperation("disable") }, snapshot.palette))
		width += 96
	}
	if !plugin.IsSystem {
		label := "Uninstall"
		if snapshot.pluginUninstallArmed == plugin.ID {
			label = "Confirm uninstall"
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
		buttons = append(buttons, a.buildFormTableButton("plugin-website", "Website", 104, true, false, a.openSelectedPluginWebsite, snapshot.palette))
	}
	if plugin.IsInstalled && strings.TrimSpace(plugin.PluginDirectory) != "" {
		buttons = append(buttons, a.buildFormTableButton("plugin-directory", "Open folder", 112, true, false, a.openSelectedPluginDirectory, snapshot.palette))
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}
}

func pluginOperationButtonLabel(snapshot settingsSnapshot, kind, pluginID, idle string) string {
	if snapshot.pluginOperation == kind+":"+pluginID {
		return idle + "…"
	}
	return idle
}
