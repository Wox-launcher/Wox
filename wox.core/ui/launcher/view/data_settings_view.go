package view

import (
	"fmt"
	"strings"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// DataBackup is the display data required for one backup table row.
type DataBackup struct {
	ID        string
	Timestamp int64
	Type      string
	Path      string
}

// DataSettingsLabels contains final user-facing copy for the data page.
type DataSettingsLabels struct {
	Title                 string
	Description           string
	StorageSection        string
	BackupSection         string
	LogsSection           string
	Open                  string
	Cancel                string
	LocationChange        string
	LocationChangeConfirm string
	LocationTitle         string
	LocationDescription   string
	AutoBackupTitle       string
	AutoBackupDescription string
	BackupListTitle       string
	BackupNow             string
	BackupEmpty           string
	BackupDate            string
	BackupType            string
	BackupOperation       string
	BackupTypeManual      string
	BackupTypeAuto        string
	BackupRestore         string
	BackupRestoreConfirm  string
	LogLevelTitle         string
	LogLevelDescription   string
	LogClearButton        string
	LogClearConfirm       string
	LogClearTitle         string
	LogClearDescription   string
	LogOpenButton         string
	Loading               string
}

// DataSettingsProps contains the immutable state and actions rendered by the data page.
type DataSettingsProps struct {
	Width              float32
	Height             float32
	Theme              woxcomponent.Theme
	Labels             DataSettingsLabels
	Location           string
	PendingLocation    string
	AutoBackup         bool
	Backups            []DataBackup
	RestoreArmed       string
	LogLevel           string
	ClearLogsArmed     bool
	Note               string
	Loading            bool
	Error              string
	OnOpenPath         func(string)
	OnChooseLocation   func()
	OnCancelLocation   func()
	OnConfirmLocation  func()
	OnToggleAutoBackup func()
	OnCreateBackup     func()
	OnRestoreBackup    func(string)
	OnCycleLogLevel    func()
	OnClearLogs        func()
	OnOpenLog          func()
}

type dataColumn struct {
	key    string
	label  string
	weight float32
}

const dataBackupTitleHeight = float32(38)

// DataSettingsView builds the storage, backup, and logs page without controller dependencies.
func DataSettingsView(props DataSettingsProps) woxwidget.Widget {
	contentWidth := SettingsPageContentWidth(props.Width)
	children := []woxwidget.Widget{
		woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{
			Title: props.Labels.Title, Description: props.Labels.Description, Width: contentWidth, Theme: props.Theme,
		}),
		dataSectionHeader(props, props.Labels.StorageSection, contentWidth),
		dataStorageField(props, contentWidth),
		dataSectionHeader(props, props.Labels.BackupSection, contentWidth),
		dataAutoBackupField(props, contentWidth),
		dataBackupTable(props, contentWidth),
		dataSectionHeader(props, props.Labels.LogsSection, contentWidth),
		dataLogLevelField(props, contentWidth),
		dataLogActionsField(props, contentWidth),
	}
	backupRows := min(5, len(props.Backups))
	backupTableHeight := dataBackupTableHeight(backupRows)
	contentHeight := woxcomponent.PageHeaderHeight + 43 + 78 + 43 + 66 + backupTableHeight + 43 + 66 + 66
	if props.Note != "" || props.Loading || props.Error != "" {
		note := props.Note
		color := props.Theme.ResultSubtitle
		if props.Loading {
			note = props.Labels.Loading
		} else if props.Error != "" {
			note = props.Error
			color = props.Theme.ErrorText
		}
		children = append(children, woxwidget.Container{Width: contentWidth, Height: 30, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.TextBlock{
			Value: note, Width: contentWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: color,
		}})
		contentHeight += 30
	}
	return SettingsPage(SettingsPageProps{
		ID: "data-settings-scroll", Width: props.Width, Height: props.Height, Children: children, ContentHeight: contentHeight,
	})
}

func dataSectionHeader(props DataSettingsProps, label string, width float32) woxwidget.Widget {
	return woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: label, Width: width, Theme: props.Theme})
}

