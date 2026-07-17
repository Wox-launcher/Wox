package launcher

import (
	"fmt"
	"strings"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildDataSettingsPage mirrors Flutter's wide storage, backup, and logs form while keeping core operations unchanged.
func (a *App) buildDataSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-82)
	children := []woxwidget.Widget{
		a.buildSettingsPageHeader(a.translate("i18n:ui_data"), a.translate("i18n:ui_data_description"), contentWidth, snapshot.palette),
		a.buildSettingsSectionHeader(a.translate("i18n:ui_data_section_storage"), contentWidth, snapshot.palette),
		a.buildDataStorageField(snapshot, contentWidth),
		a.buildSettingsSectionHeader(a.translate("i18n:ui_data_section_backup"), contentWidth, snapshot.palette),
		a.buildDataAutoBackupField(snapshot, contentWidth),
		a.buildDataBackupTable(snapshot, contentWidth),
		a.buildSettingsSectionHeader(a.translate("i18n:ui_data_section_logs"), contentWidth, snapshot.palette),
		a.buildDataLogLevelField(snapshot, contentWidth),
		a.buildDataLogActionsField(snapshot, contentWidth),
	}
	backupRows := min(5, len(snapshot.dataBackups))
	backupTableHeight := float32(76 + backupRows*36)
	if backupRows == 0 {
		backupTableHeight = 154
	}
	contentHeight := float32(72 + 43 + 78 + 43 + 66 + backupTableHeight + 43 + 66 + 66)
	if snapshot.note != "" || snapshot.dataLoading || snapshot.dataError != "" {
		note := snapshot.note
		color := snapshot.palette.resultSubtitle
		if snapshot.dataLoading {
			note = "Loading storage and backups…"
		} else if snapshot.dataError != "" {
			note = snapshot.dataError
			color = woxui.Color{R: 232, G: 95, B: 95, A: 255}
		}
		children = append(children, woxwidget.Container{Width: contentWidth, Height: 30, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.TextBlock{
			Value: note, Width: contentWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: color,
		}})
		contentHeight += 30
	}
	viewportHeight := max(float32(1), height-58)
	a.setSettingsPageGeometry(viewportHeight, contentHeight, 0)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 38, Top: 34, Right: 44, Bottom: 24}, Child: woxwidget.Gesture{
		ID: "data-settings-scroll", OnScroll: func(delta woxui.Point) { a.scrollSettingsPage(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: contentWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, contentHeight), Offset: snapshot.pageScroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
		},
	}}
}

func (a *App) buildDataStorageField(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	labelWidth := max(float32(220), width-210)
	buttons := []woxwidget.Widget{
		a.buildSettingsOutlineButton("data-location-open", a.translate("i18n:plugin_file_open"), 76, func() { a.openDataPath(snapshot.dataLocation) }, snapshot.palette),
		a.buildSettingsPrimaryButton("data-location-change", a.translate("i18n:ui_data_config_location_change"), 112, a.chooseDataLocation, snapshot.palette),
	}
	if snapshot.dataPendingLocation != "" {
		buttons = []woxwidget.Widget{
			a.buildSettingsOutlineButton("data-location-cancel", a.translate("i18n:ui_cancel"), 76, a.cancelDataLocationChange, snapshot.palette),
			a.buildSettingsPrimaryButton("data-location-confirm", a.translate("i18n:ui_data_config_location_change_confirm_button"), 112, a.confirmDataLocationChange, snapshot.palette),
		}
	}
	return woxwidget.Container{Width: width, Height: 78, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelWidth, Height: 66, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Text{Value: a.translate("i18n:ui_data_config_location"), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
			woxwidget.TextBlock{Value: a.translate("i18n:ui_data_config_location_tips"), Width: labelWidth, Height: 38, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, LineHeight: 16, Color: snapshot.palette.resultSubtitle},
		}}},
		woxwidget.Container{Width: 200, Height: 60, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: buttons}},
	}}}
}

