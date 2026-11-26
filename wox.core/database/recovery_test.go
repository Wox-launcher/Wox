package database

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestStartupRecoversCorruptDatabaseWithoutCLI(t *testing.T) {
	t.Setenv("PATH", "")
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, FileName)
	backups := filepath.Join(root, "backups")
	original := []byte("truncated database")
	require.NoError(t, os.WriteFile(path, original, 0600))
	require.NoError(t, os.WriteFile(path+"-journal", []byte("original journal"), 0600))
	good := filepath.Join(backups, "1")
	bad := filepath.Join(backups, "2")
	require.NoError(t, os.MkdirAll(good, 0700))
	require.NoError(t, os.MkdirAll(bad, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(good, "backup.json"), []byte(`{"Id":"good","Timestamp":1}`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(bad, "backup.json"), []byte(`{"Id":"bad","Timestamp":2}`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(bad, FileName), []byte("corrupt backup"), 0600))
	seedRecoveryDatabase(t, filepath.Join(good, FileName))

	handle, err := openWithRecovery(ctx, path, backups)
	require.NoError(t, err)
	conn, err := handle.DB()
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, CheckDatabase(ctx, conn))
	var saved WoxSetting
	require.NoError(t, handle.First(&saved, "key = ?", "theme-test").Error)
	require.Equal(t, "kept", saved.Value)
	event, pending := LatestRecovery(backups)
	require.True(t, pending)
	require.Equal(t, RecoveryBackup, event.Mode)
	require.EqualValues(t, 1, event.BackupTime)
	raw, err := os.ReadFile(filepath.Join(event.QuarantineDir, FileName))
	require.NoError(t, err)
	require.Equal(t, original, raw)
	require.FileExists(t, filepath.Join(event.QuarantineDir, FileName+"-journal"))
	require.NoFileExists(t, path+"-journal")
	require.NoError(t, event.Acknowledge())
	_, pending = LatestRecovery(backups)
	require.False(t, pending)
}

func TestStartupSalvagesReadableTablesAfterPageCorruption(t *testing.T) {
	t.Setenv("PATH", "")
	root := t.TempDir()
	path := filepath.Join(root, FileName)
	backups := filepath.Join(root, "backups")
	seedRecoveryDatabase(t, path)
	conn, err := openDatabaseFile(path, "rw")
	require.NoError(t, err)
	var page, size int64
	require.NoError(t, conn.QueryRow("SELECT rootpage FROM sqlite_schema WHERE name='mru_records'").Scan(&page))
	require.NoError(t, conn.QueryRow("PRAGMA page_size").Scan(&size))
	require.NoError(t, conn.Close())
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = file.WriteAt([]byte{0xff}, (page-1)*size)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	handle, err := openWithRecovery(context.Background(), path, backups)
	require.NoError(t, err)
	conn, err = handle.DB()
	require.NoError(t, err)
	defer conn.Close()
	var saved WoxSetting
	require.NoError(t, handle.First(&saved, "key = ?", "theme-test").Error)
	require.Equal(t, "kept", saved.Value)
	event, pending := LatestRecovery(backups)
	require.True(t, pending)
	require.Equal(t, RecoverySalvaged, event.Mode)
	require.Positive(t, event.SalvagedRows)
}

func TestStartupResumesPreparedEmptyFallback(t *testing.T) {
	t.Setenv("PATH", "")
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, FileName)
	backups := filepath.Join(root, "backups")
	stage := filepath.Join(root, recoveryStage)
	require.NoError(t, os.WriteFile(path, []byte("corrupt"), 0600))
	require.NoError(t, prepareRecovery(ctx, path, backups, stage))
	require.NoError(t, os.Rename(path, filepath.Join(stage, "original")))

	handle, err := openWithRecovery(ctx, path, backups)
	require.NoError(t, err)
	conn, err := handle.DB()
	require.NoError(t, err)
	defer conn.Close()
	event, pending := LatestRecovery(backups)
	require.True(t, pending)
	require.Equal(t, RecoveryEmpty, event.Mode)
	require.NoDirExists(t, stage)
	require.FileExists(t, filepath.Join(event.QuarantineDir, FileName))
}

func TestStartupResumesRecoveryWithDifferentWindowsPathCase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows paths compare case-insensitively")
	}
	t.Setenv("PATH", "")
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, FileName)
	backups := filepath.Join(root, "backups")
	stage := filepath.Join(root, recoveryStage)
	require.NoError(t, os.WriteFile(path, []byte("corrupt"), 0600))
	require.NoError(t, prepareRecovery(ctx, path, backups, stage))

	event, err := readRecoveryEvent(filepath.Join(stage, recoveryRecord))
	require.NoError(t, err)
	volume := filepath.VolumeName(backups)
	otherParent := strings.ToLower(volume) + filepath.Dir(event.QuarantineDir)[len(volume):]
	otherParent = filepath.Join(filepath.Dir(otherParent), "BACKUPS")
	event.QuarantineDir = filepath.Join(otherParent, filepath.Base(event.QuarantineDir))
	require.NoError(t, writeRecoveryEvent(stage, event))
	require.NoError(t, os.Rename(path, filepath.Join(stage, "original")))

	handle, err := openWithRecovery(ctx, path, backups)
	require.NoError(t, err)
	conn, err := handle.DB()
	require.NoError(t, err)
	defer conn.Close()
	recovered, pending := LatestRecovery(backups)
	require.True(t, pending)
	require.Equal(t, RecoveryEmpty, recovered.Mode)
	require.FileExists(t, filepath.Join(recovered.QuarantineDir, FileName))
}

