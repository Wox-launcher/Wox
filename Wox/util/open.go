package util

import (
	"os/exec"
)

func ShellOpen(path string) error {
	if IsMacOS() {
		return exec.Command("open", path).Start()
	}
	if IsWindows() {
		return exec.Command("cmd", "/C", "start", path).Start()
	}
	if IsLinux() {
		return exec.Command("xdg-open", path).Start()
	}

	return nil
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
