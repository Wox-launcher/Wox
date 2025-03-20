package shell

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"wox/util"
)

func Open(path string) error {
	if strings.HasSuffix(path, ".app") {
		_, err := Run("open", "-a", path)
		return err
	}

	_, err := Run("open", path)
	return err
}

func Run(name string, arg ...string) (*exec.Cmd, error) {
	return RunWithEnv(name, []string{}, arg...)
}

func RunWithEnv(name string, envs []string, arg ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	if len(envs) == 0 {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = append(os.Environ(), envs...)
	}
	cmdErr := cmd.Start()
	if cmdErr != nil {
		return nil, cmdErr
	}

	return cmd, nil
}

func RunOutput(name string, arg ...string) ([]byte, error) {
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

func OpenFileInFolder(path string) error {
	return exec.Command("open", "-R", path).Start()
}
