package ui

import (
	"context"
	"fmt"
	"os"

	"wox/common"
	"wox/i18n"
	"wox/setting"
	"wox/ui/contract"
	"wox/util"
	"wox/util/shell"
)

// DataLocation returns the active user-data directory.
func (s *CoreServices) DataLocation(_ context.Context, _ string) (string, error) {
	return util.GetLocation().GetUserDataDirectory(), nil
}

// DataBackups returns all settings backups in their persisted representation.
func (s *CoreServices) DataBackups(ctx context.Context, sessionID string) ([]contract.DataBackup, error) {
	backups, err := setting.GetSettingManager().FindAllBackups(uiServiceContext(ctx, sessionID))
	if err != nil {
		return nil, err
	}
	converted := make([]contract.DataBackup, len(backups))
	for index, backup := range backups {
		converted[index] = contract.DataBackup{
			ID:        backup.Id,
			Name:      backup.Name,
			Timestamp: backup.Timestamp,
			Type:      string(backup.Type),
			Path:      backup.Path,
		}
	}
	return converted, nil
}

// CreateDataBackup creates one manual settings backup.
func (s *CoreServices) CreateDataBackup(ctx context.Context, sessionID string) error {
	return setting.GetSettingManager().Backup(uiServiceContext(ctx, sessionID), setting.BackupTypeManual)
}

// RestoreDataBackup replaces current settings with one persisted backup.
func (s *CoreServices) RestoreDataBackup(ctx context.Context, sessionID string, backupID string) error {
	return setting.GetSettingManager().Restore(uiServiceContext(ctx, sessionID), backupID)
}

// ChangeDataLocation moves user-managed data through the core UI manager.
func (s *CoreServices) ChangeDataLocation(ctx context.Context, sessionID string, location string) error {
	return GetUIManager().ChangeUserDataDirectory(uiServiceContext(ctx, sessionID), location)
}

// ClearLogs removes historical logs and preserves the existing user notification behavior.
func (s *CoreServices) ClearLogs(ctx context.Context, sessionID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	if err := util.GetLogger().ClearHistory(); err != nil {
		GetUIManager().GetUI(ctx).Notify(ctx, common.NotifyMsg{
			Icon:           common.WoxIcon.String(),
			Text:           fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "ui_data_log_clear_notify_failed"), err.Error()),
			DisplaySeconds: 6,
		})
		return err
	}
	GetUIManager().GetUI(ctx).Notify(ctx, common.NotifyMsg{
		Icon:           common.WoxIcon.String(),
		Text:           i18n.GetI18nManager().TranslateWox(ctx, "ui_data_log_clear_notify_success"),
		DisplaySeconds: 4,
	})
	return nil
}

// OpenPath delegates path opening to the platform shell adapter.
func (s *CoreServices) OpenPath(_ context.Context, _ string, path string) error {
	return shell.Open(path)
}

// BackupFolder ensures and returns the configured backup directory.
func (s *CoreServices) BackupFolder(_ context.Context, _ string) (string, error) {
	backupDir := util.GetLocation().GetBackupDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(backupDir); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}
	return backupDir, nil
}

// OpenLog creates and reveals the current log file.
func (s *CoreServices) OpenLog(_ context.Context, _ string) error {
	logFile := util.GetLogger().CurrentLogPath()
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return shell.OpenFileInFolder(logFile)
}
