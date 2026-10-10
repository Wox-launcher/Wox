package view

import (
	"fmt"
	"strings"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const dataBackupOperationColumnWidth = float32(140)

// Data-page error sections. Empty is a page-level message under the header.
const (
	dataErrorSectionStorage = "storage"
	dataErrorSectionBackup  = "backup"
	dataErrorSectionLogs    = "logs"
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
	Theme              woxcomponent.ControlTheme
	Labels             DataSettingsLabels
	Location           string
	PendingLocation    string
	AutoBackup         bool
	Backups            []DataBackup
	LogLevel           string
	ClearLogsArmed     bool
	Error              string
	ErrorSection       string
	OnOpenPath         func(path, section string)
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
	var keepVisible woxwidget.Key
	if props.Error != "" {
		keepVisible = dataSettingsErrorKey(props)
	}
	return SettingsPage(SettingsPageProps{Theme: props.Theme,
		ID: "data-settings-scroll", Width: props.Width, Height: props.Height, Children: dataSettingsContent(props, contentWidth),
		KeepVisibleKey: keepVisible,
	})
}

// dataSettingsContent places each failure under the section the user just used.
// A message after the logs block stays below the fold, so Restore looks like a no-op.
func dataSettingsContent(props DataSettingsProps, width float32) []woxwidget.Widget {
	children := []woxwidget.Widget{
		woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{
			Title: props.Labels.Title, Description: props.Labels.Description, Width: width, Theme: props.Theme,
		}),
	}
	children = appendDataSectionError(children, props, "", width)
	children = append(children,
		dataSectionHeader(props, props.Labels.StorageSection, width),
		dataStorageField(props, width),
	)
	children = appendDataSectionError(children, props, dataErrorSectionStorage, width)
	children = append(children,
		dataSectionHeader(props, props.Labels.BackupSection, width),
		dataAutoBackupField(props, width),
		dataBackupTable(props, width),
	)
	children = appendDataSectionError(children, props, dataErrorSectionBackup, width)
	children = append(children,
		dataSectionHeader(props, props.Labels.LogsSection, width),
		dataLogLevelField(props, width),
		dataLogActionsField(props, width),
	)
	return appendDataSectionError(children, props, dataErrorSectionLogs, width)
}

func appendDataSectionError(children []woxwidget.Widget, props DataSettingsProps, section string, width float32) []woxwidget.Widget {
	if props.Error == "" || dataErrorSectionOrPage(props.ErrorSection) != section {
		return children
	}
	return append(children, dataSettingsError(props, width))
}

func dataErrorSectionOrPage(section string) string {
	switch section {
	case dataErrorSectionStorage, dataErrorSectionBackup, dataErrorSectionLogs:
		return section
	default:
		return ""
	}
}

func dataSettingsErrorKey(props DataSettingsProps) woxwidget.Key {
	return woxwidget.Key("data-settings-error:" + dataErrorSectionOrPage(props.ErrorSection) + ":" + props.Error)
}

// dataSettingsError keeps a long path failure readable and is the scroll target.
func dataSettingsError(props DataSettingsProps, width float32) woxwidget.Widget {
	message := props.Error
	return woxwidget.Keyed{Key: dataSettingsErrorKey(props), Child: woxwidget.Semantics{
		AutomationID: "data-settings-error", Role: woxui.AccessibilityRoleText, Label: message, Value: message,
		LiveRegion: woxui.AccessibilityLiveRegionPolite,
		Child: woxwidget.Container{Width: width, Padding: woxwidget.Insets{Top: 4, Bottom: 8}, Child: woxwidget.TextBlock{
			Value: message, Width: width, MaxLines: 3, LineHeight: props.Theme.Scaled(16),
			Style: woxui.TextStyle{Size: props.Theme.Scaled(woxcomponent.SettingsHelpFontSize)}, Color: props.Theme.Error,
		}},
	}}
}

func dataSectionHeader(props DataSettingsProps, label string, width float32) woxwidget.Widget {
	return woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: label, Width: width, Theme: props.Theme})
}

