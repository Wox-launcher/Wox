package launcher

import (
	"fmt"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type settingsInlineColumn struct {
	key    string
	label  string
	weight float32
}

// buildAISettingsPage renders the same inline tables and page rhythm as the Flutter settings surface.
func (a *App) buildAISettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-82)
	if snapshot.aiForm == nil {
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 38, Top: 34, Right: 44}, Child: woxwidget.Text{
			Value: "AI settings are unavailable.", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle,
		}}
	}

	children := []woxwidget.Widget{
		a.buildSettingsPageHeader(a.translate("i18n:ui_ai"), a.translate("i18n:ui_ai_description"), contentWidth, snapshot.palette),
	}
	contentHeight := float32(72)
	for index, definition := range snapshot.aiForm.definitions {
		table, tableHeight := a.buildAIInlineTable(snapshot, index, definition, contentWidth)
		children = append(children, table)
		contentHeight += tableHeight
	}
	if snapshot.note != "" || snapshot.aiProvidersLoading || snapshot.aiProvidersError != "" {
		note := snapshot.note
		if snapshot.aiProvidersLoading {
			note = "Loading the provider catalog…"
		} else if snapshot.aiProvidersError != "" {
			note = snapshot.aiProvidersError
		}
		children = append(children, woxwidget.Container{Width: contentWidth, Height: 30, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.TextBlock{
			Value: note, Width: contentWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle,
		}})
		contentHeight += 30
	}

	viewportHeight := max(float32(1), height-58)
	a.setSettingsPageGeometry(viewportHeight, contentHeight, 0)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 38, Top: 34, Right: 44, Bottom: 24}, Child: woxwidget.Gesture{
		ID: "ai-settings-scroll", OnScroll: func(delta woxui.Point) { a.scrollSettingsPage(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: contentWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, contentHeight), Offset: snapshot.pageScroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
		},
	}}
}

func (a *App) buildAIInlineTable(snapshot settingsSnapshot, index int, definition formDefinition, width float32) (woxwidget.Widget, float32) {
	rows, err := decodeFormTableRows(snapshot.aiForm.values[definition.Value.Key])
	if err != nil {
		rows = nil
	}
	columns, description, maxRows := a.aiInlineTableColumns(definition.Value.Key)
	visibleRows := min(len(rows), maxRows)
	titleHeight := float32(42)
	if description != "" {
		titleHeight = 60
	}
	bodyHeight := float32(38 + visibleRows*36)
	if visibleRows == 0 {
		bodyHeight = 116
	}
	sectionHeight := titleHeight + bodyHeight + 24

	button := a.buildSettingsOutlineButton(fmt.Sprintf("ai-table-add-%d", index), a.translate("i18n:ui_add"), 74, func() { a.addAISettingsTableRow(index) }, snapshot.palette)
	titleWidth := max(float32(0), width-86)
	titleChildren := []woxwidget.Widget{
		woxwidget.Text{Value: a.translate(formTableTitle(definition)), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
	}
	if description != "" {
		titleChildren = append(titleChildren, woxwidget.TextBlock{Value: description, Width: titleWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle})
	}
	title := woxwidget.Container{Width: width, Height: titleHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Container{Width: titleWidth, Height: titleHeight, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: titleChildren}},
		woxwidget.Container{Width: 74, Height: titleHeight, Padding: woxwidget.Insets{Top: 1}, Child: button},
	}}}
	grid := a.buildSettingsInlineGrid(snapshot, index, definition, columns, rows[:visibleRows], width, bodyHeight)
	return woxwidget.Container{Width: width, Height: sectionHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{title, grid}}}, sectionHeight
}

func (a *App) aiInlineTableColumns(key string) ([]settingsInlineColumn, string, int) {
	switch key {
	case "AIProviders":
		return []settingsInlineColumn{
			{key: "Status", label: "i18n:ui_ai_providers_status", weight: 0.06},
			{key: "Name", label: "i18n:ui_ai_providers_name", weight: 0.15},
			{key: "Alias", label: "i18n:ui_ai_providers_alias", weight: 0.17},
			{key: "Host", label: "i18n:ui_ai_providers_host", weight: 0.23},
			{key: "ApiKey", label: "i18n:ui_ai_providers_api_key", weight: 0.27},
			{key: "_action", label: "i18n:ui_operation", weight: 0.12},
		}, "", 4
	case "AIMCPServers":
		return []settingsInlineColumn{
			{key: "Name", label: "i18n:plugin_ai_chat_mcp_server_name", weight: 0.15},
			{key: "Tools", label: "i18n:plugin_ai_chat_mcp_server_tools", weight: 0.09},
			{key: "Disabled", label: "i18n:plugin_ai_chat_mcp_server_disabled", weight: 0.10},
			{key: "Type", label: "i18n:plugin_ai_chat_mcp_server_type", weight: 0.13},
			{key: "Command", label: "i18n:plugin_ai_chat_mcp_server_command", weight: 0.15},
			{key: "EnvironmentVariables", label: "i18n:plugin_ai_chat_mcp_server_environment_variables", weight: 0.19},
			{key: "Url", label: "i18n:plugin_ai_chat_mcp_server_url", weight: 0.19},
		}, a.translate("i18n:ui_ai_mcp_servers_tooltip"), 3
	default:
		return []settingsInlineColumn{
			{key: "Name", label: "i18n:plugin_ai_chat_skill_name", weight: 0.26},
			{key: "Source", label: "i18n:plugin_ai_chat_skill_type", weight: 0.14},
			{key: "Description", label: "i18n:plugin_ai_chat_skill_description", weight: 0.48},
			{key: "_action", label: "i18n:ui_operation", weight: 0.12},
		}, a.translate("i18n:ui_ai_skills_tooltip"), 6
	}
}

