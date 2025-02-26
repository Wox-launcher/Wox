package util

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// We only need to handle linux protocol handler
// Macos and windows protocol handler is registered in flutter side, see @main.dart
func EnsureDeepLinkProtocolHandler(ctx context.Context) {
	if !IsLinux() {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to get executable path: %s", err.Error()))
		return
	}

	desktopEntry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Wox
Exec=%s %%u
MimeType=x-scheme-handler/wox;
Terminal=false
NoDisplay=true
`, exePath)

	desktopFilePath := filepath.Join(os.Getenv("HOME"), ".local/share/applications/wox-url-handler.desktop")
	if err := os.MkdirAll(filepath.Dir(desktopFilePath), 0755); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to create directory: %s", err.Error()))
		return
	}

	if err := os.WriteFile(desktopFilePath, []byte(desktopEntry), 0644); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to write desktop file: %s", err.Error()))
		return
	}

	cmd := exec.Command("xdg-mime", "default", "wox-url-handler.desktop", "x-scheme-handler/wox")
	if err := cmd.Run(); err != nil {
		GetLogger().Error(ctx, fmt.Sprintf("failed to register protocol handler: %s", err.Error()))
	}

	cmd = exec.Command("update-desktop-database", filepath.Join(os.Getenv("HOME"), ".local/share/applications"))
	if err := cmd.Run(); err != nil {
		GetLogger().Warn(ctx, fmt.Sprintf("failed to update desktop database: %s", err.Error()))
	}

	GetLogger().Info(ctx, "Linux protocol handler registered successfully")
}
