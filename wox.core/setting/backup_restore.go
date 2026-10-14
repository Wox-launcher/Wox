package setting

import (
	"context"
	"encoding/json"
	"errors"
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
		// The in-flight run is this attempt. Retrying in an hour would start a second copy.
		if errors.Is(backupErr, ErrBackupInProgress) {
			logger.Info(ctx, "auto backup skipped because another backup is already running")
			return false
		}
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

type backupWatch struct {
	id int
	fn func(running bool, failure error)
}

// ErrBackupInProgress is returned when Backup is called while another backup is still copying.
var ErrBackupInProgress = errors.New("backup already in progress")

// beginBackup starts the single in-flight backup.
// Listeners are notified after the lock is released. A second caller gets false and no notification.
func (m *Manager) beginBackup() bool {
	m.backupMu.Lock()
	if m.backupRunning {
		m.backupMu.Unlock()
		return false
	}
	m.backupRunning = true
	watchers := slices.Clone(m.backupWatchers)
	m.backupMu.Unlock()
	notifyBackupWatchers(watchers, true, nil)
	return true
}

// endBackup marks the in-flight backup finished and reports its result to listeners.
func (m *Manager) endBackup(err error) {
	m.backupMu.Lock()
	m.backupRunning = false
	watchers := slices.Clone(m.backupWatchers)
	m.backupMu.Unlock()
	notifyBackupWatchers(watchers, false, err)
}

// BackupRunning reports whether a backup copy is in progress.
func (m *Manager) BackupRunning() bool {
	m.backupMu.Lock()
	defer m.backupMu.Unlock()
	return m.backupRunning
}

// WatchBackup reports the current run immediately and on every later change.
// Callbacks run without the backup lock. The returned function removes the watcher.
func (m *Manager) WatchBackup(fn func(bool, error)) func() {
	if fn == nil {
		return func() {}
	}
	m.backupMu.Lock()
	m.backupWatchSeq++
	id := m.backupWatchSeq
	running := m.backupRunning
	m.backupWatchers = append(m.backupWatchers, backupWatch{id: id, fn: fn})
	m.backupMu.Unlock()
	fn(running, nil)
	return func() {
		m.backupMu.Lock()
		defer m.backupMu.Unlock()
		kept := m.backupWatchers[:0]
		for _, watcher := range m.backupWatchers {
			if watcher.id != id {
				kept = append(kept, watcher)
			}
		}
		m.backupWatchers = kept
	}
}

// notifyBackupWatchers delivers one run change without holding the backup lock.
func notifyBackupWatchers(watchers []backupWatch, running bool, failure error) {
	for _, watcher := range watchers {
		if watcher.fn != nil {
			watcher.fn(running, failure)
		}
	}
}

func (m *Manager) Backup(ctx context.Context, backupType BackupType) (err error) {
	if !m.beginBackup() {
		if logger != nil {
			logger.Info(ctx, "backup already in progress, skipping")
		}
		return ErrBackupInProgress
	}
	defer func() { m.endBackup(err) }()

	logger.Info(ctx, fmt.Sprintf("backing up data: %s", backupType))

	userDataDir := util.GetLocation().GetUserDataDirectory()
	backupRoot := util.GetLocation().GetBackupDirectory()
	ts := util.GetSystemTimestamp()
	backupName := fmt.Sprintf("%d", ts)
	backupPath := filepath.Join(backupRoot, backupName)
	logger.Info(ctx, fmt.Sprintf("backup path: %s", backupPath))

	err = database.WithFileMaintenance(func() error {
		// Incomplete directories have no backup.json, so the daily check keeps
		// retrying and leaving another copy. Remove them before this attempt.
		removed, cleanErr := removeIncompleteBackups(backupRoot)
		if cleanErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to remove incomplete backups: %s", cleanErr.Error()))
		}
		if removed > 0 {
			logger.Info(ctx, fmt.Sprintf("removed %d incomplete backups", removed))
		}
		if copyErr := copyUserDataForBackup(ctx, userDataDir, backupPath); copyErr != nil {
			return copyErr
		}
		// backup.json is written before this lock is released. A later backup
		// treats a timestamp directory without it as incomplete and deletes it.
		payload, marshalErr := json.Marshal(Backup{
			Id:        uuid.New().String(),
			Name:      backupName,
			Timestamp: ts,
			Type:      backupType,
		})
		if marshalErr != nil {
			return marshalErr
		}
		return os.WriteFile(filepath.Join(backupPath, "backup.json"), payload, 0644)
	})
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to backup data: %s", err.Error()))
		if rmErr := os.RemoveAll(backupPath); rmErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to remove partial backup: %s", rmErr.Error()))
		}
		return err
	}

	logger.Info(ctx, "backup data saved successfully")

	util.Go(ctx, "clean backups", func() {
		m.cleanBackups(ctx)
	})

	return nil
}

