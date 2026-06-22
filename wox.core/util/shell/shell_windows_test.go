package shell

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateShellItemIDListHandlesSpecialPathCharacters(t *testing.T) {
	rootPath := t.TempDir()
	filePath := filepath.Join(rootPath, `$(Start-Process calc)& space dir`, `file $(Start-Process calc)& name.txt`)
	mustMkdirAll(t, filepath.Dir(filePath))
	mustWriteTestFile(t, filePath)

	itemIDList, err := createShellItemIDList(filePath)
	if err != nil {
		t.Fatalf("create Shell item ID list: %v", err)
	}
	if itemIDList == 0 {
		t.Fatal("expected non-zero Shell item ID list")
	}
	procILFree.Call(itemIDList)
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func mustWriteTestFile(t *testing.T, path string) {
	t.Helper()

	if err := os.WriteFile(path, []byte("wox shell test"), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
