//go:build linux

package util

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// EnsureDeepLinkProtocolHandler registers the desktop entry as the wox URL handler.
func EnsureDeepLinkProtocolHandler(ctx context.Context) bool {
	desktopFilePath, err := LinuxDesktopEntryPath()
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to get Linux desktop entry path: %s", err.Error()))
		return false
	}

	if err := WriteLinuxDesktopEntry(desktopFilePath, true, false); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to write Linux desktop entry: %s", err.Error()))
		return false
	}

	cmd := exec.Command("xdg-mime", "default", LinuxDesktopFileName(), "x-scheme-handler/wox")
	if err := cmd.Run(); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to register protocol handler: %s", err.Error()))
	}

	cmd = exec.Command("update-desktop-database", filepath.Dir(desktopFilePath))
	if err := cmd.Run(); err != nil {
		GetLogger().Warn(ctx, fmt.Sprintf("failed to update desktop database: %s", err.Error()))
	}

	GetLogger().Info(ctx, fmt.Sprintf("Linux desktop entry registered successfully: %s", desktopFilePath))
	return true
}
