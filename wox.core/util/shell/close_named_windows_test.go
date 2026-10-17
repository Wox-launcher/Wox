package shell

import (
	"os/exec"
	"testing"
	"time"
)

func TestCloseProcessIDsStopsTheStartedProcess(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "ping -n 30 127.0.0.1 > nul")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := closeProcessIDs(t.Context(), []int{cmd.Process.Pid}); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	if processAlive(cmd.Process.Pid) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("process still running")
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("process exit was not observed")
	}
}
