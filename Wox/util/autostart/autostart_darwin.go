package autostart

import (
	"fmt"
	"os"
	"os/exec"
)

func setAutostart(enable bool) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	if enable {
		cmd := exec.Command("osascript", "-e", fmt.Sprintf(`tell application "System Events" to make login item at end with properties {path:"%s", hidden:false}`, exePath))
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to add login item: %w", err)
		}
	} else {
		cmd := exec.Command("osascript", "-e", fmt.Sprintf(`tell application "System Events" to delete login item "%s"`, exePath))
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to remove login item: %w", err)
		}
	}

	return nil
}

func isAutostart() (bool, error) {
	exePath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command("osascript", "-e", fmt.Sprintf(`tell application "System Events" to get the name of every login item whose path contains "%s"`, exePath))
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check login items: %w", err)
	}

	return len(output) > 0, nil
}
