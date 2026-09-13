//go:build !windows && !darwin

package automationdriver

import "os/exec"

func terminateProcess(command *exec.Cmd) error {
	return killProcessGroup(command)
}
