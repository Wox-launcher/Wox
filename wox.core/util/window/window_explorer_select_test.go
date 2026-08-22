package window

import "testing"

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
