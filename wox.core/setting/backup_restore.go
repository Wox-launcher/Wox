package setting

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"wox/util"

	"github.com/google/uuid"
	cp "github.com/otiai10/copy"
)

type BackupType string

const (
	BackupTypeAuto   BackupType = "auto"
	BackupTypeManual BackupType = "manual"
	BackupTypeUpdate BackupType = "update" // backup before update Wox
)

type Backup struct {
	Id        string
	Name      string // backup folder name
	Timestamp int64
	Type      BackupType
	Path      string // backup file path
}

func (m *Manager) StartAutoBackup(ctx context.Context) {
	util.Go(ctx, "backup", func() {
		for range time.NewTimer(24 * time.Hour).C {
			// Check if auto backup is enabled in settings
			settings := m.GetWoxSetting(ctx)
			if settings == nil {
				logger.Error(ctx, "failed to get settings: settings is nil")
				continue
			}

			if !settings.EnableAutoBackup.Get() {
				logger.Info(ctx, "auto backup is disabled, skipping")
				continue
			}

			backupErr := m.Backup(ctx, BackupTypeAuto)
			if backupErr != nil {
				logger.Error(ctx, fmt.Sprintf("failed to backup data: %s", backupErr.Error()))
			}
		}
	})
}

func (m *Manager) Backup(ctx context.Context, backupType BackupType) error {
	logger.Info(ctx, fmt.Sprintf("backing up data: %s", backupType))

	ts := util.GetSystemTimestamp()
	backupName := fmt.Sprintf("%d", ts)
	backupPath := path.Join(util.GetLocation().GetBackupDirectory(), backupName)
	logger.Info(ctx, fmt.Sprintf("backup path: %s", backupPath))

	err := cp.Copy(util.GetLocation().GetUserDataDirectory(), backupPath)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to backup data: %s", err.Error()))
		return err
	}

	backup := Backup{
		Id:        uuid.New().String(),
		Name:      backupName,
		Timestamp: ts,
		Type:      backupType,
	}
	marshal, marshalErr := json.Marshal(backup)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal backup data: %s", marshalErr.Error()))
		// remove backup data
		rmErr := os.RemoveAll(backupPath)
		if rmErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to remove backup data: %s", rmErr.Error()))
		}
		return marshalErr
	}

	backupInfoPath := path.Join(backupPath, "backup.json")
	writeErr := os.WriteFile(backupInfoPath, marshal, 0644)
	if writeErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to write backup info: %s", writeErr.Error()))
		// remove backup data
		rmErr := os.RemoveAll(backupPath)
		if rmErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to remove backup data: %s", rmErr.Error()))
		}
		return writeErr
	}

	logger.Info(ctx, "backup data saved successfully")

	util.Go(ctx, "clean backups", func() {
		m.cleanBackups(ctx)
	})

	return nil
}

func (m *Manager) Restore(ctx context.Context, backupId string) error {
	logger.Info(ctx, fmt.Sprintf("restoring backup data: %s", backupId))
	backups, getErr := m.FindAllBackups(ctx)
	if getErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to get all backups: %s", getErr.Error()))
		return getErr
	}

	var backupName string
	for _, backup := range backups {
		if backup.Id == backupId {
			backupName = backup.Name
			break
		}
	}
	if backupName == "" {
		logger.Error(ctx, fmt.Sprintf("backup not found: %s", backupId))
		return fmt.Errorf("backup not found: %s", backupId)
	}

	backupPath := path.Join(util.GetLocation().GetBackupDirectory(), backupName)
	if _, statErr := os.Stat(backupPath); statErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to stat backup directory: %s", statErr.Error()))
		return statErr
	}

	userDataDir := util.GetLocation().GetUserDataDirectory()
	var userDataBackupDir string
	if _, statErr := os.Stat(userDataDir); statErr == nil {
		ts := util.GetSystemTimestamp()
		candidate := fmt.Sprintf("%s.before_restore_%d", userDataDir, ts)
		userDataBackupDir = ensureUniquePath(candidate)

		renameErr := os.Rename(userDataDir, userDataBackupDir)
		if renameErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to rename user data directory: %s", renameErr.Error()))
			return renameErr
		}
		logger.Info(ctx, fmt.Sprintf("user data directory renamed to: %s", userDataBackupDir))
	} else if !os.IsNotExist(statErr) {
		logger.Error(ctx, fmt.Sprintf("failed to stat user data directory: %s", statErr.Error()))
		return statErr
	}

	cpErr := cp.Copy(backupPath, userDataDir)
	if cpErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to restore backup data to user data directory: %s", cpErr.Error()))
		if userDataBackupDir != "" {
			_ = os.RemoveAll(userDataDir)
			_ = os.Rename(userDataBackupDir, userDataDir)
		}
		return cpErr
	}

	backupInfoPath := path.Join(userDataDir, "backup.json")
	if rmErr := os.Remove(backupInfoPath); rmErr != nil && !os.IsNotExist(rmErr) {
		logger.Error(ctx, fmt.Sprintf("failed to remove restored backup info: %s", rmErr.Error()))
		return rmErr
	}

	logger.Info(ctx, "backup data restored successfully")

	return nil
}

func ensureUniquePath(candidate string) string {
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	dir := filepath.Dir(candidate)
	base := filepath.Base(candidate)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	for i := 2; ; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, i, ext))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
	}
}

func (m *Manager) FindAllBackups(ctx context.Context) ([]Backup, error) {
	var backupList []Backup = make([]Backup, 0)

	backupDir := util.GetLocation().GetBackupDirectory()
	backupDirEntries, readDirErr := os.ReadDir(backupDir)
	if readDirErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to read backup directory: %s", readDirErr.Error()))
		return nil, readDirErr
	}

	for _, entry := range backupDirEntries {
		if strings.HasPrefix(entry.Name(), "temp_") {
			continue
		}

		//  read backup info file
		backupInfoPath := path.Join(backupDir, entry.Name(), "backup.json")
		file, readErr := os.ReadFile(backupInfoPath)
		if readErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to read backup info file: %s", readErr.Error()))
			continue
		}

		var backupInfo Backup
		decodeErr := json.Unmarshal(file, &backupInfo)
		if decodeErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to unmarshal backup info: %s", decodeErr.Error()))
			continue
		}

		backupInfo.Path = path.Join(backupDir, entry.Name())
		backupList = append(backupList, backupInfo)
	}

	return backupList, nil
}

func (m *Manager) cleanBackups(ctx context.Context) error {
	logger.Info(ctx, "cleaning backups")
	maxBackups := 5

	backups, getErr := m.FindAllBackups(ctx)
	if getErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to get all backups: %s", getErr.Error()))
		return getErr
	}

	// keep 5 backups
	if len(backups) <= maxBackups {
		return nil
	}

	// sort backups by timestamp
	slices.SortFunc(backups, func(i, j Backup) int {
		return int(i.Timestamp - j.Timestamp)
	})

	// remove old backups
	removedCount := 0
	for i := 0; i < len(backups)-maxBackups; i++ {
		backup := backups[i]
		backupPath := path.Join(util.GetLocation().GetBackupDirectory(), backup.Name)
		rmErr := os.RemoveAll(backupPath)
		if rmErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to remove backup: %s", rmErr.Error()))
			continue
		} else {
			removedCount++
			logger.Info(ctx, fmt.Sprintf("backup removed: %s, date: %s", backup.Id, util.FormatTimestamp(backup.Timestamp)))
		}
	}

	logger.Info(ctx, fmt.Sprintf("backups cleaned successfully, removed count: %d", removedCount))
	return nil
}
