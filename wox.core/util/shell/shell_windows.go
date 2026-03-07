package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"wox/util"
)

func Open(path string) error {
	cmd := BuildCommand("cmd", []string{"PYTHONIOENCODING=utf-8"})
	// Set CmdLine directly to bypass Go's automatic argument escaping.
	// This ensures our quoting is preserved so cmd.exe won't treat & as a command separator.
	cmd.SysProcAttr.CmdLine = `cmd /C start "" ` + QuoteCmdArg(path)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	cmd.Dir = getWorkingDirectory("cmd")
	return cmd.Start()
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

// QuoteCmdArg wraps a value in double quotes for use as a cmd.exe argument,
// escaping any embedded double quotes. This prevents cmd.exe from treating
// characters like & as command separators in URLs.
func QuoteCmdArg(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