func dataStorageField(props DataSettingsProps, width float32) woxwidget.Widget {
	buttons := []woxwidget.Widget{
		dataButton(props, "data-location-open", props.Labels.Open, woxcomponent.ButtonSecondary, func() {
			if props.OnOpenPath != nil {
				props.OnOpenPath(props.Location, dataErrorSectionStorage)
			}
		}),
		dataButton(props, "data-location-change", props.Labels.LocationChange, woxcomponent.ButtonSecondary, props.OnChooseLocation),
	}
	if props.PendingLocation != "" {
		buttons = []woxwidget.Widget{
			dataButton(props, "data-location-cancel", props.Labels.Cancel, woxcomponent.ButtonSecondary, props.OnCancelLocation),
			dataButton(props, "data-location-confirm", props.Labels.LocationChangeConfirm, woxcomponent.ButtonMuted, props.OnConfirmLocation),
		}
	}
	return woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: props.Labels.LocationTitle, Description: props.Labels.LocationDescription,
		Width: width, Height: 78, Gap: 10, Padding: woxwidget.Insets{Top: 5}, DescriptionMaxLines: 2, Theme: props.Theme,
		Child: woxwidget.Container{Height: 60, Padding: woxwidget.Insets{Top: 3}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 10, Children: buttons,
		}},
	})
}

