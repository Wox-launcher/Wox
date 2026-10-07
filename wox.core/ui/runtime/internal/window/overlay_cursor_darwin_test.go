//go:build darwin

package window

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestDarwinOverlaySystemCursor compares actual WindowServer cursor pixels in a background process.
// Opt in because this briefly changes the desktop cursor; ordinary unit tests must remain headless.
func TestDarwinOverlaySystemCursor(t *testing.T) {
	if os.Getenv("WOX_TEST_NATIVE_CURSOR") != "1" {
		t.Skip("set WOX_TEST_NATIVE_CURSOR=1 to verify the live macOS system cursor")
	}
	binary := filepath.Join(t.TempDir(), "screenshot-cursor")
	command := exec.Command("clang", "-fblocks", "-Wno-deprecated-declarations", "-framework", "Cocoa",
		"testdata/overlay_cursor_darwin.m", "overlay_cursor_darwin.m", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native cursor test: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native cursor/panel handoff/policy restoration check: %v\n%s", err, output)
	}
}
