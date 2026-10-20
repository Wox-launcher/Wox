package ffmpeg

import "os/exec"

// Prepare keeps a console-subsystem tool from opening a window when Wox is a Windows GUI process.
func Prepare(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	hideConsole(cmd)
}
