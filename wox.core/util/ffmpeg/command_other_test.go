//go:build !windows

package ffmpeg

import (
	"os/exec"
	"testing"
)

func TestPrepareLeavesProcessAttributesUntouched(t *testing.T) {
	cmd := exec.Command("ffmpeg", "-version")
	Prepare(cmd)
	if cmd.SysProcAttr != nil {
		t.Fatalf("unexpected process attributes: %+v", cmd.SysProcAttr)
	}
}
