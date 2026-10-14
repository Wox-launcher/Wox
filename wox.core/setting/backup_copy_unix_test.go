//go:build !windows

package setting

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyUserDataForBackupSkipsUnreadablePluginFile(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "backup")
	writeBackupFile(t, filepath.Join(src, "wox.db"), "database")
	writeBackupFile(t, filepath.Join(src, "plugins", "good", "main.txt"), "good")
	locked := filepath.Join(src, "plugins", "bad", "locked.txt")
	writeBackupFile(t, locked, "locked")
	restore := makeUnreadable(t, locked)
	defer restore()

	if err := copyUserDataForBackup(context.Background(), src, dest); err != nil {
		t.Fatal(err)
	}
	assertBackupFile(t, filepath.Join(dest, "wox.db"), "database")
	assertBackupFile(t, filepath.Join(dest, "plugins", "good", "main.txt"), "good")
	if _, statErr := os.Lstat(filepath.Join(dest, "plugins", "bad", "locked.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("unreadable plugin file copied: %v", statErr)
	}
}

func TestCopyUserDataForBackupFailsForUnreadableUserData(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "backup")
	writeBackupFile(t, filepath.Join(src, "wox.db"), "database")
	locked := filepath.Join(src, "notes", "locked.txt")
	writeBackupFile(t, locked, "locked")
	restore := makeUnreadable(t, locked)
	defer restore()

	if err := copyUserDataForBackup(context.Background(), src, dest); err == nil {
		t.Fatal("expected a notes file to fail the backup")
	}
}

func makeUnreadable(t *testing.T, path string) func() {
	t.Helper()
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	if file, err := os.Open(path); err == nil {
		file.Close()
		t.Skip("this user can still read a mode 0 file")
	}
	return func() {
		_ = os.Chmod(path, 0o644)
	}
}
