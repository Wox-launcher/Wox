package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"wox/util"
)

// Scoop-style junctions must match the versioned process path without merging other installations.
func TestWindowsGetPidResolvesCurrentJunction(t *testing.T) {
	root := t.TempDir()
	version := filepath.Join(root, "1.0")
	if err := os.Mkdir(version, 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(version, "Code.exe")
	if err := os.WriteFile(exe, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(root, "current")
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", current, version).CombinedOutput(); err != nil {
		t.Fatalf("create junction: %v: %s", err, output)
	}
	r := &WindowsRetriever{runningProcesses: []processInfo{{Pid: 42, Path: exe}}, lastProcessUpdateTime: util.GetSystemTimestamp()}

	for _, path := range []string{exe, filepath.Join(current, "Code.exe")} {
		if got := r.GetPid(context.Background(), appInfo{Path: path, Type: AppTypeDesktop}); got != 42 {
			t.Fatalf("%s: pid=%d", path, got)
		}
	}
	other := filepath.Join(root, "other")
	if err := os.Mkdir(other, 0700); err != nil {
		t.Fatal(err)
	}
	otherExe := filepath.Join(other, "Code.exe")
	if err := os.WriteFile(otherExe, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := r.GetPid(context.Background(), appInfo{Path: otherExe, Type: AppTypeDesktop}); got != 0 {
		t.Fatalf("different installation matched pid=%d", got)
	}
}
