package tool

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"wox/network"
)

// TestBashCallback verifies real shell execution, quoting, and nonzero exits.
func TestBashCallback(t *testing.T) {
	t.Setenv("SHELL", "")
	path := filepath.Join(t.TempDir(), "file with spaces.txt")
	if err := os.WriteFile(path, []byte("quoted-path-ok"), 0600); err != nil {
		t.Fatal(err)
	}
	command := `cat "` + path + `" && echo second-command-ok`
	if runtime.GOOS == "windows" {
		command = `type "` + path + `" && echo second-command-ok`
	}
	for _, timeout := range []float64{0, 5} {
		result, err := bashCallback(context.Background(), map[string]any{"command": command, "timeout": timeout})
		if err != nil || !strings.Contains(result.Text, "quoted-path-ok") || !strings.Contains(result.Text, "second-command-ok") {
			t.Fatalf("timeout=%v: result=%+v err=%v", timeout, result, err)
		}
	}
	if _, err := bashCallback(context.Background(), map[string]any{"command": "exit 7"}); err == nil || !strings.Contains(err.Error(), "code 7") {
		t.Fatalf("expected exit code 7, got %v", err)
	}
	if !strings.Contains(BashTool().Description, getShell()) {
		t.Fatal("tool schema does not identify its shell")
	}
}

// TestBashCancellationCause distinguishes offline and caller cancellation from a timeout.
func TestBashCancellationCause(t *testing.T) {
	for _, cause := range []error{network.ErrOffline, context.Canceled} {
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(cause)
		_, err := bashCallback(ctx, map[string]any{"command": "echo unused", "timeout": float64(0)})
		if !errors.Is(err, cause) || strings.Contains(err.Error(), "timed out") {
			t.Fatalf("cause %v reported as %v", cause, err)
		}
	}
}
