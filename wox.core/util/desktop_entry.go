package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// LinuxDesktopAppID is Wox's stable desktop id for Linux portals and shells.
	LinuxDesktopAppID = "io.github.WoxLauncher.Wox"
	// LinuxDesktopWMClass is the X11 WM_CLASS / StartupWMClass paired with the desktop id.
	LinuxDesktopWMClass  = "wox"
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
		// %U accepts both wox:// URLs and file:// / local .wox paths.
		execLine += " %U"
	}

	lines := []string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Wox",
		"Comment=Launch Wox",
		execLine,
		"Icon=" + LinuxDesktopAppID,
		"Categories=Utility;",
		"MimeType=" + pluginPackageURLMIME + ";" + PluginPackageMIMEType + ";",
		"Terminal=false",
		"StartupWMClass=" + LinuxDesktopWMClass,
		"X-KDE-DBUS-Restricted-Interfaces=org.kde.KWin.ScreenShot2",
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

// isEphemeralDebugExecutable reports Delve/VS Code debug binaries that must
// not be written into the user desktop entry or used as a relaunch target.
func isEphemeralDebugExecutable(path string) bool {
	return strings.HasPrefix(filepath.Base(path), "__debug_bin")
}

func linuxMimeDirectory() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".local", "share", "mime"), nil
}

func linuxPluginPackageMimePath() (string, error) {
	mimeDir, err := linuxMimeDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(mimeDir, "packages", LinuxDesktopAppID+".xml"), nil
}

func buildLinuxPluginPackageMimeType() string {
	return strings.Join([]string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<mime-info xmlns="http://www.freedesktop.org/standards/shared-mime-info">`,
		`  <mime-type type="` + PluginPackageMIMEType + `">`,
		`    <comment>Wox Plugin Package</comment>`,
		`    <glob pattern="*` + PluginPackageExtension + `"/>`,
		`  </mime-type>`,
		`</mime-info>`,
		``,
	}, "\n")
}

// writeLinuxPluginPackageMimeType installs the per-user MIME definition so
// file managers can associate *.wox archives with Wox.
func writeLinuxPluginPackageMimeType() error {
	path, err := linuxPluginPackageMimePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create MIME package directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(buildLinuxPluginPackageMimeType()), 0644); err != nil {
		return fmt.Errorf("failed to write MIME type: %w", err)
	}
	return nil
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
