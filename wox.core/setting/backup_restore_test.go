package setting

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
