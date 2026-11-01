//go:build windows

package quickjump

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

func TestFindDirectoryOpusListerRejectsNonListerWindows(t *testing.T) {
	for _, windowID := range []string{"", "abc", "0", "-1"} {
		if _, ok := findDirectoryOpusLister(os.Getpid(), windowID); ok {
			t.Fatalf("windowID %q was accepted as an Opus lister", windowID)
		}
	}

	// The taskbar is a real window of another process with a non-Opus class.
	taskbar := win.FindWindow(windows.StringToUTF16Ptr("Shell_TrayWnd"), nil)
	if taskbar == 0 {
		t.Skip("taskbar window not found")
	}
	var taskbarPid uint32
	if _, err := windows.GetWindowThreadProcessId(windows.HWND(taskbar), &taskbarPid); err != nil {
		t.Fatalf("GetWindowThreadProcessId: %v", err)
	}
	taskbarID := strconv.FormatUint(uint64(taskbar), 10)
	if _, ok := findDirectoryOpusLister(int(taskbarPid), taskbarID); ok {
		t.Fatal("Explorer's taskbar was accepted as an Opus lister")
	}
}

func TestFindDirectoryOpusListerRequiresOwnerPid(t *testing.T) {
	hwnd, pid := findAnyDirectoryOpusListerWindow(t)
	windowID := strconv.FormatUint(uint64(hwnd), 10)

	lister, ok := findDirectoryOpusLister(pid, windowID)
	if !ok {
		t.Fatal("running Opus lister was not recognized")
	}
	if _, err := os.Stat(lister.dopusrtPath); err != nil {
		t.Fatalf("dopusrt path %q: %v", lister.dopusrtPath, err)
	}
	if _, ok := findDirectoryOpusLister(pid+1, windowID); ok {
		t.Fatal("lister was accepted with a pid that does not own it")
	}
}

// TestDirectoryOpusGoToNavigatesActiveLister drives the user's real active lister through the
// production goTo/sourcePath path and restores its original folder afterwards.
func TestDirectoryOpusGoToNavigatesActiveLister(t *testing.T) {
	if os.Getenv("WOX_DIRECTORY_OPUS_INTEGRATION") != "1" {
		t.Skip("set WOX_DIRECTORY_OPUS_INTEGRATION=1 to navigate the active Directory Opus lister")
	}
	lister := findActiveDirectoryOpusLister(t)
	ctx := context.Background()

	original := lister.sourcePath(ctx)
	if original == "" {
		original = os.TempDir()
	}

	root := t.TempDir()
	// Registered after TempDir, so it runs before the temp folders are deleted. It must wait
	// until Opus has left them: if Opus sees its current folder deleted it falls back to the
	// Desktop asynchronously, which would override the next test run's navigation.
	t.Cleanup(func() {
		if err := lister.goTo(ctx, original); err != nil {
			t.Errorf("restore original Opus folder %q: %v", original, err)
			return
		}
		waitForDirectoryOpusSourcePath(t, lister, original)
	})
	folder := filepath.Join(root, "wox opus 中文")
	otherFolder := filepath.Join(root, "reveal target")
	file := filepath.Join(otherFolder, "target file.txt")
	for _, dir := range []string{folder, otherFolder} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(file, []byte("wox"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := lister.goTo(ctx, folder); err != nil {
		t.Fatalf("goTo folder: %v", err)
	}
	waitForDirectoryOpusSourcePath(t, lister, folder)

	// A drive root ends in a backslash right before the closing quote.
	driveRoot := filepath.VolumeName(root) + `\`
	if err := lister.goTo(ctx, driveRoot); err != nil {
		t.Fatalf("goTo drive root: %v", err)
	}
	waitForDirectoryOpusSourcePath(t, lister, driveRoot)

	// A file path must open its parent folder (and select the file).
	if err := lister.goTo(ctx, file); err != nil {
		t.Fatalf("goTo file: %v", err)
	}
	waitForDirectoryOpusSourcePath(t, lister, otherFolder)
}

func waitForDirectoryOpusSourcePath(t *testing.T, lister directoryOpusLister, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var trace []string
	for time.Now().Before(deadline) {
		got := lister.sourcePath(context.Background())
		if strings.EqualFold(filepath.Clean(got), filepath.Clean(want)) {
			return
		}
		if len(trace) == 0 || trace[len(trace)-1] != got {
			trace = append(trace, got)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("Opus source path never became %q; observed %q", want, trace)
}

func findAnyDirectoryOpusListerWindow(t *testing.T) (win.HWND, int) {
	t.Helper()
	hwnd := win.FindWindow(windows.StringToUTF16Ptr(directoryOpusListerClass), nil)
	if hwnd == 0 {
		t.Skip("no Directory Opus lister is open")
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(windows.HWND(hwnd), &pid); err != nil {
		t.Fatalf("GetWindowThreadProcessId: %v", err)
	}
	return hwnd, int(pid)
}

// findActiveDirectoryOpusLister returns the lister Opus reports as active, which is the one
// `dopusrt /acmd` commands run in.
func findActiveDirectoryOpusLister(t *testing.T) directoryOpusLister {
	t.Helper()
	anyHwnd, pid := findAnyDirectoryOpusListerWindow(t)
	anyLister, ok := findDirectoryOpusLister(pid, strconv.FormatUint(uint64(anyHwnd), 10))
	if !ok {
		t.Fatal("running Opus lister was not recognized")
	}

	data, err := queryDirectoryOpusPaths(context.Background(), anyLister.dopusrtPath)
	if err != nil {
		t.Fatalf("query Opus paths: %v", err)
	}
	var result struct {
		Paths []struct {
			Lister       string `xml:"lister,attr"`
			ActiveLister string `xml:"active_lister,attr"`
		} `xml:"path"`
	}
	if err := xml.Unmarshal(data, &result); err != nil {
		t.Fatalf("parse Opus paths: %v", err)
	}
	for _, path := range result.Paths {
		if path.ActiveLister != "1" {
			continue
		}
		hwnd, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(path.Lister), "0x"), 16, 64)
		if err != nil {
			t.Fatalf("parse lister %q: %v", path.Lister, err)
		}
		lister, ok := findDirectoryOpusLister(pid, fmt.Sprintf("%d", hwnd))
		if !ok {
			t.Fatalf("active lister %s was not recognized", path.Lister)
		}
		return lister
	}
	t.Skip("Opus reported no active lister")
	return directoryOpusLister{}
}
