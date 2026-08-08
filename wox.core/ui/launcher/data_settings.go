package launcher

import (
	"context"
	"fmt"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type backupInfo struct {
	ID        string `json:"Id"`
	Name      string `json:"Name"`
	Timestamp int64  `json:"Timestamp"`
	Type      string `json:"Type"`
	Path      string `json:"Path"`
}

// buildDataSettingsPage adapts controller state to the package-independent data settings view.
func (a *App) buildDataSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	backups := make([]launcherview.DataBackup, len(snapshot.dataState.Backups))
	for index, backup := range snapshot.dataState.Backups {
		backups[index] = launcherview.DataBackup{ID: backup.ID, Timestamp: backup.Timestamp, Type: backup.Type, Path: backup.Path}
	}
	return launcherview.DataSettingsView(launcherview.DataSettingsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(), Labels: a.dataSettingsLabels(),
		Location: snapshot.dataState.Location, PendingLocation: snapshot.dataState.PendingLocation, AutoBackup: snapshot.general.Data.EnableAutoBackup,
		Backups: backups, RestoreArmed: snapshot.dataState.RestoreArmed, LogLevel: snapshot.general.Data.LogLevel, ClearLogsArmed: snapshot.dataState.ClearLogsArmed,
		Error:      snapshot.dataState.Error,
		OnOpenPath: a.openDataPath, OnChooseLocation: a.chooseDataLocation, OnCancelLocation: a.cancelDataLocationChange,
		OnConfirmLocation: a.confirmDataLocationChange, OnToggleAutoBackup: a.toggleDataAutoBackup, OnCreateBackup: a.createDataBackup,
		OnRestoreBackup: a.restoreDataBackup, OnOpenLogLevel: a.openDataLogLevelPicker, OnClearLogs: a.clearDataLogs, OnOpenLog: a.openDataLog,
	})
}

// dataSettingsLabels resolves all user-facing copy before entering the view layer.
func (a *App) dataSettingsLabels() launcherview.DataSettingsLabels {
	return launcherview.DataSettingsLabels{
		Title:                 a.translate("i18n:ui_data"),
		Description:           a.translate("i18n:ui_data_description"),
		StorageSection:        a.translate("i18n:ui_data_section_storage"),
		BackupSection:         a.translate("i18n:ui_data_section_backup"),
		LogsSection:           a.translate("i18n:ui_data_section_logs"),
		Open:                  a.translate("i18n:plugin_file_open"),
		Cancel:                a.translate("i18n:ui_cancel"),
		LocationChange:        a.translate("i18n:ui_data_config_location_change"),
		LocationChangeConfirm: a.translate("i18n:ui_data_config_location_change_confirm_button"),
		LocationTitle:         a.translate("i18n:ui_data_config_location"),
		LocationDescription:   a.translate("i18n:ui_data_config_location_tips"),
		AutoBackupTitle:       a.translate("i18n:ui_data_backup_auto_title"),
		AutoBackupDescription: a.translate("i18n:ui_data_backup_auto_tips"),
		BackupListTitle:       a.translate("i18n:ui_data_backup_list_title"),
		BackupNow:             a.translate("i18n:ui_data_backup_now"),
		BackupEmpty:           a.translate("i18n:ui_data_backup_empty"),
		BackupDate:            a.translate("i18n:ui_data_backup_date"),
		BackupType:            a.translate("i18n:ui_data_backup_type"),
		BackupOperation:       a.translate("i18n:ui_operation"),
		BackupTypeManual:      a.translate("i18n:ui_data_backup_type_manual"),
		BackupTypeAuto:        a.translate("i18n:ui_data_backup_type_auto"),
		BackupRestore:         a.translate("i18n:ui_data_backup_restore"),
		BackupRestoreConfirm:  a.translate("i18n:ui_data_backup_restore_confirm"),
		LogLevelTitle:         a.translate("i18n:ui_data_log_level_title"),
		LogLevelDescription:   a.translate("i18n:ui_data_log_level_tips"),
		LogLevelInfo:          a.translate("i18n:ui_data_log_level_info"),
		LogLevelDebug:         a.translate("i18n:ui_data_log_level_debug"),
		LogClearButton:        a.translate("i18n:ui_data_log_clear_button"),
		LogClearConfirm:       a.translate("i18n:ui_data_log_clear_confirm"),
		LogClearTitle:         a.translate("i18n:ui_data_log_clear_title"),
		LogClearDescription:   a.translate("i18n:ui_data_log_clear_tips"),
		LogOpenButton:         a.translate("i18n:ui_data_log_open_button"),
	}
}

