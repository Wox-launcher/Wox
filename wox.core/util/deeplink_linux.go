//go:build linux

package util

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// EnsureDeepLinkProtocolHandler registers the desktop entry as the wox URL
// handler and the default application for .wox plugin packages.
func EnsureDeepLinkProtocolHandler(ctx context.Context) bool {
	desktopFilePath, err := LinuxDesktopEntryPath()
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to get Linux desktop entry path: %s", err.Error()))
		return false
	}

	if execPath, execErr := linuxDesktopExecPath(); execErr == nil && isEphemeralDebugExecutable(execPath) {
		GetLogger().Info(ctx, fmt.Sprintf("skipping Linux desktop entry update for debug executable: %s", execPath))
		return IsFileExists(desktopFilePath)
	}

	if err := WriteLinuxDesktopEntry(desktopFilePath, true, false); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to write Linux desktop entry: %s", err.Error()))
		return false
	}

	if err := writeLinuxPluginPackageMimeType(); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to write plugin package MIME type: %s", err.Error()))
	}

	cmd := exec.Command("xdg-mime", "default", LinuxDesktopFileName(), pluginPackageURLMIME)
	if err := cmd.Run(); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to register protocol handler: %s", err.Error()))
	}

	cmd = exec.Command("xdg-mime", "default", LinuxDesktopFileName(), PluginPackageMIMEType)
	if err := cmd.Run(); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to register .wox file association: %s", err.Error()))
	}

	cmd = exec.Command("update-desktop-database", filepath.Dir(desktopFilePath))
	if err := cmd.Run(); err != nil {
		GetLogger().Warn(ctx, fmt.Sprintf("failed to update desktop database: %s", err.Error()))
	}

	if mimeDir, mimeErr := linuxMimeDirectory(); mimeErr == nil {
		cmd = exec.Command("update-mime-database", mimeDir)
		if err := cmd.Run(); err != nil {
			GetLogger().Warn(ctx, fmt.Sprintf("failed to update MIME database: %s", err.Error()))
		}
	}

	GetLogger().Info(ctx, fmt.Sprintf("Linux desktop entry registered successfully: %s", desktopFilePath))
	return true
}
