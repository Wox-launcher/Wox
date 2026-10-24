package ffmpeg

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestPrepareHidesConsole(t *testing.T) {
	cmd := exec.Command("ffmpeg", "-version")
	Prepare(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf("console was not suppressed: %+v", cmd.SysProcAttr)
	}
}
