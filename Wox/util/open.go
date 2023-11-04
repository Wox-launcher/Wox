package util

func ShellRunOutput(name string, arg ...string) ([]byte, error) {
	cmd, err := ShellRun(name, arg...)
	if err != nil {
		return nil, err
	}
	return cmd.Output()
}
