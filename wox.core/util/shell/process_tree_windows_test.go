package shell

import (
	"os/exec"
	"testing"
	"time"
	"wox/util/processmemory"
)

// TestTerminateProcessTreeKillsDescendants covers the leftover uv.exe case: killing only the
// process Wox started leaves the helpers that process spawned.
func TestTerminateProcessTreeKillsDescendants(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "ping -n 200 127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()

	deadline := time.Now().Add(5 * time.Second)
	var descendants []processmemory.DescendantProcess
	for {
		var err error
		descendants, err = processmemory.ListDescendantProcesses(cmd.Process.Pid)
		if err != nil {
			t.Fatal(err)
		}
		if len(descendants) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cmd.exe did not spawn a child before the tree was killed")
		}
		time.Sleep(50 * time.Millisecond)
	}

	TerminateProcessTree(cmd.Process.Pid)

	deadline = time.Now().Add(10 * time.Second)
	for {
		alive := livePids(descendants)
		if len(alive) == 0 && !pidAlive(cmd.Process.Pid) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("tree survived TerminateProcessTree: root=%d descendants=%v", cmd.Process.Pid, alive)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func pidAlive(pid int) bool {
	return len(livePids([]processmemory.DescendantProcess{{ProcessID: pid}})) > 0
}