func (a *App) buildDataAutoBackupField(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	labelWidth := max(float32(220), width-54)
	return woxwidget.Gesture{ID: "data-auto-backup", OnTap: a.toggleDataAutoBackup, Child: woxwidget.Container{Width: width, Height: 66, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelWidth, Height: 58, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Text{Value: a.translate("i18n:ui_data_backup_auto_title"), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
			woxwidget.Text{Value: a.translate("i18n:ui_data_backup_auto_tips"), Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle},
		}}},
		a.buildSettingsSwitch(snapshot.data.EnableAutoBackup, snapshot.palette),
	}}}}
}

func (a *App) buildDataBackupTable(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	visibleRows := min(5, len(snapshot.dataBackups))
	height := float32(76 + visibleRows*36)
	if visibleRows == 0 {
		height = 154
	}
	title := woxwidget.Container{Width: width, Height: 38, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), width-112), Height: 38, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{Value: a.translate("i18n:ui_data_backup_list_title"), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle}},
		woxwidget.Container{Width: 100, Height: 38, Child: a.buildSettingsOutlineButton("data-backup-now", dataBackupNowLabelLocalized(a), 100, a.createDataBackup, snapshot.palette)},
	}}}
	columns := []settingsInlineColumn{
		{key: "date", label: "i18n:ui_data_backup_date", weight: 0.42},
		{key: "type", label: "i18n:ui_data_backup_type", weight: 0.23},
		{key: "operation", label: "i18n:ui_operation", weight: 0.35},
	}
	header := a.buildDataBackupGridRow(snapshot, columns, backupInfo{}, -1, width, 38, true)
	rows := []woxwidget.Widget{header}
	if visibleRows == 0 {
		rows = append(rows, woxwidget.Container{Width: width, Height: 78, BorderColor: settingsAlpha(snapshot.palette.previewSplit, 144), BorderWidth: 1,
			Padding: woxwidget.Insets{Left: max(float32(0), width/2-48), Top: 30}, Child: woxwidget.Text{Value: a.translate("i18n:ui_data_backup_empty"), Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.resultSubtitle}})
	} else {
		for index := 0; index < visibleRows; index++ {
			rows = append(rows, a.buildDataBackupGridRow(snapshot, columns, snapshot.dataBackups[index], index, width, 36, false))
		}
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		title,
		woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}}
}

func (a *App) buildDataBackupGridRow(snapshot settingsSnapshot, columns []settingsInlineColumn, backup backupInfo, rowIndex int, width, height float32, header bool) woxwidget.Widget {
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
			switch column.key {
			case "date":
				label = time.UnixMilli(backup.Timestamp).Format("2006-01-02 15:04:05")
			case "type":
				label = a.translate("i18n:ui_data_backup_type_manual")
				if strings.EqualFold(backup.Type, "auto") {
					label = a.translate("i18n:ui_data_backup_type_auto")
				}
			}
		}
		weight := woxui.FontWeightRegular
		if header {
			weight = woxui.FontWeightSemibold
		}
		cell := woxwidget.Container{Width: cellWidth, Height: height, Color: background, BorderColor: settingsAlpha(snapshot.palette.previewSplit, 144), BorderWidth: 0.5,
			Padding: woxwidget.Insets{Left: 8, Top: 10, Right: 6}, Child: woxwidget.TextBlock{Value: label, Width: max(float32(0), cellWidth-14), Height: height - 10, MaxLines: 1,
				Style: woxui.TextStyle{Size: 11, Weight: weight}, Color: snapshot.palette.resultTitle}}
		if !header && column.key == "operation" {
			current := backup
			buttonWidth := cellWidth / 2
			restoreLabel := a.translate("i18n:ui_data_backup_restore")
			if snapshot.dataRestoreArmed == current.ID {
				restoreLabel = a.translate("i18n:ui_data_backup_restore_confirm")
			}
			cell = woxwidget.Container{Width: cellWidth, Height: height, Color: background, BorderColor: settingsAlpha(snapshot.palette.previewSplit, 144), BorderWidth: 0.5,
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
					woxwidget.Gesture{ID: fmt.Sprintf("data-backup-restore-%d", rowIndex), OnTap: func() { a.restoreDataBackup(current.ID) }, Child: woxwidget.Container{
						Width: buttonWidth, Height: height, BorderColor: settingsAlpha(snapshot.palette.previewSplit, 96), BorderWidth: 0.5, Padding: woxwidget.Insets{Left: 8, Top: 10},
						Child: woxwidget.TextBlock{Value: restoreLabel, Width: max(float32(0), buttonWidth-14), Height: height - 10, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultTitle},
					}},
					woxwidget.Gesture{ID: fmt.Sprintf("data-backup-open-%d", rowIndex), OnTap: func() { a.openDataPath(current.Path) }, Child: woxwidget.Container{
						Width: cellWidth - buttonWidth, Height: height, Padding: woxwidget.Insets{Left: 8, Top: 10}, Child: woxwidget.Text{
							Value: a.translate("i18n:plugin_file_open"), Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultTitle,
						},
					}},
				}},
			}
		}
		cells = append(cells, cell)
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: cells}
}

