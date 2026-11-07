package snippets

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestWindowsClipboardTransaction uses a private window station, leaving the user's clipboard untouched.
func TestWindowsClipboardTransaction(t *testing.T) {
	compiler, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("GCC unavailable")
	}
	binary := filepath.Join(t.TempDir(), "clipboard-test.exe")
	command := exec.Command(compiler, "testdata/clipboard_windows.c", "clipboard_windows.c", "-o", binary, "-luser32", "-lshell32")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile clipboard test: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("clipboard transaction: %v\n%s", err, output)
	}
}