func (a *App) buildSettingsInlineGrid(snapshot settingsSnapshot, tableIndex int, definition formDefinition, columns []settingsInlineColumn, rows []map[string]any, width, height float32) woxwidget.Widget {
	header := a.buildSettingsInlineGridRow(snapshot, tableIndex, definition, columns, nil, -1, width, 38, true)
	children := []woxwidget.Widget{header}
	if len(rows) == 0 {
		children = append(children, woxwidget.Container{Width: width, Height: height - 38, BorderColor: settingsAlpha(snapshot.palette.previewSplit, 144), BorderWidth: 1,
			Padding: woxwidget.Insets{Left: max(float32(0), width/2-34), Top: 31}, Child: woxwidget.Text{Value: a.translate("i18n:ui_no_data"), Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.resultSubtitle}})
	} else {
		for rowIndex, row := range rows {
			children = append(children, a.buildSettingsInlineGridRow(snapshot, tableIndex, definition, columns, row, rowIndex, width, 36, false))
		}
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}

func (a *App) buildSettingsInlineGridRow(snapshot settingsSnapshot, tableIndex int, definition formDefinition, columns []settingsInlineColumn, row map[string]any, rowIndex int, width, height float32, header bool) woxwidget.Widget {
	background := settingsAlpha(snapshot.palette.queryBackground, 32)
	if header {
		background = settingsAlpha(snapshot.palette.queryBackground, 92)
	}
	cells := make([]woxwidget.Widget, 0, len(columns))
	remaining := width
	for columnIndex, column := range columns {
		cellWidth := width * column.weight
		if columnIndex == len(columns)-1 {
			cellWidth = remaining
		}
		remaining -= cellWidth
		label := a.translate(column.label)
		if !header {
			label = a.inlineTableCellValue(definition, column.key, row)
		}
		var child woxwidget.Widget = woxwidget.TextBlock{Value: label, Width: max(float32(0), cellWidth-14), Height: height - 10, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultTitle}
		if header {
			child = woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle}
		} else if column.key == "Status" {
			child = woxwidget.Container{Width: 16, Height: 16, Radius: 8, Color: woxui.Color{R: 69, G: 184, B: 88, A: 255}}
		} else if column.key == "_action" {
			child = woxwidget.Text{Value: a.translate("i18n:ui_setting_theme_edit"), Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle}
		}
		cell := woxwidget.Container{Width: cellWidth, Height: height, Color: background, BorderColor: settingsAlpha(snapshot.palette.previewSplit, 144), BorderWidth: 0.5,
			Padding: woxwidget.Insets{Left: 8, Top: 10, Right: 6}, Child: child}
		if !header {
			currentRow := rowIndex
			cell = woxwidget.Container{Width: cellWidth, Height: height, Child: woxwidget.Gesture{ID: fmt.Sprintf("ai-table-%d-row-%d-column-%d", tableIndex, rowIndex, columnIndex), OnTap: func() {
				a.openAISettingsTableRow(tableIndex, currentRow)
			}, Child: cell}}
		}
		cells = append(cells, cell)
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: cells}
}

func (a *App) inlineTableCellValue(definition formDefinition, key string, row map[string]any) string {
	if key == "_action" {
		return a.translate("i18n:ui_setting_theme_edit")
	}
	if key == "Source" {
		if strings.EqualFold(fmt.Sprint(row[key]), "remote") {
			return a.translate("i18n:ui_ai_skill_type_remote")
		}
		return a.translate("i18n:ui_ai_skill_type_local")
	}
	for _, column := range definition.Value.Columns {
		if column.Key == key {
			return compactFormTableText(a.formTableDisplayValue(column, row), 34)
		}
	}
	return compactFormTableText(fmt.Sprint(row[key]), 34)
}

func (a *App) buildSettingsOutlineButton(id, label string, width float32, onTap func(), palette uiPalette) woxwidget.Widget {
	return woxwidget.Gesture{ID: id, OnTap: onTap, Child: woxwidget.Container{Width: width, Height: 30, Radius: 4, BorderColor: palette.resultSubtitle, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 12, Top: 8, Right: 8}, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: palette.resultTitle}}}
}
