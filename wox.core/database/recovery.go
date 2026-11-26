package database

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"wox/util"

	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
)

type RecoveryMode string

const (
	RecoverySalvaged RecoveryMode = "salvaged"
	RecoveryBackup   RecoveryMode = "backup"
	RecoveryEmpty    RecoveryMode = "empty"
	recoveryStage                 = ".wox-db-recovery"
	recoveryRecord                = "recovery.json"
)

type RecoveryEvent struct {
	Mode          RecoveryMode
	Timestamp     int64
	BackupTime    int64
	SalvagedRows  int
	QuarantineDir string
}

type HealthyBackup struct {
	Path      string
	Timestamp int64
	Bytes     int64
}

var databaseSuffixes = []string{"-shm", "-wal", "-journal", ""}

// openWithRecovery only displaces the installed database after a verified candidate is ready.
func openWithRecovery(ctx context.Context, path, backupRoot string) (*gorm.DB, error) {
	stage := filepath.Join(filepath.Dir(path), recoveryStage)
	event, err := readRecoveryEvent(filepath.Join(stage, recoveryRecord))
	switch {
	case err == nil:
		if err := finishRecovery(ctx, path, backupRoot, stage, event); err != nil {
			return nil, err
		}
	case os.IsNotExist(err):
		if err := os.RemoveAll(stage); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	handle, err := openWoxDatabase(ctx, path)
	if err == nil || !isDatabaseCorruption(err) {
		return handle, err
	}
	probe, accessErr := os.OpenFile(path, os.O_RDWR, 0)
	if accessErr != nil {
		return nil, accessErr
	}
	if accessErr := probe.Close(); accessErr != nil {
		return nil, accessErr
	}
	util.GetLogger().Warn(ctx, "SQLite corruption detected, preserving the database before recovery")
	if err := prepareRecovery(ctx, path, backupRoot, stage); err != nil {
		return nil, fmt.Errorf("prepare database recovery: %w", err)
	}
	event, err = readRecoveryEvent(filepath.Join(stage, recoveryRecord))
	if err != nil {
		return nil, err
	}
	if err := finishRecovery(ctx, path, backupRoot, stage, event); err != nil {
		return nil, err
	}
	return openWoxDatabase(ctx, path)
}

func isDatabaseCorruption(err error) bool {
	var sqliteErr sqlite3.Error
	return errors.Is(err, errDatabaseIntegrity) ||
		(errors.As(err, &sqliteErr) && (sqliteErr.Code == sqlite3.ErrCorrupt || sqliteErr.Code == sqlite3.ErrNotADB))
}

// prepareRecovery keeps an untouched quarantine before opening a working copy for salvage.
func prepareRecovery(ctx context.Context, path, backupRoot, stage string) error {
	if err := os.MkdirAll(backupRoot, 0700); err != nil {
		return err
	}
	event := RecoveryEvent{Timestamp: util.GetSystemTimestamp()}
	quarantine, err := os.MkdirTemp(backupRoot, fmt.Sprintf("corrupt_%d_", event.Timestamp))
	if err != nil {
		return err
	}
	event.QuarantineDir = quarantine
	if err := copyDatabaseFiles(path, filepath.Join(quarantine, FileName)); err != nil {
		return err
	}
	if err := os.Mkdir(stage, 0700); err != nil {
		return err
	}
	candidate := filepath.Join(stage, "candidate.db")
	event.SalvagedRows, err = recoverCandidate(ctx, quarantine, stage, candidate)
	if err == nil {
		event.Mode = RecoverySalvaged
	} else {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		util.GetLogger().Warn(ctx, fmt.Sprintf("database salvage unavailable: %v", err))
		backup, err := LatestHealthyBackup(ctx, backupRoot)
		if err != nil {
			return err
		}
		event.Mode = RecoveryEmpty
		if backup.Path != "" {
			source, err := openDatabaseFile(backup.Path, "ro")
			if err != nil {
				return err
			}
			if err := errors.Join(SnapshotDatabase(ctx, source, candidate), source.Close()); err != nil {
				return err
			}
			event.Mode, event.BackupTime = RecoveryBackup, backup.Timestamp
		}
		if err := initializeRecoveryCandidate(ctx, candidate); err != nil {
			return err
		}
	}
	return writeRecoveryEvent(stage, event)
}

// finishRecovery can resume after any rename, including after the candidate has been installed.
func finishRecovery(ctx context.Context, path, backupRoot, stage string, event RecoveryEvent) error {
	parent, err := filepath.Abs(filepath.Dir(event.QuarantineDir))
	if err != nil {
		return err
	}
	root, err := filepath.Abs(backupRoot)
	if err != nil {
		return err
	}
	// Rel compares volumes and components case-insensitively on Windows.
	relative, err := filepath.Rel(root, parent)
	name := filepath.Base(event.QuarantineDir)
	if err != nil || relative != "." || !strings.HasPrefix(name, "corrupt_") {
		return fmt.Errorf("invalid database quarantine path")
	}
	event.QuarantineDir = filepath.Join(root, name)
	candidate := filepath.Join(stage, "candidate.db")
	if _, err := os.Stat(candidate); err == nil {
		if err := CheckDatabaseFile(ctx, candidate); err != nil {
			return err
		}
		for _, suffix := range databaseSuffixes {
			if err := os.Rename(path+suffix, filepath.Join(stage, "original"+suffix)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("isolate database%s: %w", suffix, err)
			}
		}
		if err := os.Rename(candidate, path); err != nil {
			return fmt.Errorf("publish recovered database: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := CheckDatabaseFile(ctx, path); err != nil {
		return err
	}
	if err := writeRecoveryEvent(event.QuarantineDir, event); err != nil {
		return err
	}
	util.GetLogger().Warn(ctx, fmt.Sprintf("database recovery %s, original files in %s", event.Mode, event.QuarantineDir))
	return os.RemoveAll(stage)
}

// LatestHealthyBackup checks completed backups newest-first and skips corrupt candidates.
func LatestHealthyBackup(ctx context.Context, root string) (HealthyBackup, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return HealthyBackup{}, nil
	}
	if err != nil {
		return HealthyBackup{}, err
	}
	var candidates []HealthyBackup
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		raw, err := os.ReadFile(filepath.Join(dir, "backup.json"))
		if err != nil {
			continue
		}
		var marker struct {
			ID        string
			Timestamp int64
		}
		if json.Unmarshal(raw, &marker) == nil && marker.ID != "" && marker.Timestamp > 0 {
			candidates = append(candidates, HealthyBackup{Path: filepath.Join(dir, FileName), Timestamp: marker.Timestamp})
		}
	}
	slices.SortFunc(candidates, func(a, b HealthyBackup) int { return cmp.Compare(b.Timestamp, a.Timestamp) })
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return HealthyBackup{}, err
		}
		if err := CheckDatabaseFile(ctx, candidate.Path); err != nil {
			continue
		}
		if info, err := os.Stat(candidate.Path); err == nil {
			candidate.Bytes = info.Size()
			return candidate, nil
		}
	}
	return HealthyBackup{}, nil
}

