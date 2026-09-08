package recentfiles

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestParseRecentlyUsedXBELKeepsLastUsedOrderFields(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<xbel version="1.0"
 xmlns:bookmark="http://www.freedesktop.org/standards/desktop-bookmark"
 xmlns:mime="http://www.freedesktop.org/standards/shared-mime-info">
  <bookmark href="file:///tmp/newer.txt" added="2026-01-01T00:00:00Z" modified="2026-08-07T08:42:13.260987Z" visited="2026-02-01T00:00:00Z"/>
  <bookmark href="file:///tmp/My%20File.txt" added="2026-01-01T00:00:00Z"/>
  <bookmark href="https://example.com/skip" added="2026-01-01T00:00:00Z"/>
</xbel>`)

	bookmarks, err := parseRecentlyUsedXBEL(data)
	if err != nil {
		t.Fatalf("parse xbel: %v", err)
	}
	if len(bookmarks) != 2 {
		t.Fatalf("bookmark count = %d, want 2", len(bookmarks))
	}
	if bookmarks[0].path != filepath.Clean("/tmp/newer.txt") {
		t.Fatalf("first path = %q", bookmarks[0].path)
	}
	if bookmarks[1].path != filepath.Clean("/tmp/My File.txt") {
		t.Fatalf("decoded path = %q", bookmarks[1].path)
	}
	want := time.Date(2026, 8, 7, 8, 42, 13, 260987000, time.UTC)
	if !bookmarks[0].lastUsed.Equal(want) {
		t.Fatalf("last used = %s, want %s", bookmarks[0].lastUsed, want)
	}
}

func TestFileURIToPathRejectsNonFileAndRemoteHosts(t *testing.T) {
	if _, ok := fileURIToPath("https://example.com/a"); ok {
		t.Fatal("http href should be rejected")
	}
	if _, ok := fileURIToPath("file://otherhost/tmp/a"); ok {
		t.Fatal("remote file host should be rejected")
	}
	path, ok := fileURIToPath("file://localhost/tmp/a")
	if !ok || path != filepath.Clean("/tmp/a") {
		t.Fatalf("localhost file uri = %q ok=%t", path, ok)
	}
	if runtime.GOOS == "windows" {
		windowsPath, windowsOK := fileURIToPath("file:///C:/Users/me/notes.txt")
		if !windowsOK || windowsPath != filepath.Clean("C:/Users/me/notes.txt") {
			t.Fatalf("windows file uri = %q ok=%t", windowsPath, windowsOK)
		}
	}
}
