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
	cmd := BuildCommand(name, envs, arg...)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
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

// HideWindowCmd sets the SysProcAttr to hide the console window on Windows
func HideWindowCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