// LatestRecovery returns the most recent completed repair and whether its notice is pending.
func LatestRecovery(root string) (RecoveryEvent, bool) {
	entries, _ := os.ReadDir(root)
	var latest RecoveryEvent
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "corrupt_") {
			continue
		}
		event, err := readRecoveryEvent(filepath.Join(root, entry.Name(), recoveryRecord))
		if err == nil && event.Timestamp > latest.Timestamp {
			event.QuarantineDir = filepath.Join(root, entry.Name())
			latest = event
		}
	}
	if latest.Timestamp == 0 {
		return latest, false
	}
	_, err := os.Stat(filepath.Join(latest.QuarantineDir, "notice-shown"))
	return latest, os.IsNotExist(err)
}

func (e RecoveryEvent) Acknowledge() error {
	return os.WriteFile(filepath.Join(e.QuarantineDir, "notice-shown"), nil, 0600)
}

func readRecoveryEvent(path string) (RecoveryEvent, error) {
	var event RecoveryEvent
	raw, err := os.ReadFile(path)
	if err != nil {
		return event, err
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return event, err
	}
	switch event.Mode {
	case RecoveryBackup, RecoveryEmpty, RecoverySalvaged:
		return event, nil
	default:
		return RecoveryEvent{}, fmt.Errorf("invalid database recovery mode")
	}
}

func writeRecoveryEvent(dir string, event RecoveryEvent) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".recovery-")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	_, err = temp.Write(raw)
	if err == nil {
		err = temp.Sync()
	}
	if err := errors.Join(err, temp.Close()); err != nil {
		return err
	}
	return os.Rename(temp.Name(), filepath.Join(dir, recoveryRecord))
}

// copyDatabaseFiles preserves available journals as well as the database itself.
func copyDatabaseFiles(src, dst string) error {
	for _, suffix := range databaseSuffixes {
		if err := copyDatabaseFile(src+suffix, dst+suffix); err != nil && (suffix == "" || !os.IsNotExist(err)) {
			return err
		}
	}
	return nil
}

func copyDatabaseFile(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("SQLite recovery source is not a regular file")
	}
	from, err := os.Open(src)
	if err != nil {
		return err
	}
	defer from.Close()
	to, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(to, from)
	if err == nil {
		err = to.Sync()
	}
	return errors.Join(err, to.Close())
}
