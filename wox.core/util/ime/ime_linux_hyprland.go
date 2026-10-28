//go:build linux

package ime

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// switchInputMethodABCHyprland deactivates Fcitx's active input method so its
// configured default keyboard handles the focused query box.
func switchInputMethodABCHyprland() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "fcitx5-remote", "--check", "-c").CombinedOutput()
	if err != nil {
		return fmt.Errorf("deactivate Fcitx input method: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