func TestFinishRecoveryRejectsQuarantineOutsideBackupRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, FileName)
	backups := filepath.Join(root, "backups")
	event := RecoveryEvent{QuarantineDir: filepath.Join(root, "backups-extra", "corrupt_test")}
	require.NoError(t, os.WriteFile(path, []byte("original"), 0600))

	err := finishRecovery(context.Background(), path, backups, filepath.Join(root, recoveryStage), event)
	require.EqualError(t, err, "invalid database quarantine path")
	require.FileExists(t, path)
}

func TestRecoveryCopyWaitsForFileMaintenance(t *testing.T) {
	t.Setenv("PATH", "")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	source := t.TempDir()
	stage := t.TempDir()
	seedRecoveryDatabase(t, filepath.Join(source, FileName))
	result := make(chan error, 1)
	destination := filepath.Join(stage, "recovered.db")

	require.NoError(t, WithFileMaintenance(func() error {
		go func() {
			_, err := recoverCandidate(ctx, source, stage, destination)
			result <- err
		}()
		var workDir string
		require.Eventually(t, func() bool {
			dirs, err := filepath.Glob(filepath.Join(stage, ".salvage-*"))
			if err != nil || len(dirs) != 1 {
				return false
			}
			workDir = dirs[0]
			return true
		}, 3*time.Second, 5*time.Millisecond)
		_, err := os.Stat(filepath.Join(workDir, FileName))
		require.ErrorIs(t, err, os.ErrNotExist)
		return nil
	}))
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, CheckDatabaseFile(ctx, destination))
}

func TestEmbeddedSalvageFindsRowsPastDamagedLeaf(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, FileName)
	handle, err := openWoxDatabase(ctx, path)
	require.NoError(t, err)
	var settings []WoxSetting
	for i := 0; i < 80; i++ {
		settings = append(settings, WoxSetting{Key: fmt.Sprintf("key-%03d", i), Value: strings.Repeat("x", 700)})
	}
	require.NoError(t, handle.Create(&settings).Error)
	conn, err := handle.DB()
	require.NoError(t, err)
	var page, size int64
	require.NoError(t, conn.QueryRow("SELECT rootpage FROM sqlite_schema WHERE name='wox_settings'").Scan(&page))
	require.NoError(t, conn.QueryRow("PRAGMA page_size").Scan(&size))
	require.NoError(t, conn.Close())
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	pageBytes := make([]byte, size)
	_, err = file.ReadAt(pageBytes, (page-1)*size)
	require.NoError(t, err)
	require.EqualValues(t, 0x05, pageBytes[0])
	cellCount := int(binary.BigEndian.Uint16(pageBytes[3:5]))
	require.Greater(t, cellCount, 2)
	index := 12 + 2*(cellCount/2)
	cell := binary.BigEndian.Uint16(pageBytes[index : index+2])
	leaf := int64(binary.BigEndian.Uint32(pageBytes[cell : cell+4]))
	_, err = file.WriteAt([]byte{0xff}, (leaf-1)*size)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	recovered := filepath.Join(root, "recovered.db")
	count, err := salvageDatabase(ctx, path, recovered)
	require.NoError(t, err)
	require.Positive(t, count)
	require.Less(t, count, len(settings))
	got, err := openDatabaseFile(recovered, "ro")
	require.NoError(t, err)
	defer got.Close()
	for _, key := range []string{"key-000", "key-079"} {
		var raw string
		require.NoError(t, got.QueryRow("SELECT value FROM wox_settings WHERE key = ?", key).Scan(&raw))
		require.Equal(t, strings.Repeat("x", 700), raw)
	}
}

func TestDatabaseRecoveryDoesNotTreatAccessFailuresAsCorruption(t *testing.T) {
	for _, code := range []sqlite3.ErrNo{sqlite3.ErrBusy, sqlite3.ErrLocked, sqlite3.ErrPerm, sqlite3.ErrIoErr, sqlite3.ErrFull, sqlite3.ErrReadonly} {
		require.False(t, isDatabaseCorruption(sqlite3.Error{Code: code}))
	}
}

func seedRecoveryDatabase(t *testing.T, path string) {
	t.Helper()
	handle, err := openWoxDatabase(context.Background(), path)
	require.NoError(t, err)
	require.NoError(t, handle.Create(&WoxSetting{Key: "theme-test", Value: "kept"}).Error)
	var conn *sql.DB
	conn, err = handle.DB()
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}
