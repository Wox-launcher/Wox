//go:build wox_ui_smoke && darwin

package clipboard

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxclipboard "wox/util/clipboard"
)

func requireClipboardIgnoredAppRuntime(t *testing.T) {
	t.Helper()
	if !smoke.CanPostDarwinKeyboardEvents() {
		t.Skip("macOS has not granted keyboard event posting to this test process")
	}
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
	pid := smoke.OpenDarwinTextEdit(t, ctx, path)
	// Foreground activation can precede loading the document. Retry the native
	// copy until its real clipboard result proves the editor is ready.
	ctx, cancel := context.WithTimeout(ctx, automationdriver.ActionTimeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if smoke.FrontmostDarwinApplicationPID() == pid {
			for _, key := range []string{"a", "c"} {
				if err := smoke.SendNativeKeyChord("command", key); err != nil {
					t.Fatalf("copy TextEdit document contents: %v", err)
				}
			}
		} else {
			smoke.ActivateDarwinApplication(pid)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for TextEdit process %d to copy its document (foreground=%d): %v", pid, smoke.FrontmostDarwinApplicationPID(), ctx.Err())
		case <-ticker.C:
		}
		if actual, err := woxclipboard.ReadText(); err == nil && actual == text {
			return
		}
	}
}
