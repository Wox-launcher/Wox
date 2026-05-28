package indexpolicy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestTraversalContextFallbackHandlesTrailingSpaceDirectory(t *testing.T) {
	if os.Getenv("WOX_INDEXPOLICY_TRAILING_SPACE_HELPER") == "1" {
		runTrailingSpaceFallbackHelper(t)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestTraversalContextFallbackHandlesTrailingSpaceDirectory")
	cmd.Env = append(os.Environ(), "WOX_INDEXPOLICY_TRAILING_SPACE_HELPER=1")
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("expected fallback policy check to finish without recursive re-rooting loop")
	}
	if err != nil {
		t.Fatalf("fallback helper failed: %v\n%s", err, output)
	}
}

func runTrailingSpaceFallbackHelper(t *testing.T) {
	t.Helper()

	rootPath, err := os.MkdirTemp("", "wox-indexpolicy-")
	if err != nil {
		t.Fatalf("create test root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(testCreatePath(rootPath))
	})

	spacedDir := filepath.Join(rootPath, "Research ")
	filePath := filepath.Join(spacedDir, "note.md")
	if err := os.MkdirAll(testCreatePath(spacedDir), 0o755); err != nil {
		t.Fatalf("mkdir spaced directory: %v", err)
	}
	if err := os.WriteFile(testCreatePath(filePath), []byte("note"), 0o644); err != nil {
		t.Fatalf("write spaced file: %v", err)
	}

	policy := New()
	context := policy.NewTraversalContext(rootPath, rootPath, rootPath)
	if context == nil {
		t.Fatalf("expected traversal context")
	}
	if !context.ShouldIndexPath(filePath, false) {
		t.Fatalf("expected spaced file path to remain indexable")
	}
}

// testCreatePath keeps Windows from trimming trailing spaces in path components.
func testCreatePath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	return `\\?\` + path
}
