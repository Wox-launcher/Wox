package ai

import (
	"testing"
)

// TestMCPServerProcessesAreDroppedOnReset guards the attribution the memory diagnostics rely on.
// A retained pid would be charged to the wrong server once Windows reuses it, so resetting the
// sessions has to forget the processes those sessions owned.
func TestMCPServerProcessesAreDroppedOnReset(t *testing.T) {
	mcpServerProcesses.Store("duckduckgo", 4321)
	mcpServerProcesses.Store("filesystem", 1234)

	processes := ListMCPServerProcesses()
	if len(processes) != 2 || processes[0].Name != "duckduckgo" || processes[1].Name != "filesystem" {
		t.Fatalf("listed processes = %#v, want both servers ordered by name", processes)
	}

	ResetMCPClients()
	if remaining := ListMCPServerProcesses(); len(remaining) != 0 {
		t.Fatalf("reset left %#v behind", remaining)
	}
}
