//go:build wox_ui_smoke

package hotkey

import (
	"context"
	"os"
	"strings"
	"testing"

	"wox/test/smoke"
)

// currentHotkeyLogSize captures the boundary before a native hotkey is injected.
func currentHotkeyLogSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat Wox log %q: %v", path, err)
	}
	return info.Size()
}

// waitForHotkeyLog waits for runtime evidence written after the captured log boundary.
func waitForHotkeyLog(t *testing.T, ctx context.Context, path string, offset int64, expected string) string {
	t.Helper()
	data := smoke.WaitForFile(t, ctx, path, func(data []byte) bool {
		return int64(len(data)) >= offset && strings.Contains(string(data[offset:]), expected)
	})
	return string(data[offset:])
}
