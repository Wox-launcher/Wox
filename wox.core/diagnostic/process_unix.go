//go:build darwin || linux

package diagnostic

import (
	"os/exec"
	"syscall"
)

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func ResolveProcessExit(waitErr error) (int, string) {
	if waitErr == nil {
		return 0, ""
	}
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				return -1, status.Signal().String()
			}
			return status.ExitStatus(), ""
		}
	}
	return -1, waitErr.Error()
}
