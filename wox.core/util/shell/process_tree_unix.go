//go:build !windows

package shell

import (
	"os"
)

func terminateProcessTree(pid int) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = process.Kill()
}