// reloadDataSettings delegates to dataSettingsController so App no longer holds data state directly.
func (a *App) reloadDataSettings() {
	a.dataSettings.Reload(context.Background(), a.services, a.sessionID)
}

// createDataBackup delegates to dataSettingsController.
func (a *App) createDataBackup() {
	a.dataSettings.CreateBackup(context.Background(), a.services, a.sessionID)
}

// restoreDataBackup delegates to dataSettingsController.
func (a *App) restoreDataBackup(id string) {
	a.dataSettings.RestoreBackup(context.Background(), a.services, a.sessionID, id)
}

// chooseDataLocation delegates to dataSettingsController, which uses the injected
// native directory picker.
func (a *App) chooseDataLocation() {
	a.dataSettings.ChooseLocation()
}

func (a *App) cancelDataLocationChange() {
	a.dataSettings.CancelLocationChange()
}

// confirmDataLocationChange delegates to dataSettingsController.
func (a *App) confirmDataLocationChange() {
	a.dataSettings.ConfirmLocationChange(context.Background(), a.services, a.sessionID)
}

// toggleDataAutoBackup reuses the regular key-value settings save and rollback behavior.
// Stays on App because it operates on the general-domain EnableAutoBackup setting.
func (a *App) toggleDataAutoBackup() {
	if a.settingSaving {
		return
	}
	next := !a.generalSettings.Data().EnableAutoBackup
	label := "Off"
	if next {
		label = "On"
	}
	a.beginSettingSave()
	a.invalidateSettingsWindow()
	util.Go(a.lifecycleCtx, "save automatic backup setting", func() {
		a.saveSetting(
			settingItem{key: "EnableAutoBackup", title: "Automatic backup", value: fmt.Sprintf("%t", !next), choices: boolChoices},
			settingChoice{value: fmt.Sprintf("%t", next), label: label},
		)
	})
}

// openDataLogLevelPicker uses the same anchored choice menu as other settings dropdowns.
func (a *App) openDataLogLevelPicker(anchor woxui.Rect) {
	current := a.generalSettings.Data().LogLevel
	if !strings.EqualFold(current, "DEBUG") {
		current = "INFO"
	} else {
		current = "DEBUG"
	}
	labels := a.dataSettingsLabels()
	a.openSettingChoicePickerAt(settingItem{
		key: "LogLevel", title: labels.LogLevelTitle, value: current,
		choices: []settingChoice{{value: "INFO", label: labels.LogLevelInfo}, {value: "DEBUG", label: labels.LogLevelDebug}},
	}, anchor)
}

// clearDataLogs delegates to dataSettingsController.
func (a *App) clearDataLogs() {
	a.dataSettings.ClearLogs(context.Background(), a.services, a.sessionID)
}

// openDataPath delegates to dataSettingsController.
func (a *App) openDataPath(path string) {
	a.dataSettings.OpenPath(context.Background(), a.services, a.sessionID, path)
}

// openDataBackupFolder delegates to dataSettingsController.
func (a *App) openDataBackupFolder() {
	a.dataSettings.OpenBackupFolder(context.Background(), a.services, a.sessionID)
}

// openDataLog delegates to dataSettingsController.
func (a *App) openDataLog() {
	a.dataSettings.OpenLog(context.Background(), a.services, a.sessionID)
}
