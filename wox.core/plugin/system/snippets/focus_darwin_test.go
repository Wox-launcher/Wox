//go:build darwin && cgo

package snippets

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestMacInputSourceThreading exercises the main-queue requirement and a blocked main queue.
func TestMacInputSourceThreading(t *testing.T) {
	compiler, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("C compiler unavailable")
	}
	binary := filepath.Join(t.TempDir(), "snippet-focus-test")
	command := exec.Command(compiler, "-fblocks", "testdata/focus_darwin.m", "expansion_darwin.m",
		"-framework", "AppKit", "-framework", "ApplicationServices", "-framework", "Carbon", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile macOS focus test: %v\n%s", err, output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command = exec.CommandContext(ctx, binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("macOS focus test: %v\n%s", err, output)
	}
}