func dataStorageField(props DataSettingsProps, width float32) woxwidget.Widget {
	labelWidth := max(float32(220), width-210)
	buttons := []woxwidget.Widget{
		dataButton(props, "data-location-open", props.Labels.Open, 76, woxcomponent.ButtonOutline, func() {
			if props.OnOpenPath != nil {
				props.OnOpenPath(props.Location)
			}
		}),
		dataButton(props, "data-location-change", props.Labels.LocationChange, 112, woxcomponent.ButtonMuted, props.OnChooseLocation),
	}
	if props.PendingLocation != "" {
		buttons = []woxwidget.Widget{
			dataButton(props, "data-location-cancel", props.Labels.Cancel, 76, woxcomponent.ButtonOutline, props.OnCancelLocation),
			dataButton(props, "data-location-confirm", props.Labels.LocationChangeConfirm, 112, woxcomponent.ButtonMuted, props.OnConfirmLocation),
		}
	}
	return woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: props.Labels.LocationTitle, Description: props.Labels.LocationDescription,
		Width: width, Height: 78, LabelWidth: labelWidth, Gap: 10, Padding: woxwidget.Insets{Top: 5}, DescriptionMaxLines: 2, Theme: props.Theme,
		Child: woxwidget.Container{Width: 200, Height: 60, Padding: woxwidget.Insets{Top: 3}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: buttons}},
	})
}

func dataAutoBackupField(props DataSettingsProps, width float32) woxwidget.Widget {
	label := props.Labels.AutoBackupTitle
	return woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: label, Description: props.Labels.AutoBackupDescription, Width: width, Height: 66,
		LabelWidth: max(float32(220), width-54), Gap: 12, Padding: woxwidget.Insets{Top: 5}, Theme: props.Theme,
		Child: woxwidget.Container{Width: 42, Height: 48, Padding: woxwidget.Insets{Top: 4}, Child: woxcomponent.WoxSwitch(woxcomponent.SwitchProps{
			ID: "data-auto-backup-switch", Label: label, Value: props.AutoBackup, OnChange: func(bool) {
				if props.OnToggleAutoBackup != nil {
					props.OnToggleAutoBackup()
				}
			}, Theme: props.Theme,
		})},
	})
}

func dataBackupTable(props DataSettingsProps, width float32) woxwidget.Widget {
	visibleRows := min(5, len(props.Backups))
	height := dataBackupTableHeight(visibleRows)
	title := woxwidget.Container{Width: width, Height: dataBackupTitleHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), width-112), Height: dataBackupTitleHeight, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{
			Value: props.Labels.BackupListTitle, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle,
		}},
		dataButton(props, "data-backup-now", props.Labels.BackupNow, 100, woxcomponent.ButtonOutline, props.OnCreateBackup),
	}}}
	columns := []dataColumn{
		{key: "date", label: props.Labels.BackupDate, weight: 0.42},
		{key: "type", label: props.Labels.BackupType, weight: 0.23},
		{key: "operation", label: props.Labels.BackupOperation, weight: 0.35},
	}
	style := newTableSurfaceStyle(props.Theme)
	rows := []woxwidget.Widget{dataBackupGridRow(props, columns, DataBackup{}, -1, width, tableSurfaceHeaderHeight, true)}
	if visibleRows == 0 {
		rows = append(rows, woxwidget.Container{Width: width, Height: tableSurfaceEmptyHeight, Color: style.bodyBackground, BorderColor: style.border, BorderWidth: tableSurfaceBorderWidth,
			Padding: woxwidget.Insets{Left: max(float32(0), width/2-48), Top: 30}, Child: woxwidget.Text{
				Value: props.Labels.BackupEmpty, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle,
			}})
	} else {
		for index := 0; index < visibleRows; index++ {
			rows = append(rows, dataBackupGridRow(props, columns, props.Backups[index], index, width, tableSurfaceRowHeight, false))
		}
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		title,
		woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}}
}

func dataBackupTableHeight(rowCount int) float32 {
	bodyHeight := tableSurfaceEmptyHeight
	if rowCount > 0 {
		bodyHeight = float32(rowCount) * tableSurfaceRowHeight
	}
	return dataBackupTitleHeight + tableSurfaceHeaderHeight + bodyHeight
}

