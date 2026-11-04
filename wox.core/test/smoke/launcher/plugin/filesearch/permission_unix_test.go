//go:build wox_ui_smoke && !windows

package filesearch

import (
	"os"
	"testing"
)

// denyFileSearchRoot removes read access from the temporary fixture and restores it on cleanup.
func denyFileSearchRoot(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0o755); err != nil {
			t.Errorf("restore fixture access: %v", err)
		}
	})
	if _, err := os.ReadDir(path); err == nil {
		t.Skip("the current process can still read the denied directory")
	}
}
