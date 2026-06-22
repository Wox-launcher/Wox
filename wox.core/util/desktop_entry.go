package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// LinuxDesktopAppID is Wox's stable desktop id for Linux portals and shells.
	LinuxDesktopAppID    = "io.github.WoxLauncher.Wox"
	linuxDesktopFile     = LinuxDesktopAppID + ".desktop"
	linuxDesktopIconFile = LinuxDesktopAppID + ".png"
)

// LinuxDesktopEntryPath returns the per-user desktop entry path used as Wox's
// stable Linux application identity.
func LinuxDesktopEntryPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".local", "share", "applications", linuxDesktopFile), nil
}

// LinuxAutostartDesktopEntryPath returns the per-user autostart entry path with
// the same desktop id used by the primary application entry.
func LinuxAutostartDesktopEntryPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "autostart", linuxDesktopFile), nil
}

// LinuxDesktopIconPath returns the per-user icon path referenced by Wox's
// generated desktop entry.
func LinuxDesktopIconPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".local", "share", "icons", "hicolor", "256x256", "apps", linuxDesktopIconFile), nil
}

// LinuxIconThemePath returns the user icon theme root containing Wox's icon.
func LinuxIconThemePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".local", "share", "icons", "hicolor"), nil
}

// BuildLinuxDesktopEntry builds Wox's Linux desktop entry using APPIMAGE when
// available so distributed AppImage runs keep a stable executable target.
func BuildLinuxDesktopEntry(includeURLField bool, autostart bool) (string, error) {
	execPath, err := linuxDesktopExecPath()
	if err != nil {
		return "", err
	}

	execLine := fmt.Sprintf("Exec=%s", quoteDesktopExecArg(execPath))
	if includeURLField {
		execLine += " %u"
	}

	lines := []string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Wox",
		"Comment=Launch Wox",
		execLine,
		"Icon=" + LinuxDesktopAppID,
		"Categories=Utility;",
		"MimeType=x-scheme-handler/wox;",
		"Terminal=false",
		"StartupWMClass=wox",
	}

	if autostart {
		lines = append(lines,
			"Hidden=false",
			"NoDisplay=false",
			"X-GNOME-Autostart-enabled=true",
		)
	}

	return strings.Join(lines, "\n") + "\n", nil
}

// WriteLinuxDesktopEntry writes a desktop entry and creates its parent
// directory when needed.
func WriteLinuxDesktopEntry(path string, includeURLField bool, autostart bool) error {
	desktopEntry, err := BuildLinuxDesktopEntry(includeURLField, autostart)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create desktop entry directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(desktopEntry), 0644); err != nil {
		return fmt.Errorf("failed to write desktop entry: %w", err)
	}
	return nil
}

// LinuxDesktopFileName returns the stable desktop file name registered with
// xdg-mime and installed into AppImage metadata.
func LinuxDesktopFileName() string {
	return linuxDesktopFile
}

func linuxDesktopExecPath() (string, error) {
	if appImagePath := os.Getenv("APPIMAGE"); appImagePath != "" {
		return appImagePath, nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	return exePath, nil
}

func quoteDesktopExecArg(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"$", "\\$",
		"`", "\\`",
	)
	return `"` + replacer.Replace(value) + `"`
}
