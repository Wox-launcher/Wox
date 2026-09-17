//go:build linux

package shell

type hyprlandTerminalLauncher struct{}

func (hyprlandTerminalLauncher) name() string {
	return "hyprland"
}

func (hyprlandTerminalLauncher) open(interpreter string, command string, directory string) error {
	candidates := []string{linuxEnvTerminal(), "kitty", "foot", "alacritty", "wezterm", "ghostty"}
	return openFirstLinuxTerminal(candidates, interpreter, command, directory)
}
