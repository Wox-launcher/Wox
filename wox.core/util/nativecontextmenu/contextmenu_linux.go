package nativecontextmenu

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// ShowContextMenu displays the system context menu for a file or folder on Linux
// This implementation uses xdg-open as a fallback since Linux doesn't have
// a standardized context menu API across different desktop environments
func ShowContextMenu(path string) error {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Try different desktop environment specific methods
	// First, try to detect the desktop environment
	desktopEnv := detectDesktopEnvironment()

	switch desktopEnv {
	case "KDE":
		// For KDE/Plasma, we can use dolphin with --select
		return showContextMenuKDE(absPath)
	case "GNOME":
		// For GNOME, we can use nautilus with --select
		return showContextMenuGNOME(absPath)
	default:
		// Fallback: just open the containing folder
		// This is not ideal but works across all desktop environments
		return showContextMenuFallback(absPath)
	}
}

func detectDesktopEnvironment() string {
	// Try to detect desktop environment from environment variables
	cmd := exec.Command("sh", "-c", "echo $XDG_CURRENT_DESKTOP")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		desktop := string(output)
		if contains(desktop, "KDE") {
			return "KDE"
		}
		if contains(desktop, "GNOME") {
			return "GNOME"
		}
	}

	// Try alternative detection
	cmd = exec.Command("sh", "-c", "echo $DESKTOP_SESSION")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		desktop := string(output)
		if contains(desktop, "kde") || contains(desktop, "plasma") {
			return "KDE"
		}
		if contains(desktop, "gnome") {
			return "GNOME"
		}
	}

	return "Unknown"
}

func showContextMenuKDE(path string) error {
	// Try to use dolphin to show the file
	cmd := exec.Command("dolphin", "--select", path)
	err := cmd.Start()
	if err != nil {
		// Fallback to opening the parent directory
		return showContextMenuFallback(path)
	}
	return nil
}

func showContextMenuGNOME(path string) error {
	// Try to use nautilus to show the file
	cmd := exec.Command("nautilus", "--select", path)
	err := cmd.Start()
	if err != nil {
		// Fallback to opening the parent directory
		return showContextMenuFallback(path)
	}
	return nil
}

func showContextMenuFallback(path string) error {
	// Open the parent directory containing the file
	dir := filepath.Dir(path)
	cmd := exec.Command("xdg-open", dir)
	return cmd.Start()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr)))
}
