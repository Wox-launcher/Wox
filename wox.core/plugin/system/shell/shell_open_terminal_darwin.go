//go:build darwin

package shell

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"wox/util"
)

func openExternalTerminal(interpreter string, command string, directory string) error {
	app := darwinPreferredTerminalApp()
	script := buildDarwinTerminalScript(app, interpreter, command, directory)
	return startVisibleLaunch(util.NewTraceContext(), visibleProcessLaunch{
		File: "osascript",
		Args: []string{"-e", script},
	})
}

func darwinPreferredTerminalApp() string {
	homes := []string{"/Applications/iTerm.app"}
	if home, err := os.UserHomeDir(); err == nil {
		homes = append([]string{filepath.Join(home, "Applications", "iTerm.app")}, homes...)
	}
	for _, path := range homes {
		if _, err := os.Stat(path); err == nil {
			return "iTerm"
		}
	}
	return "Terminal"
}

func startVisibleProcess(ctx context.Context, launch visibleProcessLaunch) error {
	cmd := exec.Command(launch.File, launch.Args...)
	if launch.Directory != "" {
		cmd.Dir = launch.Directory
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	reapVisibleProcess(ctx, cmd.Wait)
	return nil
}
