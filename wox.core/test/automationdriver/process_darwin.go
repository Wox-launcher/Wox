//go:build darwin

package automationdriver

/*
#cgo LDFLAGS: -framework Cocoa
int woxAutomationTerminateApplication(int pid);
*/
import "C"

import (
	"os/exec"
	"syscall"
	"time"
)

// terminateProcess asks AppKit to quit so Dock can drop the Regular-policy tile
// before the process group is force-killed. SIGKILL alone leaves ghost icons
// for the unsigned wox-go-ui-smoke binary.
func terminateProcess(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return nil
	}
	pid := command.Process.Pid
	if C.woxAutomationTerminateApplication(C.int(pid)) != 0 {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if err := command.Process.Signal(syscall.Signal(0)); err != nil {
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	// The application can exit before its children; always clean up the isolated group.
	return killProcessGroup(command)
}
