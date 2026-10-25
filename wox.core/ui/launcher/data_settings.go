package launcher

import (
	"context"
	"fmt"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
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
func (c *dataSettingsController) buildDataSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	backups := make([]launcherview.DataBackup, len(snapshot.dataState.Backups))
	for index, backup := range snapshot.dataState.Backups {
		backups[index] = launcherview.DataBackup{ID: backup.ID, Timestamp: backup.Timestamp, Type: backup.Type, Path: backup.Path}
	}
	return launcherview.DataSettingsView(launcherview.DataSettingsProps{
		Width: width, Height: height, Theme: snapshot.palette, Labels: c.dataSettingsLabels(),
		Location: snapshot.dataState.Location, PendingLocation: snapshot.dataState.PendingLocation, AutoBackup: snapshot.general.Data.EnableAutoBackup,
		Backups: backups, BackupRunning: snapshot.dataState.BackupRunning || snapshot.dataState.Busy == "backup",
		DeletingBackupID: snapshot.dataState.DeletingBackupID,
		LogLevel:         snapshot.general.Data.LogLevel, ClearLogsArmed: snapshot.dataState.ClearLogsArmed,
		Error: snapshot.dataState.Error, ErrorSection: snapshot.dataState.ErrorSection,
		OnOpenPath: c.openDataPath, OnChooseLocation: c.chooseDataLocation, OnCancelLocation: c.cancelDataLocationChange,
		OnConfirmLocation: c.confirmDataLocationChange, OnToggleAutoBackup: c.toggleDataAutoBackup, OnCreateBackup: c.createDataBackup,
		OnRestoreBackup: c.restoreDataBackup, OnDeleteBackup: c.deleteDataBackup, OnOpenLogLevel: c.openDataLogLevelPicker, OnClearLogs: c.clearDataLogs, OnOpenLog: c.openDataLog,
	})
}

// dataSettingsLabels resolves all user-facing copy before entering the view layer.
func (c *dataSettingsController) dataSettingsLabels() launcherview.DataSettingsLabels {
	return launcherview.DataSettingsLabels{
		Title:                 c.language.translate("i18n:ui_data"),
		Description:           c.language.translate("i18n:ui_data_description"),
		StorageSection:        c.language.translate("i18n:ui_data_section_storage"),
		BackupSection:         c.language.translate("i18n:ui_data_section_backup"),
		LogsSection:           c.language.translate("i18n:ui_data_section_logs"),
		Open:                  c.language.translate("i18n:plugin_file_open"),
		Cancel:                c.language.translate("i18n:ui_cancel"),
		LocationChange:        c.language.translate("i18n:ui_data_config_location_change"),
		LocationChangeConfirm: c.language.translate("i18n:ui_data_config_location_change_confirm_button"),
		LocationTitle:         c.language.translate("i18n:ui_data_config_location"),
		LocationDescription:   c.language.translate("i18n:ui_data_config_location_tips"),
		AutoBackupTitle:       c.language.translate("i18n:ui_data_backup_auto_title"),
		AutoBackupDescription: c.language.translate("i18n:ui_data_backup_auto_tips"),
		BackupListTitle:       c.language.translate("i18n:ui_data_backup_list_title"),
		BackupNow:             c.language.translate("i18n:ui_data_backup_now"),
		BackupInProgress:      c.language.translate("i18n:ui_data_backup_in_progress"),
		BackupEmpty:           c.language.translate("i18n:ui_data_backup_empty"),
		BackupDate:            c.language.translate("i18n:ui_data_backup_date"),
		BackupType:            c.language.translate("i18n:ui_data_backup_type"),
		BackupOperation:       c.language.translate("i18n:ui_operation"),
		BackupTypeManual:      c.language.translate("i18n:ui_data_backup_type_manual"),
		BackupTypeAuto:        c.language.translate("i18n:ui_data_backup_type_auto"),
		BackupRestore:         c.language.translate("i18n:ui_data_backup_restore"),
		BackupRestoreConfirm:  c.language.translate("i18n:ui_data_backup_restore_confirm"),
		BackupDelete:          c.language.translate("i18n:ui_delete"),
		BackupDeleteConfirm:   c.language.translate("i18n:ui_data_backup_restore_confirm"),
		BackupDeleting:        c.language.translate("i18n:ui_data_backup_deleting"),
		LogLevelTitle:         c.language.translate("i18n:ui_data_log_level_title"),
		LogLevelDescription:   c.language.translate("i18n:ui_data_log_level_tips"),
		LogLevelInfo:          c.language.translate("i18n:ui_data_log_level_info"),
		LogLevelDebug:         c.language.translate("i18n:ui_data_log_level_debug"),
		LogClearButton:        c.language.translate("i18n:ui_data_log_clear_button"),
		LogClearConfirm:       c.language.translate("i18n:ui_data_log_clear_confirm"),
		LogClearTitle:         c.language.translate("i18n:ui_data_log_clear_title"),
		LogClearDescription:   c.language.translate("i18n:ui_data_log_clear_tips"),
		LogOpenButton:         c.language.translate("i18n:ui_data_log_open_button"),
	}
}

