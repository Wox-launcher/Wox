package setting

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRestoreUserDataDirectoryReplacesTheLiveDirectory(t *testing.T) {
	root := t.TempDir()
	userData := filepath.Join(root, "wox-user")
	backup := filepath.Join(root, "backup")
	if err := os.Mkdir(userData, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userData, "current.txt"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(backup, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "current.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "backup.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RestoreUserDataDirectory(context.Background(), backup, userData); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(userData, "current.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("restored content = %q, want new", got)
	}
	if _, statErr := os.Stat(filepath.Join(userData, "backup.json")); !os.IsNotExist(statErr) {
		t.Fatal("restored backup.json should be removed")
	}
	matches, err := filepath.Glob(userData + ".before_restore_*")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("previous directories = %v, want one", matches)
	}
	previous, err := os.ReadFile(filepath.Join(matches[0], "current.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(previous) != "old" {
		t.Fatalf("previous content = %q, want old", previous)
	}
}

func TestRestoreUserDataDirectoryRejectsTheSamePath(t *testing.T) {
	dir := t.TempDir()
	if err := RestoreUserDataDirectory(context.Background(), dir, dir); err == nil {
		t.Fatal("restoring a directory onto itself should fail")
	}
}

func TestHasAutoBackupOnLocalDay(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 10, 10, 21, 40, 0, 0, location)
	today := now.UnixMilli()
	yesterday := now.AddDate(0, 0, -1).UnixMilli()

	if hasAutoBackupOnLocalDay([]Backup{
		{Type: BackupTypeManual, Timestamp: today},
		{Type: BackupTypeAuto, Timestamp: yesterday},
	}, now) {
		t.Fatal("expected no automatic backup for the local day")
	}
	if !hasAutoBackupOnLocalDay([]Backup{
		{Type: BackupTypeManual, Timestamp: today},
		{Type: BackupTypeAuto, Timestamp: today},
	}, now) {
		t.Fatal("expected today's automatic backup to count")
	}
}

func TestHasAutoBackupOnLocalDaySplitsAtLocalMidnight(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 10, 10, 0, 30, 0, 0, location)
	beforeMidnight := time.Date(2026, 10, 9, 23, 50, 0, 0, location).UnixMilli()
	afterMidnight := time.Date(2026, 10, 10, 0, 1, 0, 0, location).UnixMilli()

	if hasAutoBackupOnLocalDay([]Backup{{Type: BackupTypeAuto, Timestamp: beforeMidnight}}, now) {
		t.Fatal("expected the previous local day to be due")
	}
	if !hasAutoBackupOnLocalDay([]Backup{{Type: BackupTypeAuto, Timestamp: afterMidnight}}, now) {
		t.Fatal("expected the backup after local midnight to count")
	}
}

func TestDurationUntilNextLocalMidnight(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	evening := time.Date(2026, 10, 10, 23, 50, 0, 0, location)
	if got := durationUntilNextLocalMidnight(evening); got != 10*time.Minute {
		t.Fatalf("duration = %s, want 10m", got)
	}
	midnight := time.Date(2026, 10, 10, 0, 0, 0, 0, location)
	if got := durationUntilNextLocalMidnight(midnight); got != 24*time.Hour {
		t.Fatalf("duration = %s, want 24h", got)
	}
}
