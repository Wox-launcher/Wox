package tool

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
