//go:build wox_ui_smoke && windows

package filesearch

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"testing"
)

// denyFileSearchRoot applies and later removes an access-deny ACL on the temporary fixture only.
func denyFileSearchRoot(t *testing.T, path string) {
	t.Helper()
	account, err := user.Current()
	if err != nil || account.Uid == "" {
		t.Skipf("cannot resolve current Windows user SID: %v", err)
	}
	sid := "*" + account.Uid
	output, err := exec.Command("icacls.exe", path, "/deny", fmt.Sprintf("%s:(OI)(CI)(RX)", sid)).CombinedOutput()
	if err != nil {
		t.Fatalf("deny fixture access: %s: %v", output, err)
	}
	t.Cleanup(func() {
		if output, err := exec.Command("icacls.exe", path, "/remove:d", sid).CombinedOutput(); err != nil {
			t.Errorf("restore fixture access: %s: %v", output, err)
		}
	})
	if _, err := os.ReadDir(path); err == nil {
		t.Skip("the current process can still read the denied directory")
	}
}
