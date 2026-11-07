//go:build linux && cgo

package snippets

import (
	"errors"
	"os"
	"testing"
)

// TestX11ListenerLifecycle exercises the native subscription against the active X server.
func TestX11ListenerLifecycle(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("an X server is required")
	}
	backend := newX11ExpansionBackend()
	stop, err := backend.start(func(inputEvent) {})
	if errors.Is(err, errFocusUnavailable) {
		t.Skip("an accessibility bus is required")
	}
	if err != nil {
		t.Fatal(err)
	}
	_ = backend.target()
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if got := backend.target(); got != 0 {
		t.Fatalf("closed backend target = %d", got)
	}
}
