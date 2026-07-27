package launcher

import (
	"context"
	"fmt"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
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
		Note: snapshot.note, Loading: snapshot.dataState.Loading, Error: snapshot.dataState.Error,
		OnOpenPath: a.openDataPath, OnChooseLocation: a.chooseDataLocation, OnCancelLocation: a.cancelDataLocationChange,
		OnConfirmLocation: a.confirmDataLocationChange, OnToggleAutoBackup: a.toggleDataAutoBackup, OnCreateBackup: a.createDataBackup,
		OnRestoreBackup: a.restoreDataBackup, OnCycleLogLevel: a.cycleDataLogLevel, OnClearLogs: a.clearDataLogs, OnOpenLog: a.openDataLog,
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
		LogClearButton:        a.translate("i18n:ui_data_log_clear_button"),
		LogClearConfirm:       a.translate("i18n:ui_data_log_clear_confirm"),
		LogClearTitle:         a.translate("i18n:ui_data_log_clear_title"),
		LogClearDescription:   a.translate("i18n:ui_data_log_clear_tips"),
		LogOpenButton:         a.translate("i18n:ui_data_log_open_button"),
		Loading:               "Loading storage and backups…",
	}
}

// reloadDataSettings delegates to dataSettingsController so App no longer holds data state directly.
func (a *App) reloadDataSettings() {
	a.dataSettings.Reload(context.Background(), a.client)
}

// createDataBackup delegates to dataSettingsController.
func (a *App) createDataBackup() {
	a.dataSettings.CreateBackup(context.Background(), a.client)
}

// restoreDataBackup delegates to dataSettingsController.
func (a *App) restoreDataBackup(id string) {
	a.dataSettings.RestoreBackup(context.Background(), a.client, id)
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
	a.dataSettings.ConfirmLocationChange(context.Background(), a.client)
}

// toggleDataAutoBackup reuses the regular key-value settings save and rollback behavior.
// Stays on App because it operates on the general-domain EnableAutoBackup setting and
// the shared settingSaving/settingNote/saveSetting machinery.
func (a *App) toggleDataAutoBackup() {
	a.mu.Lock()
	if a.settingSaving {
		a.mu.Unlock()
		return
	}
	next := !a.generalSettings.Data().EnableAutoBackup
	label := "Off"
	if next {
		label = "On"
	}
	a.settingSaving = true
	a.settingNote = "Saving Automatic backup…"
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	go a.saveSetting(
		settingItem{key: "EnableAutoBackup", title: "Automatic backup", value: fmt.Sprintf("%t", !next), choices: boolChoices},
		settingChoice{value: fmt.Sprintf("%t", next), label: label},
	)
}

// cycleDataLogLevel keeps the compact page to the two log levels accepted by core.
// Stays on App for the same reason as toggleDataAutoBackup: it edits the general-domain
// LogLevel setting through the shared save flow.
func (a *App) cycleDataLogLevel() {
	a.mu.Lock()
	if a.settingSaving {
		a.mu.Unlock()
		return
	}
	current := a.generalSettings.Data().LogLevel
	next := "DEBUG"
	if strings.EqualFold(current, "DEBUG") {
		next = "INFO"
	}
	a.settingSaving = true
	a.settingNote = "Saving Log level…"
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	go a.saveSetting(
		settingItem{key: "LogLevel", title: "Log level", value: current, choices: []settingChoice{{"INFO", "Info"}, {"DEBUG", "Debug"}}},
		settingChoice{value: next, label: strings.ToLower(next)},
	)
}

// clearDataLogs delegates to dataSettingsController.
func (a *App) clearDataLogs() {
	a.dataSettings.ClearLogs(context.Background(), a.client)
}

// openDataPath delegates to dataSettingsController.
func (a *App) openDataPath(path string) {
	a.dataSettings.OpenPath(context.Background(), a.client, path)
}

// openDataBackupFolder delegates to dataSettingsController.
func (a *App) openDataBackupFolder() {
	a.dataSettings.OpenBackupFolder(context.Background(), a.client)
}

// openDataLog delegates to dataSettingsController.
func (a *App) openDataLog() {
	a.dataSettings.OpenLog(context.Background(), a.client)
}
