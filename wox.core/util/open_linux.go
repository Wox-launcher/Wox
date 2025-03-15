package util

import (
	"os"
	"os/exec"
	"strings"
)

func ShellOpen(path string) error {
	if strings.HasSuffix(path, ".desktop") {
		_, err := os.Stat(path)
		if err != nil {
			return err
		}
		return exec.Command("gio", "launch", path).Start()
	}
	return exec.Command("xdg-open", path).Start()
}

func ShellRun(name string, arg ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = GetLogger().GetWriter()
	cmd.Stderr = GetLogger().GetWriter()
	cmdErr := cmd.Start()
	if cmdErr != nil {
		return nil, cmdErr
	}

	return cmd, nil
}

func ShellRunWithEnv(name string, envs []string, arg ...string) (*exec.Cmd, error) {
	if len(envs) == 0 {
		return ShellRun(name, arg...)
	}

	cmd := exec.Command(name, arg...)
	cmd.Stdout = GetLogger().GetWriter()
	cmd.Stderr = GetLogger().GetWriter()
	cmd.Env = append(os.Environ(), envs...)
	cmdErr := cmd.Start()
	if cmdErr != nil {
		return nil, cmdErr
	}

	return cmd, nil
}

func ShellRunOutput(name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)
	return cmd.Output()
}

func ShellOpenFileInFolder(path string) error {
	return exec.Command("xdg-open", path).Start()
}
