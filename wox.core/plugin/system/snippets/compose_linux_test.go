//go:build linux && cgo

package snippets

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestComposeInput exercises native dead-key and cancellation handling against XKB.
func TestComposeInput(t *testing.T) {
	compiler, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("C compiler unavailable")
	}
	binary := filepath.Join(t.TempDir(), "compose-test")
	command := exec.Command(compiler, "-I.", "testdata/compose_linux.c", "compose_linux.c", "-lxkbcommon", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile compose test: %v\n%s", err, output)
	}
	command = exec.Command(binary)
	command.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose handling: %v\n%s", err, output)
	}
}
