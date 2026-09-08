//go:build !windows

package tool

import "os/exec"

func configureBashCMD(cmd *exec.Cmd, command string) {}
