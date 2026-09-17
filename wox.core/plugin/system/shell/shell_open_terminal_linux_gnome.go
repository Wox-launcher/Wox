//go:build linux

package shell

type gnomeTerminalLauncher struct{}

func (gnomeTerminalLauncher) name() string {
	return "gnome"
}

func (gnomeTerminalLauncher) open(interpreter string, command string, directory string) error {
	return openFirstLinuxTerminal([]string{"gnome-terminal", "kgx"}, interpreter, command, directory)
}
