//go:build darwin || linux

package shell

import (
	"os/exec"
	"syscall"
)

func prepareShellCommand(interpreter string, command string) string {
	return command
}

func setCommandProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func decodeShellOutputChunk(chunk []byte) string {
	return string(chunk)
}
