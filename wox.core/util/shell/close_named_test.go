package shell

import "testing"

func TestMatchesProcessName(t *testing.T) {
	wanted := map[string]struct{}{"raycast.exe": {}, "raycast.uiaccess.exe": {}}
	if !matchesProcessName("Raycast.exe", wanted) || !matchesProcessName("  Raycast.UIAccess.exe ", wanted) {
		t.Fatal("expected both Raycast executables")
	}
	if matchesProcessName("Raycast", wanted) || matchesProcessName("", wanted) {
		t.Fatal("unrelated names matched")
	}
}

func TestCloseNamedProcessesIgnoresAnUnknownName(t *testing.T) {
	if err := CloseNamedProcesses(t.Context(), "wox-migration-no-such-process.exe"); err != nil {
		t.Fatal(err)
	}
}
