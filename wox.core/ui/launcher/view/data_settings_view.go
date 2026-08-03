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
	LogLevelInfo          string
	LogLevelDebug         string
	LogClearButton        string
	LogClearConfirm       string
	LogClearTitle         string
	LogClearDescription   string
	LogOpenButton         string
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
	Error              string
	OnOpenPath         func(string)
	OnChooseLocation   func()
	OnCancelLocation   func()
	OnConfirmLocation  func()
	OnToggleAutoBackup func()
	OnCreateBackup     func()
	OnRestoreBackup    func(string)
	OnOpenLogLevel     func(woxui.Rect)
	OnClearLogs        func()
	OnOpenLog          func()
}

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
	backupTableHeight := FormTableFieldHeight(true, "", backupRows, int(tableSurfaceHeaderHeight+tableSurfaceRowHeight*5))
	contentHeight := woxcomponent.PageHeaderHeight + 43 + 78 + 43 + 66 + backupTableHeight + 43 + 66 + 66
	if props.Error != "" {
		children = append(children, woxwidget.Container{Width: contentWidth, Height: 30, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.TextBlock{
			Value: props.Error, Width: contentWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ErrorText,
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
		dataButton(props, "data-location-change", props.Labels.LocationChange, 112, woxcomponent.ButtonOutline, props.OnChooseLocation),
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
	rows := make([]FormTableRow, 0, visibleRows)
	for index := 0; index < visibleRows; index++ {
		backup := props.Backups[index]
		backupType := props.Labels.BackupTypeManual
		if strings.EqualFold(backup.Type, "auto") {
			backupType = props.Labels.BackupTypeAuto
		}
		rows = append(rows, FormTableRow{Index: index, Cells: []FormTableCell{
			{Text: time.UnixMilli(backup.Timestamp).Format("2006-01-02 15:04:05")},
			{Text: backupType},
			{Child: dataBackupOperationCell(props, backup, index)},
		}})
	}
	maxHeight := int(tableSurfaceHeaderHeight + tableSurfaceRowHeight*5)
	return FormTableField(FormTableFieldProps{
		ID: "data-backups", Title: props.Labels.BackupListTitle, Width: width,
		Height: FormTableFieldHeight(true, "", visibleRows, maxHeight), MaxHeight: maxHeight, InlineTitle: true, ReadOnly: true,
		Columns: []FormTableColumn{{Label: props.Labels.BackupDate, Width: 350}, {Label: props.Labels.BackupType, Width: 220}, {Label: props.Labels.BackupOperation, Width: 300}},
		Rows:    rows, SecondaryLabel: props.Labels.BackupNow, EmptyLabel: props.Labels.BackupEmpty, Theme: props.Theme, OnSecondary: props.OnCreateBackup,
	})
}

// dataBackupOperationCell keeps backup-specific actions inside the shared table cell.
func dataBackupOperationCell(props DataSettingsProps, backup DataBackup, rowIndex int) woxwidget.Widget {
	restoreLabel := props.Labels.BackupRestore
	if props.RestoreArmed == backup.ID {
		restoreLabel = props.Labels.BackupRestoreConfirm
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: fmt.Sprintf("data-backup-restore-%d", rowIndex), Label: restoreLabel, Width: 80, Height: 24,
			Padding: woxwidget.Insets{Left: 4, Right: 4}, FontSize: woxcomponent.TableBodyFontSize, Variant: woxcomponent.ButtonText, OnTap: func() {
				if props.OnRestoreBackup != nil {
					props.OnRestoreBackup(backup.ID)
				}
			}, Theme: props.Theme,
		}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: fmt.Sprintf("data-backup-open-%d", rowIndex), Label: props.Labels.Open, Width: 52, Height: 24,
			Padding: woxwidget.Insets{Left: 4, Right: 4}, FontSize: woxcomponent.TableBodyFontSize, Variant: woxcomponent.ButtonText, OnTap: func() {
				if props.OnOpenPath != nil {
					props.OnOpenPath(backup.Path)
				}
			}, Theme: props.Theme,
		}),
	}}
}

func dataLogLevelField(props DataSettingsProps, width float32) woxwidget.Widget {
	level := strings.ToUpper(props.LogLevel)
	if level != "DEBUG" {
		level = "INFO"
	}
	controlWidth := min(float32(280), width*0.34)
	labelWidth := max(float32(220), width-controlWidth-32)
	choice := woxwidget.Keyed{Key: SettingChoiceAnchorKey("LogLevel"), Child: woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
		ID: "data-log-level", Label: props.Labels.LogLevelTitle, Value: level, Width: controlWidth, Height: 34,
		Outline: settingsColorAlpha(props.Theme.ResultSubtitle, 140), Foreground: props.Theme.ResultTitle, Theme: props.Theme, OnTapBounds: props.OnOpenLogLevel,
	})}
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
