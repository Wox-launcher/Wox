//go:build linux

package screenshot

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLinuxHyprlandScreenshotCursorPosition exercises the IPC path without an X11 display.
func TestLinuxHyprlandScreenshotCursorPosition(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
		bounds   Rect
		want     *Point
	}{
		{name: "native pointer", response: `{"x":473,"y":553}`, want: &Point{X: 473, Y: 553}},
		{name: "valid origin", response: `{"x":0,"y":0}`, want: &Point{}},
		{name: "scaled secondary monitor", response: `{"x":2080,"y":180}`, bounds: Rect{X: 1920, Y: 80, Width: 1280, Height: 720}, want: &Point{X: 160, Y: 100}},
		{name: "negative monitor origin", response: `{"x":-1180,"y":-200}`, bounds: Rect{X: -1280, Y: -300, Width: 1280, Height: 720}, want: &Point{X: 100, Y: 100}},
		{name: "outside captured monitor", response: `{"x":500,"y":300}`, bounds: Rect{X: 1920, Width: 1280, Height: 720}, want: &Point{X: -1420, Y: 300}},
		{name: "missing coordinate", response: `{"x":473}`},
		{name: "invalid response", response: `not json`},
	} {
		t.Run(test.name, func(t *testing.T) {
			serveScreenshotHyprlandCursor(t, test.response)
			got := linuxScreenshotCursorPosition(test.bounds)
			if test.want == nil {
				if got != nil {
					t.Fatalf("unavailable pointer = %+v, want nil", got)
				}
				return
			}
			if got == nil || *got != *test.want {
				t.Fatalf("capture pointer = %+v, want %+v", got, test.want)
			}
		})
	}
}

// TestLinuxHyprlandScreenshotCursorUnavailable rejects failed queries instead of fabricating an origin.
func TestLinuxHyprlandScreenshotCursorUnavailable(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "missing")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("DISPLAY", "")
	if got := linuxScreenshotCursorPosition(Rect{}); got != nil {
		t.Fatalf("unavailable pointer = %+v, want nil", got)
	}
}

// serveScreenshotHyprlandCursor keeps socket paths short and verifies the compositor request.
func serveScreenshotHyprlandCursor(t *testing.T, response string) {
	t.Helper()
	runtimeDir, err := os.MkdirTemp("", "wox-cursor-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(runtimeDir) })
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test")
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("DISPLAY", "")
	socketPath := filepath.Join(runtimeDir, "hypr", "test", ".socket.sock")
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		if err := connection.SetDeadline(time.Now().Add(time.Second)); err != nil {
			t.Error(err)
			return
		}
		request := make([]byte, len("j/cursorpos"))
		if _, err := io.ReadFull(connection, request); err != nil {
			t.Error(err)
			return
		}
		if string(request) != "j/cursorpos" {
			t.Errorf("unexpected cursor request: %q", request)
			return
		}
		if _, err := io.WriteString(connection, response); err != nil {
			t.Error(err)
		}
	}()
}
