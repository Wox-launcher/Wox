//go:build linux && cgo

package snippets

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestWaylandInputAdapter checks native XKB translation and uinput event encoding without device access.
func TestWaylandInputAdapter(t *testing.T) {
	compiler, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("C compiler unavailable")
	}
	binary := filepath.Join(t.TempDir(), "wayland-input-test")
	command := exec.Command(compiler, "-I.", "testdata/wayland_linux.c", "compose_linux.c", "-lwayland-client", "-lxkbcommon", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile Wayland input test: %v\n%s", err, output)
	}
	command = exec.Command(binary)
	command.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Wayland input events: %v\n%s", err, output)
	}
}
