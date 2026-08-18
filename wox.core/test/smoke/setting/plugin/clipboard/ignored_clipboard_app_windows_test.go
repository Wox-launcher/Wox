//go:build wox_ui_smoke && windows

package clipboard

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/test/smoke"
)

func requireClipboardIgnoredAppRuntime(t *testing.T) {
	t.Helper()
}

func ignoredClipboardAppTarget(t *testing.T) (string, string) {
	t.Helper()
	return "notepad.exe", "notepad.exe"
}

// copyTextFromIgnoredApplication launches an isolated Notepad document and copies its contents.
func copyTextFromIgnoredApplication(t *testing.T, ctx context.Context, text string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wox-private-clipboard.txt")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write temporary Notepad document: %v", err)
	}
	smoke.OpenWindowsNotepad(t, ctx, path)
	if err := smoke.SendNativeKeyChord("ctrl", "a"); err != nil {
		t.Fatalf("select Notepad document contents: %v", err)
	}
	if err := smoke.SendNativeKeyChord("ctrl", "c"); err != nil {
		t.Fatalf("copy Notepad document contents: %v", err)
	}
}
