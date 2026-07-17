package launcher

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const dataBackupRowHeight = float32(46)

type backupInfo struct {
	ID        string `json:"Id"`
	Name      string `json:"Name"`
	Timestamp int64  `json:"Timestamp"`
	Type      string `json:"Type"`
	Path      string `json:"Path"`
}

// buildDataSettingsPage adapts controller state to the package-independent data settings view.
func (a *App) buildDataSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	backups := make([]launcherview.DataBackup, len(snapshot.dataBackups))
	for index, backup := range snapshot.dataBackups {
		backups[index] = launcherview.DataBackup{ID: backup.ID, Timestamp: backup.Timestamp, Type: backup.Type, Path: backup.Path}
	}
	return launcherview.DataSettingsView(launcherview.DataSettingsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(), Labels: a.dataSettingsLabels(),
		Location: snapshot.dataLocation, PendingLocation: snapshot.dataPendingLocation, AutoBackup: snapshot.data.EnableAutoBackup,
		Backups: backups, RestoreArmed: snapshot.dataRestoreArmed, LogLevel: snapshot.data.LogLevel, ClearLogsArmed: snapshot.dataClearLogsArmed,
		PageScroll: snapshot.pageScroll.offset, Note: snapshot.note, Loading: snapshot.dataLoading, Error: snapshot.dataError,
		OnOpenPath: a.openDataPath, OnChooseLocation: a.chooseDataLocation, OnCancelLocation: a.cancelDataLocationChange,
		OnConfirmLocation: a.confirmDataLocationChange, OnToggleAutoBackup: a.toggleDataAutoBackup, OnCreateBackup: a.createDataBackup,
		OnRestoreBackup: a.restoreDataBackup, OnCycleLogLevel: a.cycleDataLogLevel, OnClearLogs: a.clearDataLogs, OnOpenLog: a.openDataLog,
		OnScroll: a.scrollSettingsPage,
		OnSetPageGeometry: func(viewportHeight, contentHeight float32) {
			a.setSettingsPageGeometry(viewportHeight, contentHeight)
		},
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

// reloadDataSettings refreshes the storage location and backup catalog through existing core routes.
func (a *App) reloadDataSettings() {
	a.mu.Lock()
	if a.dataLoading {
		a.mu.Unlock()
		return
	}
	a.dataLoading = true
	a.dataError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var location string
	var backups []backupInfo
	locationErr := a.client.Post(ctx, "/setting/userdata/location", map[string]any{}, &location)
	backupsErr := a.client.Post(ctx, "/backup/all", map[string]any{}, &backups)
	sort.SliceStable(backups, func(i, j int) bool { return backups[i].Timestamp > backups[j].Timestamp })

	errorText := ""
	if locationErr != nil {
		errorText = "load data location: " + locationErr.Error()
	}
	if backupsErr != nil {
		if errorText != "" {
			errorText += " · "
		}
		errorText += "load backups: " + backupsErr.Error()
	}
	a.mu.Lock()
	a.dataLoading = false
	a.dataLoaded = errorText == ""
	if locationErr == nil {
		a.dataLocation = location
	}
	if backupsErr == nil {
		a.dataBackups = backups
		a.clampDataBackupScrollLocked()
	}
	a.dataError = errorText
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// createDataBackup runs the potentially slow archive operation away from the UI event loop.
func (a *App) createDataBackup() {
	a.mu.Lock()
	if a.dataBusy != "" {
		a.mu.Unlock()
		return
	}
	a.dataBusy = "backup"
	a.dataError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		err := a.client.Post(ctx, "/backup/now", map[string]any{}, nil)
		cancel()
		a.mu.Lock()
		a.dataBusy = ""
		if err != nil {
			a.dataError = "Could not create backup: " + err.Error()
		} else {
			a.settingNote = "Manual backup created"
		}
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		if err == nil {
			a.reloadDataSettings()
		}
	}()
}

// restoreDataBackup requires two explicit activations before core replaces current settings.
func (a *App) restoreDataBackup(id string) {
	a.mu.Lock()
	if a.dataBusy != "" || strings.TrimSpace(id) == "" {
		a.mu.Unlock()
		return
	}
	if a.dataRestoreArmed != id {
		a.dataRestoreArmed = id
		a.settingNote = "Press Confirm restore to replace current settings with this backup."
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		return
	}
	a.dataRestoreArmed = ""
	a.dataBusy = "restore"
	a.dataError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		err := a.client.Post(ctx, "/backup/restore", map[string]string{"id": id}, nil)
		cancel()
		if err == nil {
			err = a.reloadSettings()
		}
		a.mu.Lock()
		a.dataBusy = ""
		if err != nil {
			a.dataError = "Could not restore backup: " + err.Error()
		} else {
			a.settingNote = "Backup restored"
		}
		a.mu.Unlock()
		a.invalidateSettingsWindow()
	}()
}

