//go:build windows

package diagnostic

import (
	"os/exec"

	"golang.org/x/sys/windows"
)

const windowsProcessStillActive = 259

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Bug fix: Go's standard syscall package does not expose
	// PROCESS_QUERY_LIMITED_INFORMATION in this toolchain. Use x/sys/windows for
	// the same low-privilege process probe so Windows builds stay compatible
	// without requesting broader PROCESS_QUERY_INFORMATION access.
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	// Bug fix: OpenProcess can still succeed for a terminated process object on
	// Windows. The supervisor only needs to wait while the parent is genuinely
	// still executing, so use GetExitCodeProcess and treat STILL_ACTIVE as the
	// running state instead of equating an openable process handle with liveness.
	return exitCode == windowsProcessStillActive
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
