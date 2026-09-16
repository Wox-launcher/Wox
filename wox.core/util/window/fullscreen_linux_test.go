package window

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestFullscreenDesktopQueries exercises real command execution with deterministic
// desktop replies, including absent windows, maximization and malformed responses.
func TestFullscreenDesktopQueries(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	for name, script := range map[string]string{
		"hyprctl": "#!/bin/sh\nprintf '%s' \"$WOX_TEST_ACTIVE\"\nexit \"${WOX_TEST_EXIT:-0}\"\n",
		"xprop":   "#!/bin/sh\nif [ \"$1\" = '-root' ]; then printf '%s' \"$WOX_TEST_ACTIVE\"; else printf '%s' \"$WOX_TEST_STATE\"; fi\nexit \"${WOX_TEST_EXIT:-0}\"\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		active string
		want   bool
	}{
		{`{"fullscreen":2}`, true}, {`{"fullscreen":3}`, true},
		{`{"fullscreen":1}`, false}, {`{"fullscreen":0,"fullscreenClient":2}`, false},
		{`{}`, false}, {`invalid`, false},
	} {
		t.Setenv("WOX_TEST_ACTIVE", tc.active)
		if got := isHyprlandActiveWindowFullscreen(context.Background()); got != tc.want {
			t.Errorf("Hyprland %s: got %v", tc.active, got)
		}
	}
	for _, tc := range []struct {
		active, state string
		want          bool
	}{
		{"_NET_ACTIVE_WINDOW(WINDOW): window id # 0x123", "_NET_WM_STATE(ATOM) = _NET_WM_STATE_FULLSCREEN", true},
		{"_NET_ACTIVE_WINDOW(WINDOW): window id # 0x123", "_NET_WM_STATE(ATOM) = _NET_WM_STATE_ABOVE, _NET_WM_STATE_FULLSCREEN", true},
		{"_NET_ACTIVE_WINDOW(WINDOW): window id # 0x123", "_NET_WM_STATE(ATOM) = _NET_WM_STATE_MAXIMIZED_VERT, _NET_WM_STATE_MAXIMIZED_HORZ", false},
		{"_NET_ACTIVE_WINDOW(WINDOW): window id # 0x0", "_NET_WM_STATE(ATOM) = _NET_WM_STATE_FULLSCREEN", false},
		{"missing", "", false},
		{"_NET_ACTIVE_WINDOW(WINDOW): window id # 0x123", "_NET_WM_STATE: not found.", false},
	} {
		t.Setenv("WOX_TEST_ACTIVE", tc.active)
		t.Setenv("WOX_TEST_STATE", tc.state)
		if got := isX11ActiveWindowFullscreen(context.Background()); got != tc.want {
			t.Errorf("X11 %s / %s: got %v", tc.active, tc.state, got)
		}
	}
	t.Setenv("WOX_TEST_EXIT", "1")
	t.Setenv("WOX_TEST_ACTIVE", `{"fullscreen":2}`)
	if isHyprlandActiveWindowFullscreen(context.Background()) {
		t.Fatal("failed command suppressed hotkey")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if isHyprlandActiveWindowFullscreen(ctx) || isX11ActiveWindowFullscreen(ctx) {
		t.Fatal("cancelled query suppressed hotkey")
	}
}
