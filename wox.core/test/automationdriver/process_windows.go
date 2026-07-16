//go:build windows

package automationdriver

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func configureProcess(command *exec.Cmd) {}

func terminateProcess(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return nil
	}
	taskkill := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", command.Process.Pid))
	if output, err := taskkill.CombinedOutput(); err != nil {
		if killErr := command.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return fmt.Errorf("terminate Wox process tree: %v (%s); fallback: %w", err, output, killErr)
		}
	}
	return nil
}
