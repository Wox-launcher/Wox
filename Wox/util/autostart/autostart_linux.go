package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

func setAutostart(enable bool) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	autostartDir := filepath.Join(homeDir, ".config", "autostart")
	desktopFilePath := filepath.Join(autostartDir, "wox-launcher.desktop")

	if enable {
		if err := os.MkdirAll(autostartDir, 0755); err != nil {
			return fmt.Errorf("failed to create autostart directory: %w", err)
		}
		return createDesktopFile(desktopFilePath)
	} else {
		return os.Remove(desktopFilePath)
	}
}

func createDesktopFile(desktopFilePath string) error {
	desktopFileContent := `[Desktop Entry]
Type=Application
Name=Wox Launcher
Exec={{ .ExePath }}
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
`

	tmpl, err := template.New("desktop").Parse(desktopFileContent)
	if err != nil {
		return fmt.Errorf("failed to parse desktop file template: %w", err)
	}

	file, err := os.Create(desktopFilePath)
	if err != nil {
		return fmt.Errorf("failed to create desktop file: %w", err)
	}
	defer file.Close()

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	err = tmpl.Execute(file, struct{ ExePath string }{ExePath: exePath})
	if err != nil {
		return fmt.Errorf("failed to write desktop file: %w", err)
	}

	return nil
}
