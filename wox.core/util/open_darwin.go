package util

import (
	"fmt"
	"os/exec"
)

func ShellOpen(path string) error {
	return exec.Command("open", path).Start()
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

func ShellRunOutput(name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if output != nil {
			return nil, fmt.Errorf("%s: %s", err, output)
		}

		return nil, err
	} else {
		return output, nil
	}
}

func ShellOpenFileInFolder(path string) error {
	return exec.Command("open", "-R", path).Start()
}
