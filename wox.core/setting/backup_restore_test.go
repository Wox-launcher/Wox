package setting

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestPluginBackupPath(t *testing.T) {
	root := t.TempDir()
	pluginFile := filepath.Join(root, "plugins", "demo", "main.py")
	if !pluginBackupPath(root, pluginFile) {
		t.Fatal("expected a file inside plugins to be skippable")
	}
	if !pluginBackupPath(root, filepath.Join(root, "plugins")) {
		t.Fatal("expected the plugins directory to be skippable")
	}
	if pluginBackupPath(root, filepath.Join(root, "wox.db")) {
		t.Fatal("expected the database to fail the backup")
	}
	if pluginBackupPath(root, filepath.Join(root, "plugins-extra", "a")) {
		t.Fatal("expected a sibling of plugins to fail the backup")
	}
	outside := filepath.Join(filepath.Dir(root), "other", "plugins", "main.py")
	if pluginBackupPath(root, outside) {
		t.Fatal("expected a path outside user data to fail the backup")
	}
	if runtime.GOOS == "windows" {
		messyRoot := filepath.ToSlash(root)
		messyFile := filepath.ToSlash(pluginFile)
		if !pluginBackupPath(messyRoot, messyFile) {
			t.Fatal("expected cleaned separators to still match plugins")
		}
	}
}

func TestCopyUserDataForBackupSkipsDirectoryLinks(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "backup")
	writeBackupFile(t, filepath.Join(src, "wox.db"), "database")
	writeBackupFile(t, filepath.Join(src, "settings", "settings.json"), "{}")
	writeBackupFile(t, filepath.Join(src, "plugins", "good", "main.txt"), "good")
	writeBackupFile(t, filepath.Join(src, "plugins", "flow", "real-plugin", "data.txt"), "plugin")
	writeBackupFile(t, filepath.Join(src, "plugins", "flow", ".rt", "Settings", "Settings.json"), "{}")

	link := filepath.Join(src, "plugins", "flow", ".rt", "Plugins", "real-plugin")
	target := filepath.Join(src, "plugins", "flow", "real-plugin")
	makeDirLink(t, link, target)
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if !backupEntrySkipped(info) {
		t.Fatalf("link mode %v should be skipped", info.Mode())
	}

	if err := copyUserDataForBackup(context.Background(), src, dest); err != nil {
		t.Fatal(err)
	}
	assertBackupFile(t, filepath.Join(dest, "wox.db"), "database")
	assertBackupFile(t, filepath.Join(dest, "settings", "settings.json"), "{}")
	assertBackupFile(t, filepath.Join(dest, "plugins", "good", "main.txt"), "good")
	assertBackupFile(t, filepath.Join(dest, "plugins", "flow", "real-plugin", "data.txt"), "plugin")
	assertBackupFile(t, filepath.Join(dest, "plugins", "flow", ".rt", "Settings", "Settings.json"), "{}")
	copiedLink := filepath.Join(dest, "plugins", "flow", ".rt", "Plugins", "real-plugin")
	if _, statErr := os.Lstat(copiedLink); !os.IsNotExist(statErr) {
		t.Fatalf("directory link copied into backup: %v", statErr)
	}
}

func TestRemoveIncompleteBackupsKeepsFinishedAndForeignDirectories(t *testing.T) {
	root := t.TempDir()
	writeBackupFile(t, filepath.Join(root, "1790956258983", "backup.json"), "{}")
	writeBackupFile(t, filepath.Join(root, "1790956258983", "wox.db"), "kept")
	writeBackupFile(t, filepath.Join(root, "1791981902473", "plugins", "good", "main.txt"), "partial")
	writeBackupFile(t, filepath.Join(root, "db_repair_1791", "wox.db"), "repair")
	writeBackupFile(t, filepath.Join(root, "temp_export", "wox.db"), "temp")
	writeBackupFile(t, filepath.Join(root, "notes"), "not-a-directory-name")
	if err := os.WriteFile(filepath.Join(root, "555"), []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}

	removed, err := removeIncompleteBackups(root)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, statErr := os.Stat(filepath.Join(root, "1790956258983", "backup.json")); statErr != nil {
		t.Fatal(statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "1791981902473")); !os.IsNotExist(statErr) {
		t.Fatal("incomplete backup directory should be removed")
	}
	if _, statErr := os.Stat(filepath.Join(root, "db_repair_1791", "wox.db")); statErr != nil {
		t.Fatal(statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "temp_export", "wox.db")); statErr != nil {
		t.Fatal(statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "555")); statErr != nil {
		t.Fatal(statErr)
	}
}

