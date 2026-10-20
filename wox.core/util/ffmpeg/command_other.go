//go:build !windows

package ffmpeg

import "os/exec"

func hideConsole(cmd *exec.Cmd) {}
