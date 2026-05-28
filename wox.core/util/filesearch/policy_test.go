package filesearch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatPathTypeTreatsSymlinkDirectoryAsFile(t *testing.T) {
	rootPath := t.TempDir()
	targetPath := filepath.Join(rootPath, "target")
	linkPath := filepath.Join(rootPath, "linked-target")

	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	mustCreateSymlink(t, targetPath, linkPath)

	isDir, known := statPathType(linkPath)
	if !known || isDir {
		t.Fatalf("expected symlink-to-directory to be known non-directory, got known=%v isDir=%v", known, isDir)
	}
}