func TestDeleteBackupDirectoryRemovesOnlyThatTimestampBackup(t *testing.T) {
	root := t.TempDir()
	writeBackupFile(t, filepath.Join(root, "1790956258983", "backup.json"), "{}")
	writeBackupFile(t, filepath.Join(root, "1790956258983", "wox.db"), "gone")
	writeBackupFile(t, filepath.Join(root, "1790956259929", "backup.json"), "{}")
	writeBackupFile(t, filepath.Join(root, "db_repair_1791", "wox.db"), "repair")

	if err := deleteBackupDirectory(root, Backup{Name: "1790956258983", Path: filepath.Join(root, "1790956258983")}); err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "1790956258983")); !os.IsNotExist(statErr) {
		t.Fatal("selected backup should be removed")
	}
	if _, statErr := os.Stat(filepath.Join(root, "1790956259929", "backup.json")); statErr != nil {
		t.Fatal(statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "db_repair_1791", "wox.db")); statErr != nil {
		t.Fatal(statErr)
	}

	if err := deleteBackupDirectory(root, Backup{Name: "db_repair_1791", Path: filepath.Join(root, "db_repair_1791")}); err == nil {
		t.Fatal("repair directory should be rejected")
	}
	outside := t.TempDir()
	writeBackupFile(t, filepath.Join(outside, "1790956258983", "backup.json"), "keep")
	if err := deleteBackupDirectory(root, Backup{Path: filepath.Join(outside, "1790956258983")}); err == nil {
		t.Fatal("backup outside the backup root should be rejected")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "1790956258983", "backup.json")); statErr != nil {
		t.Fatal(statErr)
	}
}

func TestRemoveIncompleteBackupsMissingDirectory(t *testing.T) {
	removed, err := removeIncompleteBackups(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}
}

func writeBackupFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertBackupFile(t *testing.T, path string, content string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("%s = %q, want %q", path, got, content)
	}
}

func makeDirLink(t *testing.T, link, target string) {
	t.Helper()
	link, err := filepath.Abs(link)
	if err != nil {
		t.Fatal(err)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		command := exec.Command("cmd", "/c", "mklink", "/J", link, target)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("mklink /J: %v: %s", err, out)
		}
		return
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
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

func TestBackupRunIsSingleFlight(t *testing.T) {
	m := &Manager{}
	if !m.beginBackup() {
		t.Fatal("first begin should start the run")
	}
	if m.beginBackup() {
		t.Fatal("second begin should be rejected")
	}
	if !m.BackupRunning() {
		t.Fatal("backup should be running")
	}
	err := m.Backup(context.Background(), BackupTypeManual)
	if !errors.Is(err, ErrBackupInProgress) {
		t.Fatalf("Backup err = %v, want in progress", err)
	}
	if !m.BackupRunning() {
		t.Fatal("rejected backup cleared the in-flight run")
	}
	m.endBackup(nil)
	if m.BackupRunning() {
		t.Fatal("backup should be idle after end")
	}
	if !m.beginBackup() {
		t.Fatal("begin after end should start again")
	}
	m.endBackup(nil)
}

func TestWatchBackupReportsTransitions(t *testing.T) {
	m := &Manager{}
	type event struct {
		running bool
		failure string
	}
	var events []event
	record := func(running bool, failure error) {
		text := ""
		if failure != nil {
			text = failure.Error()
		}
		events = append(events, event{running: running, failure: text})
	}
	cancel := m.WatchBackup(record)
	if !m.beginBackup() {
		t.Fatal("begin")
	}
	if m.beginBackup() {
		t.Fatal("overlapping begin should be rejected without a second notification")
	}
	m.endBackup(errors.New("disk full"))
	cancel()
	if !m.beginBackup() {
		t.Fatal("begin after cancel")
	}
	m.endBackup(nil)

	want := []event{{}, {running: true}, {failure: "disk full"}}
	if len(events) != len(want) {
		t.Fatalf("events = %+v, want %+v", events, want)
	}
	for index := range want {
		if events[index] != want[index] {
			t.Fatalf("events = %+v, want %+v", events, want)
		}
	}
}
