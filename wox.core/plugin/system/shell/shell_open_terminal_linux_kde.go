//go:build linux

package shell

type kdeTerminalLauncher struct{}

func (kdeTerminalLauncher) name() string {
	return "kde"
}

func (kdeTerminalLauncher) open(interpreter string, command string, directory string) error {
	return openFirstLinuxTerminal([]string{"konsole"}, interpreter, command, directory)
}
