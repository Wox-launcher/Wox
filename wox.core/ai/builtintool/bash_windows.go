package tool

import (
	"os/exec"
	"syscall"
)

// configureBashCMD preserves shell quoting instead of Go's C-runtime argument escaping.
func configureBashCMD(cmd *exec.Cmd, command string) {
	cmd.Args = []string{cmd.Path, "/d", "/s", "/c", command}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CmdLine = `"` + cmd.Path + `" /d /s /c "` + command + `"`
}
