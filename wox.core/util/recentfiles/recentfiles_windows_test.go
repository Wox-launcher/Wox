//go:build windows

package recentfiles

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
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

	coldStart := time.Now()
	files, err := List(context.Background(), ListOption{Limit: 50})
	coldElapsed := time.Since(coldStart)
	if err != nil {
		t.Fatalf("list recent files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("AutomaticDestinations was present but no existing recent files were returned")
	}
	warmStart := time.Now()
	warmFiles, err := List(context.Background(), ListOption{Limit: 50})
	warmElapsed := time.Since(warmStart)
	if err != nil {
		t.Fatalf("warm list recent files: %v", err)
	}
	if len(warmFiles) != len(files) {
		t.Fatalf("warm result count %d != cold %d", len(warmFiles), len(files))
	}
	t.Logf("recent file sample=%q count=%d cold=%s warm=%s", files[0].Path, len(files), coldElapsed, warmElapsed)
}

func buildTestDestList(path string, used time.Time) []byte {
	units := utf16.Encode([]rune(path))
	buf := make([]byte, destListHeaderSize+destListEntryV2PathOffset+len(units)*2+destListEntryV2TrailingAlign)
	binary.LittleEndian.PutUint32(buf[0:4], 3)
	entry := buf[destListHeaderSize:]
	ticks := uint64(used.UnixNano()/100) + windowsToUnixFiletimeEpochTicks
	binary.LittleEndian.PutUint64(entry[destListEntryFiletimeOffset:destListEntryFiletimeOffset+8], ticks)
	binary.LittleEndian.PutUint16(entry[destListEntryV2PathSizeOffset:destListEntryV2PathSizeOffset+2], uint16(len(units)))
	for i, u := range units {
		binary.LittleEndian.PutUint16(entry[destListEntryV2PathOffset+i*2:destListEntryV2PathOffset+i*2+2], u)
	}
	return buf
}

func TestListRecentCacheAndShortcutCap(t *testing.T) {
	origResolve := resolveRecentShortcutFn
	origReadOLE := readOleStreamFn
	t.Cleanup(func() {
		resolveRecentShortcutFn = origResolve
		readOleStreamFn = origReadOLE
	})

	recentDir := t.TempDir()
	jumpDir := filepath.Join(recentDir, "AutomaticDestinations")
	targetDir := t.TempDir()
	if err := os.MkdirAll(jumpDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create 150 .lnk files to verify mtime sorting + 120-entry cap on resolution.
	targets := make(map[string]string, 150)
	baseTime := time.Now().Add(-300 * time.Minute)
	for i := 0; i < 150; i++ {
		targetPath := filepath.Join(targetDir, fmt.Sprintf("target-%03d.txt", i))
		if err := os.WriteFile(targetPath, []byte("ok"), 0644); err != nil {
			t.Fatal(err)
		}
		lnkPath := filepath.Join(recentDir, fmt.Sprintf("item-%03d.lnk", i))
		if err := os.WriteFile(lnkPath, []byte("lnk"), 0644); err != nil {
			t.Fatal(err)
		}
		mtime := baseTime.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(lnkPath, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		targets[strings.ToLower(lnkPath)] = targetPath
	}

	jumpTarget := filepath.Join(targetDir, "jump-target.txt")
	if err := os.WriteFile(jumpTarget, []byte("j1"), 0644); err != nil {
		t.Fatal(err)
	}
	jumpFile := filepath.Join(jumpDir, "app.automaticdestinations-ms")
	if err := os.WriteFile(jumpFile, []byte("ole1"), 0644); err != nil {
		t.Fatal(err)
	}

	resolveCalls := 0
	resolveRecentShortcutFn = func(lnkPath string) (string, bool) {
		resolveCalls++
		target, ok := targets[strings.ToLower(lnkPath)]
		return target, ok
	}
	oleCalls := 0
	currentJumpData := buildTestDestList(jumpTarget, baseTime.Add(200*time.Minute))
	readOleStreamFn = func(storagePath, streamName string) ([]byte, bool) {
		oleCalls++
		return currentJumpData, true
	}

	ctx := context.Background()
	files, err := listRecentFromFolder(ctx, recentDir, ListOption{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 10 {
		t.Fatalf("expected 10 files, got %d", len(files))
	}
	if files[0].Path != jumpTarget {
		t.Fatalf("expected newest jump list target %q first, got %q", jumpTarget, files[0].Path)
	}
	if !files[0].HasMeta || files[0].IsDir || files[0].Size != 2 {
		t.Fatalf("expected populated metadata for %q, got HasMeta=%v IsDir=%v Size=%d", files[0].Path, files[0].HasMeta, files[0].IsDir, files[0].Size)
	}
	if files[1].Path != filepath.Join(targetDir, "target-149.txt") {
		t.Fatalf("expected newest shortcut target-149.txt second, got %q", files[1].Path)
	}
	if resolveCalls != 120 {
		t.Fatalf("expected capped 120 shortcut resolutions, got %d", resolveCalls)
	}
	if oleCalls != 1 {
		t.Fatalf("expected 1 OLE read on cold pass, got %d", oleCalls)
	}

	// Second call with unchanged (modTime, size) must hit both caches.
	resolveCalls = 0
	oleCalls = 0
	files2, err := listRecentFromFolder(ctx, recentDir, ListOption{Limit: 10})
	if err != nil || len(files2) != 10 || files2[0].Path != jumpTarget {
		t.Fatalf("warm call mismatch: %+v %v", files2, err)
	}
	if resolveCalls != 0 || oleCalls != 0 {
		t.Fatalf("expected 0 COM/OLE calls on warm cache hit, got resolve=%d ole=%d", resolveCalls, oleCalls)
	}

	// Mutating one .lnk and the jump list must invalidate only those modified entries.
	updatedTarget := filepath.Join(targetDir, "updated.txt")
	if err := os.WriteFile(updatedTarget, []byte("u"), 0644); err != nil {
		t.Fatal(err)
	}
	hotLnk := filepath.Join(recentDir, "item-149.lnk")
	targets[strings.ToLower(hotLnk)] = updatedTarget
	if err := os.WriteFile(hotLnk, []byte("lnk-modified"), 0644); err != nil {
		t.Fatal(err)
	}
	newMtime := baseTime.Add(250 * time.Minute)
	if err := os.Chtimes(hotLnk, newMtime, newMtime); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jumpFile, []byte("ole2-updated"), 0644); err != nil {
		t.Fatal(err)
	}
	currentJumpData = buildTestDestList(jumpTarget, baseTime.Add(210*time.Minute))

	files3, err := listRecentFromFolder(ctx, recentDir, ListOption{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if resolveCalls != 1 || oleCalls != 1 {
		t.Fatalf("expected only modified entries to miss cache, got resolve=%d ole=%d", resolveCalls, oleCalls)
	}
	if files3[0].Path != updatedTarget {
		t.Fatalf("expected updated shortcut target %q first, got %q", updatedTarget, files3[0].Path)
	}
}