// chooseDataLocation separates native directory selection from the destructive move confirmation.
func (a *App) chooseDataLocation() {
	path, err := a.settingsNativeWindow().PickFile(woxui.FileDialogOptions{Directory: true})
	a.mu.Lock()
	if err != nil {
		a.dataError = "Could not select data directory: " + err.Error()
	} else if strings.TrimSpace(path) != "" && path != a.dataLocation {
		a.dataPendingLocation = path
		a.settingNote = "Confirm the new data directory before Wox moves its files."
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) cancelDataLocationChange() {
	a.mu.Lock()
	a.dataPendingLocation = ""
	a.settingNote = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// confirmDataLocationChange delegates the actual migration to core after the visible confirmation step.
func (a *App) confirmDataLocationChange() {
	a.mu.Lock()
	location := a.dataPendingLocation
	if a.dataBusy != "" || strings.TrimSpace(location) == "" {
		a.mu.Unlock()
		return
	}
	a.dataPendingLocation = ""
	a.dataBusy = "location"
	a.dataError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err := a.client.Post(ctx, "/setting/userdata/location/update", map[string]string{"location": location}, nil)
		cancel()
		a.mu.Lock()
		a.dataBusy = ""
		if err != nil {
			a.dataPendingLocation = location
			a.dataError = "Could not move data directory: " + err.Error()
		} else {
			a.dataLocation = location
			a.settingNote = "Data directory updated"
		}
		a.mu.Unlock()
		a.invalidateSettingsWindow()
	}()
}

// toggleDataAutoBackup reuses the regular key-value settings save and rollback behavior.
func (a *App) toggleDataAutoBackup() {
	a.mu.Lock()
	if a.settingSaving {
		a.mu.Unlock()
		return
	}
	next := !a.settings.EnableAutoBackup
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
func (a *App) cycleDataLogLevel() {
	a.mu.Lock()
	if a.settingSaving {
		a.mu.Unlock()
		return
	}
	current := a.settings.LogLevel
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

// clearDataLogs uses the same two-step confirmation as backup restore to avoid accidental data loss.
func (a *App) clearDataLogs() {
	a.mu.Lock()
	if a.dataBusy != "" {
		a.mu.Unlock()
		return
	}
	if !a.dataClearLogsArmed {
		a.dataClearLogsArmed = true
		a.settingNote = "Press Confirm clear to delete historical logs."
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		return
	}
	a.dataClearLogsArmed = false
	a.dataBusy = "logs"
	a.dataError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := a.client.Post(ctx, "/log/clear", map[string]any{}, nil)
		cancel()
		a.mu.Lock()
		a.dataBusy = ""
		if err != nil {
			a.dataError = "Could not clear logs: " + err.Error()
		} else {
			a.settingNote = "Logs cleared"
		}
		a.mu.Unlock()
		a.invalidateSettingsWindow()
	}()
}

// openDataPath delegates platform shell behavior to core's existing cross-platform route.
func (a *App) openDataPath(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.client.Post(ctx, "/open", map[string]string{"path": path}, nil)
		cancel()
		if err != nil {
			a.mu.Lock()
			a.dataError = "Could not open path: " + err.Error()
			a.mu.Unlock()
			a.invalidateSettingsWindow()
		}
	}()
}

// openDataBackupFolder resolves the configured folder in core before asking the desktop to open it.
func (a *App) openDataBackupFolder() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		var path string
		err := a.client.Post(ctx, "/backup/folder", map[string]any{}, &path)
		cancel()
		if err != nil {
			a.mu.Lock()
			a.dataError = "Could not open backup folder: " + err.Error()
			a.mu.Unlock()
			a.invalidateSettingsWindow()
			return
		}
		a.openDataPath(path)
	}()
}

// openDataLog lets core create and reveal the current log file with its platform shell adapter.
func (a *App) openDataLog() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.client.Post(ctx, "/log/open", map[string]any{}, nil)
		cancel()
		if err != nil {
			a.mu.Lock()
			a.dataError = "Could not open log: " + err.Error()
			a.mu.Unlock()
			a.invalidateSettingsWindow()
		}
	}()
}

func (a *App) setDataBackupViewport(height float32) {
	a.mu.Lock()
	a.dataListViewport = max(float32(1), height)
	a.clampDataBackupScrollLocked()
	a.mu.Unlock()
}

func (a *App) scrollDataBackups(delta float32) {
	a.mu.Lock()
	a.dataListScroll += delta
	a.clampDataBackupScrollLocked()
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) clampDataBackupScrollLocked() {
	maxOffset := max(float32(0), float32(len(a.dataBackups))*dataBackupRowHeight-max(float32(1), a.dataListViewport))
	a.dataListScroll = min(max(float32(0), a.dataListScroll), maxOffset)
}
