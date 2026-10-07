//go:build darwin

package screenshot

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestDarwinScreenshotObjectSelection checks native animation, provider refinement, cancellation, and shared AX flag leases without requesting permission.
func TestDarwinScreenshotObjectSelection(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "screenshot-selector")
	command := exec.Command("clang", "-fblocks", "-fsanitize=address", "-Wno-deprecated-declarations",
		"-framework", "Cocoa", "-framework", "ApplicationServices", "-framework", "QuartzCore",
		"testdata/object_selection_darwin.m", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native selector test: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native selector hierarchy and animation: %v\n%s", err, output)
	}
}