// copyUserDataForBackup copies user data into backupPath.
// Junctions and other non-regular files are not opened. A Windows directory junction is an
// irregular file in Go 1.23 and later, and reading it fails with "Incorrect function".
// Flow plugins point those junctions back into plugins/, so following them would also loop.
// A later failure inside plugins/ is skipped. The database and the rest of the user data
// still have to copy, so one plugin cannot reject the whole backup.
func copyUserDataForBackup(ctx context.Context, userDataDir, backupPath string) error {
	return cp.Copy(userDataDir, backupPath, cp.Options{
		OnSymlink: func(string) cp.SymlinkAction { return cp.Skip },
		Skip: func(info os.FileInfo, srcPath, _ string) (bool, error) {
			if !backupEntrySkipped(info) {
				return false, nil
			}
			if logger != nil {
				logger.Info(ctx, fmt.Sprintf("skipped link during backup: %s", srcPath))
			}
			return true, nil
		},
		OnError: func(srcPath, destPath string, err error) error {
			if err == nil || !pluginBackupPath(userDataDir, srcPath) {
				return err
			}
			if logger != nil {
				logger.Warn(ctx, fmt.Sprintf("skipped plugin path during backup: %s: %s", srcPath, err.Error()))
			}
			if filepath.Clean(destPath) != filepath.Clean(backupPath) {
				_ = os.RemoveAll(destPath)
			}
			return nil
		},
	})
}

// backupEntrySkipped reports entries backup must not open.
func backupEntrySkipped(info os.FileInfo) bool {
	return !info.Mode().IsRegular() && !info.IsDir()
}

// pluginBackupPath reports whether src is the plugins directory or something inside it.
func pluginBackupPath(userDataDir, src string) bool {
	rel, err := filepath.Rel(filepath.Clean(userDataDir), filepath.Clean(src))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return rel == "plugins" || strings.HasPrefix(rel, "plugins"+string(filepath.Separator))
}

// removeIncompleteBackups deletes timestamp backup directories that never received backup.json.
// Other directories in the backup folder, including db_repair_*, are left alone.
func removeIncompleteBackups(backupDir string) (int, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	removed := 0
	var firstErr error
	for _, entry := range entries {
		if !entry.IsDir() || !timestampBackupName(entry.Name()) {
			continue
		}
		dir := filepath.Join(backupDir, entry.Name())
		infoPath := filepath.Join(dir, "backup.json")
		if _, statErr := os.Stat(infoPath); statErr == nil {
			continue
		} else if !os.IsNotExist(statErr) {
			if firstErr == nil {
				firstErr = statErr
			}
			continue
		}
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("remove %s: %w", dir, rmErr)
			}
			continue
		}
		removed++
	}
	return removed, firstErr
}

func timestampBackupName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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

		// A directory without backup.json is an unfinished copy. Listing stays quiet;
		// the next backup removes timestamp directories in that state.
		backupInfoPath := path.Join(backupDir, entry.Name(), "backup.json")
		file, readErr := os.ReadFile(backupInfoPath)
		if readErr != nil {
			if !os.IsNotExist(readErr) {
				logger.Error(ctx, fmt.Sprintf("failed to read backup info file: %s", readErr.Error()))
			}
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
		rmErr := deleteBackupDirectory(util.GetLocation().GetBackupDirectory(), backup)
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

// DeleteBackup removes one finished backup selected by its id.
func (m *Manager) DeleteBackup(ctx context.Context, backupID string) error {
	backupID = strings.TrimSpace(backupID)
	if backupID == "" {
		return fmt.Errorf("backup id is empty")
	}
	backups, err := m.FindAllBackups(ctx)
	if err != nil {
		return err
	}
	var target *Backup
	for index := range backups {
		if backups[index].Id == backupID {
			target = &backups[index]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}
	if err := deleteBackupDirectory(util.GetLocation().GetBackupDirectory(), *target); err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to remove backup: %s", err.Error()))
		return err
	}
	logger.Info(ctx, fmt.Sprintf("backup removed: %s, date: %s", target.Id, util.FormatTimestamp(target.Timestamp)))
	return nil
}

// deleteBackupDirectory removes one finished timestamp backup directly inside backupRoot.
// Other directories, including db_repair_* and anything outside backupRoot, stay in place.
func deleteBackupDirectory(backupRoot string, backup Backup) error {
	root := filepath.Clean(backupRoot)
	target := filepath.Clean(backup.Path)
	if target == "" || target == "." {
		target = filepath.Join(root, backup.Name)
	}
	name := filepath.Base(target)
	if !timestampBackupName(name) {
		return fmt.Errorf("backup directory is not a timestamp backup")
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel != name {
		return fmt.Errorf("backup path is outside the backup directory")
	}
	if _, statErr := os.Stat(filepath.Join(target, "backup.json")); statErr != nil {
		return statErr
	}
	return os.RemoveAll(target)
}
