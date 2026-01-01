//go:build linux

package trash

import (
	"fmt"
	"os/exec"
	"strings"
)

func MoveToTrash(path string) error {
	if path == "" {
		return fmt.Errorf("trash path is empty")
	}

	if gioPath, err := exec.LookPath("gio"); err == nil {
		if err := runTrashCommand(gioPath, "trash", path); err != nil {
			return err
		}
		return nil
	}

	if trashPath, err := exec.LookPath("trash-put"); err == nil {
		if err := runTrashCommand(trashPath, path); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("no trash command available (gio or trash-put)")
}

func runTrashCommand(cmd string, args ...string) error {
	output, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("trash failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}
