//go:build linux && cgo

package snippets

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestX11ReplacementInIsolatedServer verifies native events without affecting the user's display.
func TestX11ReplacementInIsolatedServer(t *testing.T) {
	if os.Getenv("WOX_TEST_X11_ISOLATED") != "1" || os.Getenv("DISPLAY") == "" {
		t.Skip("requires a dedicated Xvfb display and WOX_TEST_X11_ISOLATED=1")
	}
	compiler, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("C compiler unavailable")
	}
	binary := filepath.Join(t.TempDir(), "x11-replacement-test")
	command := exec.Command(compiler, "testdata/replacement_linux_x11.c", "expansion_linux_x11.c", "compose_linux.c",
		"-lX11", "-lXi", "-lXtst", "-lxkbcommon", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile X11 replacement test: %v\n%s", err, output)
	}
	command = exec.Command(binary)
	command.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("X11 replacement events: %v\n%s", err, output)
	}
}
