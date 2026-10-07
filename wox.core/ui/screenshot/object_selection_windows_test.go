//go:build windows

package screenshot

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestWindowsScreenshotObjectCache checks cache hits, cancellation progress, and bounded native cleanup without desktop interaction.
func TestWindowsScreenshotObjectCache(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "screenshot-selector.exe")
	command := exec.Command("g++", "-std=c++17", "-D_WIN32_WINNT=0x0A00",
		"testdata/object_selection_windows.cpp", "-lole32", "-loleaut32", "-luiautomationcore", "-luuid", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native selector test: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native selector cache: %v\n%s", err, output)
	}
}