// reloadDataSettings delegates to dataSettingsController so App no longer holds data state directly.
func (c *dataSettingsController) reloadDataSettings() {
	c.Reload(context.Background(), c.services, c.sessionID)
}

// createDataBackup delegates to dataSettingsController.
func (c *dataSettingsController) createDataBackup() {
	c.CreateBackup(context.Background(), c.services, c.sessionID)
}

// restoreDataBackup delegates to dataSettingsController.
func (c *dataSettingsController) restoreDataBackup(id string) {
	c.RestoreBackup(context.Background(), c.services, c.sessionID, id)
}

// deleteDataBackup delegates to dataSettingsController.
func (c *dataSettingsController) deleteDataBackup(id string) {
	c.DeleteBackup(context.Background(), c.services, c.sessionID, id)
}

// chooseDataLocation delegates to dataSettingsController, which uses the injected
// native directory picker.
func (c *dataSettingsController) chooseDataLocation() {
	c.ChooseLocation()
}

func (c *dataSettingsController) cancelDataLocationChange() {
	c.CancelLocationChange()
}

// confirmDataLocationChange delegates to dataSettingsController.
func (c *dataSettingsController) confirmDataLocationChange() {
	c.ConfirmLocationChange(context.Background(), c.services, c.sessionID)
}

// toggleDataAutoBackup reuses the regular key-value settings save and rollback behavior.
// Stays on App because it operates on the general-domain EnableAutoBackup setting.
func (c *dataSettingsController) toggleDataAutoBackup() {
	if c.settings.saving {
		return
	}
	next := !c.generalSettings().Data().EnableAutoBackup
	label := "Off"
	if next {
		label = "On"
	}
	c.settings.invalidateSettingsWindow()
	c.startGeneralSettingSave(
		"save automatic backup setting",
		settingItem{key: "EnableAutoBackup", title: "Automatic backup", value: fmt.Sprintf("%t", !next), choices: boolChoices},
		settingChoice{value: fmt.Sprintf("%t", next), label: label},
	)
}

// openDataLogLevelPicker uses the same anchored choice menu as other settings dropdowns.
func (c *dataSettingsController) openDataLogLevelPicker(anchor woxui.Rect) {
	current := c.generalSettings().Data().LogLevel
	if !strings.EqualFold(current, "DEBUG") {
		current = "INFO"
	} else {
		current = "DEBUG"
	}
	labels := c.dataSettingsLabels()
	c.openSettingChoicePickerAt(settingItem{
		key: "LogLevel", title: labels.LogLevelTitle, value: current,
		choices: []settingChoice{{value: "INFO", label: labels.LogLevelInfo}, {value: "DEBUG", label: labels.LogLevelDebug}},
	}, anchor)
}

// clearDataLogs delegates to dataSettingsController.
func (c *dataSettingsController) clearDataLogs() {
	c.ClearLogs(context.Background(), c.services, c.sessionID)
}

// openDataPath delegates to dataSettingsController.
func (c *dataSettingsController) openDataPath(path, section string) {
	c.OpenPath(context.Background(), c.services, c.sessionID, path, section)
}

// openDataBackupFolder delegates to dataSettingsController.
func (c *dataSettingsController) openDataBackupFolder() {
	c.OpenBackupFolder(context.Background(), c.services, c.sessionID)
}

// openDataLog delegates to dataSettingsController.
func (c *dataSettingsController) openDataLog() {
	c.OpenLog(context.Background(), c.services, c.sessionID)
}
