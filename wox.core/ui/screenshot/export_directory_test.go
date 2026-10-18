package screenshot

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScreenshotExportUsesSelectedDirectory verifies the export reservation honors a caller's folder and never reuses a name.
func TestScreenshotExportUsesSelectedDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "OneDrive", "Screenshots")
	first, err := reserveScreenshotExportFilePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	second, err := reserveScreenshotExportFilePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(first) != directory || filepath.Dir(second) != directory || first == second {
		t.Fatalf("invalid reservations: %q, %q", first, second)
	}
	for _, path := range []string{first, second} {
		if info, err := os.Stat(path); err != nil || info.Size() != 0 {
			t.Fatalf("reservation missing: path=%s err=%v", path, err)
		}
	}
}
