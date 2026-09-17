//go:build windows

package shell

import (
	"context"
	"os/exec"
	"syscall"
	"wox/util"
)

const windowsCreateNewConsole = 0x00000010

func openExternalTerminal(interpreter string, command string, directory string) error {
	return startVisibleLaunch(util.NewTraceContext(), buildWindowsExternalTerminalLaunch(interpreter, command, directory, windowsTerminalExecutable()))
}

func windowsTerminalExecutable() string {
	for _, name := range []string{"wt.exe", "wt"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func startVisibleProcess(ctx context.Context, launch visibleProcessLaunch) error {
	cmd := exec.Command(launch.File, launch.Args...)
	if launch.Directory != "" {
		cmd.Dir = launch.Directory
	}
	// Visible terminals must not inherit Wox's hidden-console launch flags or
	// the process-lifetime job, or the window would never appear or would die
	// with Wox.
	sys := &syscall.SysProcAttr{HideWindow: false}
	if launch.NewConsole {
		sys.CreationFlags = windowsCreateNewConsole
	}
	cmd.SysProcAttr = sys
	if err := cmd.Start(); err != nil {
		return err
	}
	reapVisibleProcess(ctx, cmd.Wait)
	return nil
}
