//go:build windows

package diagnostic

import (
	"os/exec"
	"syscall"
)

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = syscall.CloseHandle(handle)
	return true
}

func ResolveProcessExit(waitErr error) (int, string) {
	if waitErr == nil {
		return 0, ""
	}
	if exitErr, ok := waitErr.(*exec.ExitError); ok && exitErr.ProcessState != nil {
		return exitErr.ProcessState.ExitCode(), ""
	}
	return -1, waitErr.Error()
}