func dataBackupGridRow(props DataSettingsProps, columns []dataColumn, backup DataBackup, rowIndex int, width, height float32, header bool) woxwidget.Widget {
	style := newTableSurfaceStyle(props.Theme)
	background := style.bodyBackground
	if header {
		background = style.headerBackground
	}
	cells := make([]woxwidget.Widget, 0, len(columns))
	remaining := width
	for columnIndex, column := range columns {
		cellWidth := width * column.weight
		if columnIndex == len(columns)-1 {
			cellWidth = remaining
		}
		remaining -= cellWidth
		label := column.label
		if !header {
			switch column.key {
			case "date":
				label = time.UnixMilli(backup.Timestamp).Format("2006-01-02 15:04:05")
			case "type":
				label = props.Labels.BackupTypeManual
				if strings.EqualFold(backup.Type, "auto") {
					label = props.Labels.BackupTypeAuto
				}
			}
		}
		weight := woxui.FontWeightRegular
		fontSize := float32(11)
		textColor := props.Theme.ResultTitle
		paddingTop := float32(10)
		if header {
			weight = woxui.FontWeightSemibold
			fontSize = tableSurfaceHeaderFontSize
			textColor = style.headerText
			paddingTop = 9
		}
		cell := woxwidget.Container{Width: cellWidth, Height: height, Color: background, BorderColor: style.border, BorderWidth: tableSurfaceBorderWidth,
			Padding: woxwidget.Insets{Left: 8, Top: paddingTop, Right: 6}, Child: woxwidget.TextBlock{
				Value: label, Width: max(float32(0), cellWidth-14), Height: height - 10, MaxLines: 1,
				Style: woxui.TextStyle{Size: fontSize, Weight: weight}, Color: textColor,
			}}
		if !header && column.key == "operation" {
			current := backup
			buttonWidth := cellWidth / 2
			restoreLabel := props.Labels.BackupRestore
			if props.RestoreArmed == current.ID {
				restoreLabel = props.Labels.BackupRestoreConfirm
			}
			cell = woxwidget.Container{Width: cellWidth, Height: height, Color: background, BorderColor: style.border, BorderWidth: tableSurfaceBorderWidth,
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
					woxwidget.Gesture{ID: fmt.Sprintf("data-backup-restore-%d", rowIndex), OnTap: func() {
						if props.OnRestoreBackup != nil {
							props.OnRestoreBackup(current.ID)
						}
					}, Child: woxwidget.Container{Width: buttonWidth, Height: height, BorderColor: style.border, BorderWidth: tableSurfaceBorderWidth, Padding: woxwidget.Insets{Left: 8, Top: 10}, Child: woxwidget.TextBlock{
						Value: restoreLabel, Width: max(float32(0), buttonWidth-14), Height: height - 10, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultTitle,
					}}},
					woxwidget.Gesture{ID: fmt.Sprintf("data-backup-open-%d", rowIndex), OnTap: func() {
						if props.OnOpenPath != nil {
							props.OnOpenPath(current.Path)
						}
					}, Child: woxwidget.Container{Width: cellWidth - buttonWidth, Height: height, Padding: woxwidget.Insets{Left: 8, Top: 10}, Child: woxwidget.Text{
						Value: props.Labels.Open, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultTitle,
					}}},
				}},
			}
		}
		cells = append(cells, cell)
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: cells}
}

func dataLogLevelField(props DataSettingsProps, width float32) woxwidget.Widget {
	level := strings.ToUpper(props.LogLevel)
	if level != "DEBUG" {
		level = "INFO"
	}
	controlWidth := min(float32(280), width*0.34)
	labelWidth := max(float32(220), width-controlWidth-32)
	choice := woxwidget.Gesture{ID: "data-log-level", OnTap: props.OnCycleLogLevel, Child: woxwidget.Container{
		Width: controlWidth, Height: 34, Radius: 4, BorderColor: settingsColorAlpha(props.Theme.ResultSubtitle, 140), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 8, Top: 5, Right: 8, Bottom: 5}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Align{Width: max(float32(0), controlWidth-40), Height: 24, Vertical: 0.5, Child: woxwidget.Text{Value: level, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultTitle}},
			dropdownIndicator(24, 24, props.Theme.ResultTitle),
		}},
	}}
	return woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: props.Labels.LogLevelTitle, Description: props.Labels.LogLevelDescription,
		Width: width, Height: 66, LabelWidth: labelWidth, Gap: 32, Padding: woxwidget.Insets{Top: 5}, Child: choice, Theme: props.Theme,
	})
}

func dataLogActionsField(props DataSettingsProps, width float32) woxwidget.Widget {
	clearLabel := props.Labels.LogClearButton
	if props.ClearLogsArmed {
		clearLabel = props.Labels.LogClearConfirm
	}
	return woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: props.Labels.LogClearTitle, Description: props.Labels.LogClearDescription,
		Width: width, Height: 66, LabelWidth: max(float32(220), width-236), Gap: 10, Padding: woxwidget.Insets{Top: 5}, Theme: props.Theme,
		Child: woxwidget.Container{Width: 226, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			dataButton(props, "data-log-clear", clearLabel, 104, woxcomponent.ButtonOutline, props.OnClearLogs),
			dataButton(props, "data-log-open", props.Labels.LogOpenButton, 112, woxcomponent.ButtonOutline, props.OnOpenLog),
		}}},
	})
}

func dataButton(props DataSettingsProps, id, label string, width float32, variant woxcomponent.ButtonVariant, onTap func()) woxwidget.Widget {
	return woxcomponent.WoxButton(woxcomponent.ButtonProps{
		ID: id, Label: label, Width: width, Variant: variant, Size: woxcomponent.ButtonCompact, OnTap: onTap, Theme: props.Theme,
	})
}
