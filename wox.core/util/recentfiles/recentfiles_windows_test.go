//go:build windows

package recentfiles

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListRecentReadsAutomaticDestinations(t *testing.T) {
	folder, err := windowsRecentFolder()
	if err != nil {
		t.Fatalf("recent folder: %v", err)
	}
	jumpListDir := filepath.Join(folder, "AutomaticDestinations")
	entries, err := os.ReadDir(jumpListDir)
	if err != nil {
		t.Skip("no AutomaticDestinations folder")
	}
	hasJumpList := false
	for _, entry := range entries {
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".automaticdestinations-ms") {
			hasJumpList = true
			break
		}
	}
	if !hasJumpList {
		t.Skip("no automatic destination files")
	}

	files, err := List(context.Background(), ListOption{Limit: 10})
	if err != nil {
		t.Fatalf("list recent files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("AutomaticDestinations was present but no existing recent files were returned")
	}
	t.Logf("recent file sample: %q", files[0].Path)
}