func dataAutoBackupField(props DataSettingsProps, width float32) woxwidget.Widget {
	label := props.Labels.AutoBackupTitle
	return woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: label, Description: props.Labels.AutoBackupDescription, Width: width, Height: woxcomponent.SettingsRowHeight,
		Gap: 12, Padding: woxwidget.Insets{Top: 5}, Theme: props.Theme,
		Child: woxwidget.Align{Width: woxcomponent.SettingsSwitchWidth, Height: woxcomponent.SettingsControlHeight, Horizontal: 1, Vertical: 0.5, Child: woxcomponent.WoxSwitch(woxcomponent.SwitchProps{
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
	height := FormTableFieldHeight(true, "", visibleRows, maxHeight)
	if visibleRows == 0 && strings.TrimSpace(props.Labels.BackupEmpty) != "" {
		height += woxcomponent.SettingsControlHeight
	}
	return FormTableField(FormTableFieldProps{
		ID: "data-backups", Title: props.Labels.BackupListTitle, Width: width,
		Height: height, MaxHeight: maxHeight, InlineTitle: true, ReadOnly: true,
		Columns: []FormTableColumn{{Label: props.Labels.BackupDate, Width: 350}, {Label: props.Labels.BackupType, Width: 220}, {Label: props.Labels.BackupOperation, Width: dataBackupOperationColumnWidth}},
		Rows:    rows, SecondaryLabel: props.Labels.BackupNow, EmptyLabel: props.Labels.BackupEmpty,
		HeaderWeight: woxui.FontWeightSemibold, Theme: props.Theme, OnSecondary: props.OnCreateBackup,
	})
}

type dataBackupRestoreState struct {
	confirm bool
}

type dataBackupRestoreWidget struct {
	id           string
	backupID     string
	label        string
	confirmLabel string
	theme        woxcomponent.ControlTheme
	onRestore    func(string)
}

// dataBackupRestoreButton confirms on the button itself. Pointer leave, blur, or
// Escape returns it to Restore, matching chat delete and plugin uninstall.
func dataBackupRestoreButton(props DataSettingsProps, backup DataBackup, rowIndex int) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: woxwidget.Key("data-backup-restore-" + backup.ID), Type: (*dataBackupRestoreState)(nil),
		Widget: dataBackupRestoreWidget{
			id: fmt.Sprintf("data-backup-restore-%d", rowIndex), backupID: backup.ID,
			label: props.Labels.BackupRestore, confirmLabel: props.Labels.BackupRestoreConfirm,
			theme: props.Theme, onRestore: props.OnRestoreBackup,
		},
		CreateState: func() woxwidget.State { return &dataBackupRestoreState{} },
	}
}

func (s *dataBackupRestoreState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *dataBackupRestoreState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

func (s *dataBackupRestoreState) Dispose() {}

func (s *dataBackupRestoreState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(dataBackupRestoreWidget)
	label := props.label
	theme := props.theme
	var weight woxui.FontWeight
	if s.confirm {
		label = props.confirmLabel
		weight = woxui.FontWeightSemibold
		if theme.Warning.A != 0 {
			theme.Text = theme.Warning
		}
	}
	clear := func() {
		if s.confirm {
			context.SetState(func() { s.confirm = false })
		}
	}
	return woxcomponent.WoxButton(woxcomponent.ButtonProps{
		ID: props.id, Label: label, FontWeight: weight,
		Padding: woxwidget.Insets{Left: 4, Right: 4}, FontSize: woxcomponent.TableBodyFontSize, Variant: woxcomponent.ButtonText,
		OnTap: func() {
			confirmed := false
			context.SetState(func() { confirmed = s.advance() })
			if confirmed && props.onRestore != nil {
				props.onRestore(props.backupID)
			}
		},
		OnHoverAt: func(inside bool, _ woxui.Rect) {
			if !inside {
				clear()
			}
		},
		OnFocusChange: func(focused bool) {
			if !focused {
				clear()
			}
		},
		OnKey: func(event woxui.KeyEvent) bool {
			if event.Key != woxui.KeyEscape || !s.confirm {
				return false
			}
			if event.Down {
				clear()
			}
			return true
		},
		Theme: theme,
	})
}

// advance moves from Restore to Confirm, then clears confirmation and reports the second click.
func (s *dataBackupRestoreState) advance() bool {
	if !s.confirm {
		s.confirm = true
		return false
	}
	s.confirm = false
	return true
}

// dataBackupOperationCell keeps backup-specific actions inside the shared table cell.
func dataBackupOperationCell(props DataSettingsProps, backup DataBackup, rowIndex int) woxwidget.Widget {
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		dataBackupRestoreButton(props, backup, rowIndex),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: fmt.Sprintf("data-backup-open-%d", rowIndex), Label: props.Labels.Open,
			Padding: woxwidget.Insets{Left: 4, Right: 4}, FontSize: woxcomponent.TableBodyFontSize, Variant: woxcomponent.ButtonText, OnTap: func() {
				if props.OnOpenPath != nil {
					props.OnOpenPath(backup.Path, dataErrorSectionBackup)
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
	choice := woxwidget.Keyed{Key: SettingChoiceAnchorKey("LogLevel"), Child: woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
		ID: "data-log-level", Label: props.Labels.LogLevelTitle, Value: level, Width: controlWidth, Height: woxcomponent.SettingsControlHeight,
		Foreground: props.Theme.Text, Theme: props.Theme, OnTapBounds: props.OnOpenLogLevel,
	})}
	return woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: props.Labels.LogLevelTitle, Description: props.Labels.LogLevelDescription,
		Width: width, Height: woxcomponent.SettingsRowHeight, Gap: 32, Padding: woxwidget.Insets{Top: 5}, Child: choice, Theme: props.Theme,
	})
}

func dataLogActionsField(props DataSettingsProps, width float32) woxwidget.Widget {
	clearLabel := props.Labels.LogClearButton
	if props.ClearLogsArmed {
		clearLabel = props.Labels.LogClearConfirm
	}
	return woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: props.Labels.LogClearTitle, Description: props.Labels.LogClearDescription,
		Width: width, Height: woxcomponent.SettingsRowHeight, Gap: 10, Padding: woxwidget.Insets{Top: 5}, Theme: props.Theme,
		Child: woxwidget.Container{Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			dataButton(props, "data-log-clear", clearLabel, woxcomponent.ButtonSecondary, props.OnClearLogs),
			dataButton(props, "data-log-open", props.Labels.LogOpenButton, woxcomponent.ButtonSecondary, props.OnOpenLog),
		}}},
	})
}

func dataButton(props DataSettingsProps, id, label string, variant woxcomponent.ButtonVariant, onTap func()) woxwidget.Widget {
	return woxcomponent.WoxButton(woxcomponent.ButtonProps{
		ID: id, Label: label, Variant: variant, OnTap: onTap, Theme: props.Theme,
	})
}
