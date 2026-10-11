//go:build windows

package automationdriver

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestTerminateProcessIgnoresAnExitedProcess(t *testing.T) {
	command := hiddenCommand("cmd.exe", "/d", "/c", "exit 0")
	if err := command.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = command.Wait() })

	deadline := time.Now().Add(5 * time.Second)
	for processStillRunning(command.Process.Pid) {
		if time.Now().After(deadline) {
			t.Fatal("process did not exit")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := terminateProcess(command); err != nil {
		t.Fatal(err)
	}
}

func TestTerminateProcessStopsARunningProcess(t *testing.T) {
	command := hiddenCommand("cmd.exe", "/d", "/c", "ping -n 30 127.0.0.1 >nul")
	if err := command.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = command.Wait() })
	if !processStillRunning(command.Process.Pid) {
		t.Fatal("process exited before terminate")
	}
	if err := terminateProcess(command); err != nil {
		t.Fatal(err)
	}
	if processStillRunning(command.Process.Pid) {
		t.Fatal("process still running")
	}
}

func hiddenCommand(name string, args ...string) *exec.Cmd {
	command := exec.Command(name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command
}
