package app

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveInboxAppsFolderPath(t *testing.T) {
	systemRoot := getWindowsSystemRoot()
	got := resolveInboxAppsFolderPath(`{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\magnify.exe`)
	want := filepath.Join(systemRoot, "System32", "magnify.exe")
	if !strings.EqualFold(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if resolveInboxAppsFolderPath(`Microsoft.Windows.Explorer`) != "" {
		t.Fatal("expected empty path for shell AUMIDs")
	}
}

func TestListWindowsAppsFolderEntriesIncludesIndexableApps(t *testing.T) {
	entries, err := listWindowsAppsFolderEntries()
	if err != nil {
		t.Fatalf("listWindowsAppsFolderEntries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected AppsFolder to contain installed apps")
	}

	found := false
	for _, entry := range entries {
		if shouldIndexAppsFolderAppID(entry.AppID) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected at least one packaged Store app or web app in AppsFolder")
	}
}
