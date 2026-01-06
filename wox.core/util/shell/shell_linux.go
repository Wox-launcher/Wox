package shell

import (
	"os/exec"
	"wox/util"
)

func Open(path string) error {
	cmd := exec.Command("xdg-open", path)
	cmd.Dir = getWorkingDirectory(path)
	return cmd.Start()
}

func Run(name string, arg ...string) (*exec.Cmd, error) {
	cmd := BuildCommand(name, nil, arg...)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	cmd.Dir = getWorkingDirectory(name)
	cmdErr := cmd.Start()
	if cmdErr != nil {
		return nil, cmdErr
	}

	return cmd, nil
}

func RunWithEnv(name string, envs []string, arg ...string) (*exec.Cmd, error) {
	if len(envs) == 0 {
		return Run(name, arg...)
	}

	cmd := BuildCommand(name, envs, arg...)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	// Set working directory: use file's directory if name is a file path, otherwise use user home directory
	cmd.Dir = getWorkingDirectory(name)
	cmdErr := cmd.Start()
	if cmdErr != nil {
		return nil, cmdErr
	}

	return cmd, nil
}

func RunOutput(name string, arg ...string) ([]byte, error) {
	cmd := BuildCommand(name, nil, arg...)
	return cmd.Output()
}

func OpenFileInFolder(path string) error {
	return exec.Command("xdg-open", path).Start()
}

// HideWindowCmd is a no-op on Linux as there's no console window to hide
func HideWindowCmd(cmd *exec.Cmd) {
	// No-op on Linux
}
