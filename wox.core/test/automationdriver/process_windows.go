//go:build windows

package automationdriver

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/windows"
)

const windowsProcessStillActive = 259

func configureProcess(command *exec.Cmd) {}

// terminateProcess force-closes the Wox process tree.
// taskkill /T exits non-zero when any child has already exited, even after the
// rest of the tree is gone. TerminateProcess then returns access denied for that
// exited process. Either outcome is success once the root process is gone.
func terminateProcess(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return nil
	}
	pid := command.Process.Pid
	taskkill := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", pid))
	output, err := taskkill.CombinedOutput()
	if err == nil || !processStillRunning(pid) {
		return nil
	}
	killErr := command.Process.Kill()
	if killErr == nil || errors.Is(killErr, os.ErrProcessDone) || !processStillRunning(pid) {
		return nil
	}
	return fmt.Errorf("terminate Wox process tree: %v (%s); fallback: %w", err, output, killErr)
}

// processStillRunning reports whether pid is still executing.
// OpenProcess succeeds for a terminated process object, so the exit code is what
// distinguishes a live process from one taskkill has already removed.
func processStillRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == windowsProcessStillActive
}
