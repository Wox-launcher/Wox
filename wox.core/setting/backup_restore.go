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
	"wox/database"
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

const (
	// autoBackupStartupDelay keeps the copy off the startup path.
	autoBackupStartupDelay = time.Minute
	// autoBackupRetryInterval is how soon a failed daily backup is tried again.
	autoBackupRetryInterval = time.Hour
)

// StartAutoBackup starts one loop that keeps a single automatic backup per local day.
// time.Timer fires once, so each wait creates a new timer. The first check runs shortly
// after startup, which still backs up after a restart that never stays up for 24 hours.
func (m *Manager) StartAutoBackup(ctx context.Context) {
	if util.IsTestMode() {
		return
	}
	m.autoBackupOnce.Do(func() {
		util.Go(ctx, "backup", func() {
			if !waitForAutoBackup(ctx, autoBackupStartupDelay) {
				return
			}
			for {
				retry := m.backupIfDue(ctx)
				wait := durationUntilNextLocalMidnight(time.Now())
				if retry && autoBackupRetryInterval < wait {
					wait = autoBackupRetryInterval
				}
				if !waitForAutoBackup(ctx, wait) {
					return
				}
			}
		})
	})
}

func waitForAutoBackup(ctx context.Context, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		timer.Stop()
		return false
	}
}

// backupIfDue creates today's automatic backup when the setting is on and today does not have one yet.
// It returns true when the attempt failed and should be retried before the next midnight.
func (m *Manager) backupIfDue(ctx context.Context) bool {
	settings := m.GetWoxSetting(ctx)
	if settings == nil {
		logger.Error(ctx, "failed to get settings: settings is nil")
		return true
	}
	if !settings.EnableAutoBackup.Get() {
		logger.Info(ctx, "auto backup is disabled, skipping")
		return false
	}

	backups, err := m.FindAllBackups(ctx)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to list backups before auto backup: %s", err.Error()))
		return true
	}
	if hasAutoBackupOnLocalDay(backups, time.Now()) {
		logger.Info(ctx, "auto backup already exists for today, skipping")
		return false
	}

	if backupErr := m.Backup(ctx, BackupTypeAuto); backupErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to backup data: %s", backupErr.Error()))
		return true
	}
	return false
}

// hasAutoBackupOnLocalDay reports whether an automatic backup already falls on now's local calendar day.
func hasAutoBackupOnLocalDay(backups []Backup, now time.Time) bool {
	year, month, day := now.Date()
	location := now.Location()
	for _, backup := range backups {
		if backup.Type != BackupTypeAuto {
			continue
		}
		backupYear, backupMonth, backupDay := time.UnixMilli(backup.Timestamp).In(location).Date()
		if backupYear == year && backupMonth == month && backupDay == day {
			return true
		}
	}
	return false
}

func durationUntilNextLocalMidnight(now time.Time) time.Duration {
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return nextMidnight.Sub(now)
}

func (m *Manager) Backup(ctx context.Context, backupType BackupType) error {
	logger.Info(ctx, fmt.Sprintf("backing up data: %s", backupType))

	ts := util.GetSystemTimestamp()
	backupName := fmt.Sprintf("%d", ts)
	backupPath := path.Join(util.GetLocation().GetBackupDirectory(), backupName)
	logger.Info(ctx, fmt.Sprintf("backup path: %s", backupPath))

	err := database.WithFileMaintenance(func() error {
		return cp.Copy(util.GetLocation().GetUserDataDirectory(), backupPath)
	})
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
	return RestoreUserDataDirectory(ctx, backupPath, util.GetLocation().GetUserDataDirectory())
}

const (
	restoreRenameAttempts = 25
	restoreRenameInterval = 200 * time.Millisecond
)

// RestoreUserDataDirectory replaces userDataDir with the backup directory.
// Callers must already have released every handle inside userDataDir. Windows
// refuses to rename a directory while any file in it is open, so the supervisor
// runs this only after Wox has exited. The rename is retried briefly because
// plugin processes can disappear a moment after the main process.
func RestoreUserDataDirectory(ctx context.Context, backupPath string, userDataDir string) error {
	backupPath = filepath.Clean(backupPath)
	userDataDir = filepath.Clean(userDataDir)
	if backupPath == "" || userDataDir == "" {
		return fmt.Errorf("backup path and user data directory are required")
	}
	if backupPath == userDataDir {
		return fmt.Errorf("backup path and user data directory are the same")
	}
	if _, statErr := os.Stat(backupPath); statErr != nil {
		logRestore(ctx, fmt.Sprintf("failed to stat backup directory: %s", statErr.Error()))
		return statErr
	}

	var userDataBackupDir string
	if _, statErr := os.Stat(userDataDir); statErr == nil {
		candidate := fmt.Sprintf("%s.before_restore_%d", userDataDir, util.GetSystemTimestamp())
		userDataBackupDir = ensureUniquePath(candidate)
		if renameErr := renameForRestore(userDataDir, userDataBackupDir); renameErr != nil {
			logRestore(ctx, fmt.Sprintf("failed to rename user data directory: %s", renameErr.Error()))
			return renameErr
		}
		logRestore(ctx, fmt.Sprintf("user data directory renamed to: %s", userDataBackupDir))
	} else if !os.IsNotExist(statErr) {
		logRestore(ctx, fmt.Sprintf("failed to stat user data directory: %s", statErr.Error()))
		return statErr
	}

	if cpErr := cp.Copy(backupPath, userDataDir); cpErr != nil {
		logRestore(ctx, fmt.Sprintf("failed to restore backup data to user data directory: %s", cpErr.Error()))
		if userDataBackupDir != "" {
			_ = os.RemoveAll(userDataDir)
			_ = os.Rename(userDataBackupDir, userDataDir)
		}
		return cpErr
	}

	backupInfoPath := filepath.Join(userDataDir, "backup.json")
	if rmErr := os.Remove(backupInfoPath); rmErr != nil && !os.IsNotExist(rmErr) {
		logRestore(ctx, fmt.Sprintf("failed to remove restored backup info: %s", rmErr.Error()))
		return rmErr
	}

	logRestore(ctx, "backup data restored successfully")
	return nil
}

func renameForRestore(from string, to string) error {
	var renameErr error
	for attempt := 1; attempt <= restoreRenameAttempts; attempt++ {
		renameErr = os.Rename(from, to)
		if renameErr == nil {
			return nil
		}
		if attempt == restoreRenameAttempts {
			break
		}
		time.Sleep(restoreRenameInterval)
	}
	return renameErr
}

func logRestore(ctx context.Context, message string) {
	// Tests point the data directory at a temporary folder. Opening the log there
	// keeps the file locked until the process exits and makes cleanup fail.
	if util.GetLocation().GetWoxDataDirectory() == "" || util.IsTestMode() {
		return
	}
	util.GetLogger().Info(ctx, message)
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
