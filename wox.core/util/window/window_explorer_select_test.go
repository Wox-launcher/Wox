package window

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSelectBestExplorerShellWindowUsesExactHwnd(t *testing.T) {
	candidates := []explorerShellWindowCandidate{
		{index: 0, hwnd: 0x100, path: `C:\alpha`, locationName: "alpha", z: 2},
		{index: 1, hwnd: 0x200, path: `C:\beta`, locationName: "beta", z: 1},
	}

	got := selectBestExplorerShellWindowCandidate(candidates, 0x100, "beta")
	if got != 0 {
		t.Fatalf("exact HWND lost to title match: got=%d", got)
	}
}

func TestSelectBestExplorerShellWindowUsesLatestTitleForSharedHwnd(t *testing.T) {
	candidates := []explorerShellWindowCandidate{
		{index: 0, hwnd: 0x100, path: `C:\work\docs`, locationName: "docs", z: 0},
		{index: 1, hwnd: 0x100, path: `D:\archive\docs`, locationName: "docs", z: 0},
		{index: 2, hwnd: 0x100, path: `C:\work\src`, locationName: "src", z: 0},
	}

	got := selectBestExplorerShellWindowCandidate(candidates, 0x100, "src")
	if got != 2 {
		t.Fatalf("shared HWND should pick the current tab title, got=%d", got)
	}
}

func TestSelectBestExplorerShellWindowDoesNotPickSameNameDifferentPath(t *testing.T) {
	candidates := []explorerShellWindowCandidate{
		{index: 0, hwnd: 0x100, path: `C:\Users\a\Documents`, locationName: "Documents", z: 1},
		{index: 1, hwnd: 0x200, path: `D:\Work\Documents`, locationName: "Documents", z: 0},
	}

	got := selectBestExplorerShellWindowCandidate(candidates, 0x200, "Documents")
	if got != 1 {
		t.Fatalf("same folder name should still honor the exact HWND, got=%d", got)
	}
}

func TestExplorerCabinetWindowClassRejectsBrowseForFolderDialog(t *testing.T) {
	if !isExplorerCabinetWindowClass("CabinetWClass") || !isExplorerCabinetWindowClass("ExploreWClass") {
		t.Fatal("Explorer cabinet classes should be accepted")
	}
	if isExplorerCabinetWindowClass("#32770") || isExplorerCabinetWindowClass("SHBrowseForFolder ShellNameSpace Control") {
		t.Fatal("browse-for-folder windows must not be treated as Explorer cabinet windows")
	}
}

func TestShouldQueryExplorerShellWindowPathSkipsOtherCabinets(t *testing.T) {
	if shouldQueryExplorerShellWindowPath(0, 0x100) {
		t.Fatal("zero hwnd must not be queried")
	}
	if !shouldQueryExplorerShellWindowPath(0x100, 0) {
		t.Fatal("no preferred hwnd should query every cabinet")
	}
	if !shouldQueryExplorerShellWindowPath(0x200, 0x200) {
		t.Fatal("the captured hwnd must be queried")
	}
	if shouldQueryExplorerShellWindowPath(0x100, 0x200) {
		t.Fatal("other explorer.exe cabinets must not be queried while a preferred hwnd is set")
	}
}

func TestFilesystemPathFromShellLocationURL(t *testing.T) {
	dir := t.TempDir()
	slash := filepath.ToSlash(dir)
	raw := "file://" + slash
	if runtime.GOOS == "windows" && len(slash) >= 2 && slash[1] == ':' {
		raw = "file:///" + slash
	}
	if got := filesystemPathFromShellLocationURL(raw); got != dir {
		t.Fatalf("file URL %q = %q, want %q", raw, got, dir)
	}

	spaced := filepath.Join(dir, "My Documents")
	if err := os.Mkdir(spaced, 0o755); err != nil {
		t.Fatal(err)
	}
	spacedSlash := filepath.ToSlash(spaced)
	encoded := strings.ReplaceAll(spacedSlash, " ", "%20")
	spacedURL := "file://" + encoded
	if runtime.GOOS == "windows" && len(spacedSlash) >= 2 && spacedSlash[1] == ':' {
		spacedURL = "file:///" + encoded
	}
	if got := filesystemPathFromShellLocationURL(spacedURL); got != spaced {
		t.Fatalf("encoded file URL = %q, want %q", got, spaced)
	}

	if got := filesystemPathFromShellLocationURL(""); got != "" {
		t.Fatalf("empty URL = %q", got)
	}
	if got := filesystemPathFromShellLocationURL("shell:::{20D04FE0-3AEA-1069-A2D8-08002B30309D}"); got != "" {
		t.Fatalf("shell URL = %q", got)
	}
	if got := filesystemPathFromShellLocationURL("http://example.invalid/folder"); got != "" {
		t.Fatalf("http URL = %q", got)
	}
}

func TestExistingFilesystemDirectoryRejectsVirtualAndMissing(t *testing.T) {
	if got := existingFilesystemDirectory(""); got != "" {
		t.Fatalf("empty path = %q", got)
	}
	if got := existingFilesystemDirectory("::{20D04FE0-3AEA-1069-A2D8-08002B30309D}"); got != "" {
		t.Fatalf("virtual shell path = %q", got)
	}
	if got := existingFilesystemDirectory(`C:\this-path-should-not-exist-wox-quick-switch`); got != "" {
		t.Fatalf("missing path = %q", got)
	}

	dir := t.TempDir()
	if got := existingFilesystemDirectory(dir); got != dir {
		t.Fatalf("existing dir = %q, want %q", got, dir)
	}
}
