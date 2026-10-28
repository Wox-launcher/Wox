//go:build linux

package shell

import (
	"os/exec"
	"strings"
	"wox/util"
)

type hyprlandTerminalLauncher struct{}

func (hyprlandTerminalLauncher) name() string {
	return "hyprland"
}

func (hyprlandTerminalLauncher) open(interpreter string, command string, directory string) error {
	if util.IsOmarchy() {
		if path, err := exec.LookPath("xdg-terminal-exec"); err == nil {
			var args []string
			directory = strings.TrimSpace(directory)
			if directory != "" {
				args = append(args, "--dir="+directory)
			}
			if hold := linuxHoldArgs(interpreter, command); len(hold) > 0 {
				args = append(args, "--")
				args = append(args, hold...)
			}
			if err := startVisibleLaunch(util.NewTraceContext(), visibleProcessLaunch{File: path, Args: args, Directory: directory}); err == nil {
				return nil
			}
		}
	}
	candidates := []string{linuxEnvTerminal(), "kitty", "foot", "alacritty", "wezterm", "ghostty"}
	return openFirstLinuxTerminal(candidates, interpreter, command, directory)
}
