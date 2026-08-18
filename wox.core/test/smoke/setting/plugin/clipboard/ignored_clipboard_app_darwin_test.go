//go:build wox_ui_smoke && darwin

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
	return "com.apple.TextEdit", "com.apple.TextEdit"
}

// copyTextFromIgnoredApplication launches an isolated TextEdit document and copies its contents.
func copyTextFromIgnoredApplication(t *testing.T, ctx context.Context, text string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wox-private-clipboard.txt")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write temporary TextEdit document: %v", err)
	}
	smoke.OpenDarwinTextEdit(t, ctx, path)
	if err := smoke.SendNativeKeyChord("command", "a"); err != nil {
		t.Fatalf("select TextEdit document contents: %v", err)
	}
	if err := smoke.SendNativeKeyChord("command", "c"); err != nil {
		t.Fatalf("copy TextEdit document contents: %v", err)
	}
}
