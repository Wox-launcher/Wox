//go:build wox_ui_smoke && windows

package hotkey

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/test/smoke"
)

func requireIgnoredHotkeyAppRuntime(t *testing.T) {
	t.Helper()
}

func ignoredHotkeyAppTarget(t *testing.T) (string, string) {
	t.Helper()
	return "notepad.exe", "notepad.exe"
}

func ignoredHotkeyAppHotkey(t *testing.T) string {
	t.Helper()
	return "ctrl+f12"
}

func sendIgnoredAppNativeHotkey(t *testing.T, hotkey string) {
	t.Helper()
	sendNativeHotkey(t, hotkey)
}

// activateIgnoredHotkeyApp starts one isolated Notepad process and waits until its window owns foreground focus.
func activateIgnoredHotkeyApp(t *testing.T, ctx context.Context) {
	t.Helper()
	tempFile, err := os.CreateTemp("", "wox-smoke-notepad-*.txt")
	if err != nil {
		t.Fatalf("create temporary Notepad document: %v", err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		t.Fatalf("close temporary Notepad document: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tempPath) })
	smoke.OpenWindowsNotepad(t, ctx, filepath.Clean(tempPath))
}
