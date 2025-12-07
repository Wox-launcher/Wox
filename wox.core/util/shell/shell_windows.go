package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"wox/util"
)

func Open(path string) error {
	_, err := Run("cmd", "/C", "start", "", path)
	return err
}

func Run(name string, arg ...string) (*exec.Cmd, error) {
	return RunWithEnv(name, []string{"PYTHONIOENCODING=utf-8"}, arg...)
}

func RunWithEnv(name string, envs []string, arg ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} // Hide the window
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	cmd.Dir = getWorkingDirectory(name)
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
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} // Hide the window
	return cmd.Output()
}

func OpenFileInFolder(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	powershellCmd := fmt.Sprintf("Start-Process \"explorer.exe\" -ArgumentList \"/select,%s\"", absPath)
	_, err = Run("powershell.exe", "-Command", powershellCmd)
	return err
}
