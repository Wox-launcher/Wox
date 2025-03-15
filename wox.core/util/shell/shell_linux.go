package shell

import (
	"os"
	"os/exec"
	"wox/util"
	"strings"
)

func Open(path string) error {
	if strings.HasSuffix(path, ".desktop") {
		_, err := os.Stat(path)
		if err != nil {
			return err
		}
		return exec.Command("gio", "launch", path).Start()
	}
	return exec.Command("xdg-open", path).Start()
}

func Run(name string, arg ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
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

	cmd := exec.Command(name, arg...)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	cmd.Env = append(os.Environ(), envs...)
	cmdErr := cmd.Start()
	if cmdErr != nil {
		return nil, cmdErr
	}

	return cmd, nil
}

func RunOutput(name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)
	return cmd.Output()
}

func OpenFileInFolder(path string) error {
	return exec.Command("xdg-open", path).Start()
}
