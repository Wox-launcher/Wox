package util

import (
	"os/exec"
)

func ShellOpen(path string) {
	if IsMacOS() {
		exec.Command("open", path).Start()
	}
	if IsWindows() {
		exec.Command("cmd", "/C", "start", path).Start()
	}
}

func ShellRun(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = GetLogger().GetWriter()
	cmd.Stderr = GetLogger().GetWriter()
	cmdErr := cmd.Start()
	if cmdErr != nil {
		return cmdErr
	}

	return nil
}
