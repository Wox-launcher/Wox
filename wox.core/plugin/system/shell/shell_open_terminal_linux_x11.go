//go:build linux

package shell

type x11TerminalLauncher struct{}

func (x11TerminalLauncher) name() string {
	return "x11"
}

func (x11TerminalLauncher) open(interpreter string, command string, directory string) error {
	candidates := []string{"x-terminal-emulator", linuxEnvTerminal(), "xterm", "kitty", "alacritty", "wezterm", "gnome-terminal", "konsole"}
	return openFirstLinuxTerminal(candidates, interpreter, command, directory)
}
