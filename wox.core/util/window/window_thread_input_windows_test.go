//go:build windows && cgo

package window

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestThreadInputAttachmentBalancesNativeQueues verifies native attachment cleanup without altering live desktop focus.
func TestThreadInputAttachmentBalancesNativeQueues(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "window-thread-input.exe")
	command := exec.Command("g++", "-std=c++17", "-O2", "-Wall", "-Wextra", "-Werror", "testdata/window_thread_input_windows.cpp", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native thread-input fixture: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native thread-input cleanup: %v\n%s", err, output)
	}
}
