//go:build darwin && cgo

package window

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestFileDialogDetectionSkipsDataRows exercises native AX classification without opening windows.
func TestFileDialogDetectionSkipsDataRows(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "file-dialog-detect")
	command := exec.Command("clang", "testdata/file_dialog_detect_darwin.m", "-framework", "Cocoa", "-framework", "ApplicationServices", "-framework", "ScriptingBridge", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native file dialog fixture: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native file dialog detection: %v\n%s", err, output)
	}
}
