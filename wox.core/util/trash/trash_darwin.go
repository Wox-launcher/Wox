//go:build darwin

package trash

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func MoveToTrash(path string) error {
	if path == "" {
		return fmt.Errorf("trash path is empty")
	}

	script := fmt.Sprintf("tell application \"Finder\" to delete POSIX file %s", strconv.Quote(path))
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("trash failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}
