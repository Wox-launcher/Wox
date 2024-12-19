package util

import (
	"testing"
)

func TestShellRunOutput(t *testing.T) {
	output, err := ShellRunOutput("zsh", "-c", "nvm current")
	if err != nil {
		t.Logf("ShellRunOutput() failed, err: %v", err)
		return
	}

	t.Logf("ShellRunOutput() output: %s", output)
}
