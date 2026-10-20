package ffmpeg

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// hideConsole suppresses the console that Windows allocates for a GUI parent's console child.
func hideConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