func (a *App) buildDataLogLevelField(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	level := strings.ToUpper(snapshot.data.LogLevel)
	if level != "DEBUG" {
		level = "INFO"
	}
	controlWidth := min(float32(280), width*0.34)
	labelWidth := max(float32(220), width-controlWidth-32)
	choice := woxwidget.Gesture{ID: "data-log-level", OnTap: a.cycleDataLogLevel, Child: woxwidget.Container{Width: controlWidth, Height: 38, Radius: 4, BorderColor: snapshot.palette.resultSubtitle, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), controlWidth-38), Height: 24, Child: woxwidget.Text{Value: level, Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.resultTitle}},
			woxwidget.Text{Value: "▾", Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle},
		}}}}
	return woxwidget.Container{Width: width, Height: 66, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 32, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelWidth, Height: 56, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Text{Value: a.translate("i18n:ui_data_log_level_title"), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
			woxwidget.Text{Value: a.translate("i18n:ui_data_log_level_tips"), Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle},
		}}},
		choice,
	}}}
}

func (a *App) buildDataLogActionsField(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	labelWidth := max(float32(220), width-236)
	clearLabel := a.translate("i18n:ui_data_log_clear_button")
	if snapshot.dataClearLogsArmed {
		clearLabel = a.translate("i18n:ui_data_log_clear_confirm")
	}
	return woxwidget.Container{Width: width, Height: 66, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelWidth, Height: 56, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Text{Value: a.translate("i18n:ui_data_log_clear_title"), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
			woxwidget.Text{Value: a.translate("i18n:ui_data_log_clear_tips"), Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle},
		}}},
		woxwidget.Container{Width: 226, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			a.buildSettingsOutlineButton("data-log-clear", clearLabel, 104, a.clearDataLogs, snapshot.palette),
			a.buildSettingsOutlineButton("data-log-open", a.translate("i18n:ui_data_log_open_button"), 112, a.openDataLog, snapshot.palette),
		}}},
	}}}
}

func (a *App) buildSettingsSwitch(enabled bool, palette uiPalette) woxwidget.Widget {
	trackColor := settingsAlpha(palette.resultSubtitle, 104)
	knobLeft := float32(2)
	if enabled {
		trackColor = palette.cursor
		knobLeft = 22
	}
	return woxwidget.Container{Width: 42, Height: 48, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.Stack{Width: 42, Height: 22, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: 42, Height: 22, Radius: 11, Color: trackColor}},
		{Left: knobLeft, Top: 2, Child: woxwidget.Container{Width: 18, Height: 18, Radius: 9, Color: woxui.Color{R: 248, G: 248, B: 248, A: 255}}},
	}}}
}

func (a *App) buildSettingsPrimaryButton(id, label string, width float32, onTap func(), palette uiPalette) woxwidget.Widget {
	return woxwidget.Gesture{ID: id, OnTap: onTap, Child: woxwidget.Container{Width: width, Height: 30, Radius: 4, Color: settingsAlpha(palette.resultSubtitle, 72),
		Padding: woxwidget.Insets{Left: 12, Top: 8, Right: 8}, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: palette.resultTitle}}}
}

func dataBackupNowLabelLocalized(a *App) string {
	return a.translate("i18n:ui_data_backup_now")
}
