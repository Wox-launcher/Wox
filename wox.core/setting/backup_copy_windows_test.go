//go:build windows

package setting

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCopyUserDataForBackupSkipsUnreadablePluginFile(t *testing.T) {
	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "backup")
	writeBackupFile(t, filepath.Join(src, "wox.db"), "database")
	writeBackupFile(t, filepath.Join(src, "plugins", "good", "main.txt"), "good")
	locked := filepath.Join(src, "plugins", "bad", "locked.txt")
	writeBackupFile(t, locked, "locked")
	closeLocked := lockExclusive(t, locked)
	defer closeLocked()

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
	closeLocked := lockExclusive(t, locked)
	defer closeLocked()

	if err := copyUserDataForBackup(context.Background(), src, dest); err == nil {
		t.Fatal("expected a notes file to fail the backup")
	}
}

func lockExclusive(t *testing.T, path string) func() {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	return func() {
		_ = windows.CloseHandle(handle)
	}
}
