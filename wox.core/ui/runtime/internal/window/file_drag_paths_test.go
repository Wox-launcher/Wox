package window

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFileDragChecksExistenceAtUse rejects paths deleted after their query completed.
func TestFileDragChecksExistenceAtUse(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "main.go")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	paths := []string{file, "relative.go", filepath.Join(root, "missing")}
	if got := existingFileDragPaths(paths); len(got) != 1 || got[0] != file {
		t.Fatalf("drag paths: %v", got)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if got := existingFileDragPaths(paths); len(got) != 0 {
		t.Fatalf("deleted file remained draggable: %v", got)
	}
}
