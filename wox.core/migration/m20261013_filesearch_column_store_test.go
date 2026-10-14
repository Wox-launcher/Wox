package migration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRetireFilenameCachePreservesContentAndReplacement(t *testing.T) {
	directory := t.TempDir()
	legacy := []string{"filesearch.db", "filesearch.db-wal", "filesearch.db-shm"}
	retained := []string{"metadata.db", "contentsearch.db", "contentsearch.db-wal", "unrelated"}
	for _, name := range append(legacy, retained...) {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := retireFilenameCache(context.Background(), directory); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range legacy {
		if _, err := os.Stat(filepath.Join(directory, name)); !os.IsNotExist(err) {
			t.Fatalf("legacy %s remains: %v", name, err)
		}
	}
	for _, name := range retained {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || string(data) != name {
			t.Fatalf("changed %s: %q %v", name, data, err)
		}
	}
}
