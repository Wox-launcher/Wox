//go:build linux

package util

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	linuxDesktopRelaunchAttemptedEnv = "WOX_LINUX_DESKTOP_RELAUNCH_ATTEMPTED"
	linuxDesktopLaunchConfirmedEnv   = "WOX_LINUX_DESKTOP_ENTRY_LAUNCH_CONFIRMED"
)

// ShouldRelaunchLinuxFromDesktopEntry reports whether this process should hand
// off startup to the stable desktop entry before full initialization.
func ShouldRelaunchLinuxFromDesktopEntry(args []string) bool {
	if len(args) > 0 {
		return false
	}
	if os.Getenv(linuxDesktopRelaunchAttemptedEnv) == "1" {
		return false
	}

	return IsLinuxWaylandSession() && !IsLinuxLaunchedFromStableDesktopEntry()
}

// RelaunchLinuxFromDesktopEntry starts Wox through its registered desktop file
// so Wayland portals can associate the process with Wox's stable app id.
func RelaunchLinuxFromDesktopEntry(ctx context.Context) error {
	desktopFilePath, err := LinuxDesktopEntryPath()
	if err != nil {
		return err
	}
	if !IsFileExists(desktopFilePath) {
		return fmt.Errorf("Linux desktop entry does not exist: %s", desktopFilePath)
	}

	launchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	commands := linuxDesktopLaunchCommands(desktopFilePath)
	var launchErrors []string
	for _, command := range commands {
		cmd := exec.CommandContext(launchCtx, command[0], command[1:]...)
		cmd.Env = append(os.Environ(),
			linuxDesktopRelaunchAttemptedEnv+"=1",
			linuxDesktopLaunchConfirmedEnv+"="+desktopFilePath,
		)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		if err := cmd.Run(); err == nil {
			GetLogger().Info(ctx, fmt.Sprintf("launched Linux desktop entry via: %s", strings.Join(command, " ")))
			return nil
		} else {
			launchErrors = append(launchErrors, fmt.Sprintf("%s: %s", strings.Join(command, " "), err.Error()))
		}
	}

	return fmt.Errorf("failed to launch Linux desktop entry: %s", strings.Join(launchErrors, "; "))
}

func linuxDesktopLaunchCommands(desktopFilePath string) [][]string {
	commands := [][]string{}

	// systemd-run --user --scope gives Wox its own cgroup
	// (app-<app_id>-<pid>.scope) so xdg-desktop-portal can identify Wox by
	// its stable app id instead of inheriting the launcher's cgroup
	// (e.g. FileManager1 from gio launch), which causes portal backends
	// like GNOME's RemoteDesktop/Clipboard to reject the session with
	// NotAllowed (response code 2).
	if systemdScopeCommand := buildSystemdScopeLaunchCommand(desktopFilePath); systemdScopeCommand != nil {
		commands = append(commands, systemdScopeCommand)
	}

	if IsKDEDesktopSession() {
		commands = append(commands, []string{"kioclient", "exec", desktopFilePath})
	}

	return append(commands,
		[]string{"gio", "launch", desktopFilePath},
		[]string{"gtk-launch", strings.TrimSuffix(LinuxDesktopFileName(), ".desktop")},
	)
}

// buildSystemdScopeLaunchCommand returns a systemd-run --user command that
// starts the desktop entry in its own systemd service unit named
// app-<escaped-app_id>-<pid>.service. xdg-desktop-portal parses this unit
// name (from /proc/<pid>/cgroup) to recover the app id and grant portal
// access. Without this, gio/gtk-launch inherits the launcher's cgroup (e.g.
// FileManager1 from nautilus dbus activation), which causes GNOME's
// RemoteDesktop/Clipboard portal to reject the session with NotAllowed
// (response code 2).
//
// --no-block makes systemd-run return immediately after enqueueing the
// service so the old Wox process can exit. --scope is intentionally avoided
// because it blocks until the child exits.
// Returns nil if systemd-run is unavailable or the exec path cannot be resolved.
func buildSystemdScopeLaunchCommand(desktopFilePath string) []string {
	if _, err := exec.LookPath("systemd-run"); err != nil {
		return nil
	}

	execPath, err := linuxDesktopExecPath()
	if err != nil {
		return nil
	}

	unitName := fmt.Sprintf("app-%s-%d.service", escapeSystemdUnitName(LinuxDesktopAppID), os.Getpid())
	args := []string{
		"systemd-run", "--user", "--no-block",
		"--unit=" + unitName,
	}
	// systemd-run services start with a minimal environment and do not inherit
	// arbitrary variables from the launching process, so the relaunch control
	// env vars must be passed explicitly via -E to prevent a relaunch loop.
	for key, value := range map[string]string{
		linuxDesktopRelaunchAttemptedEnv: "1",
		linuxDesktopLaunchConfirmedEnv:   desktopFilePath,
	} {
		args = append(args, "-E", key+"="+value)
	}
	args = append(args, "--", execPath)
	return args
}

// escapeSystemdUnitName escapes characters that systemd forbids in unit names
// (only [a-zA-Z0-9:_-] are allowed unescaped). Dots in the app id must become
// \x2e so the unit name is valid and xdg-desktop-portal can reverse the
// escaping to recover the original app id.
func escapeSystemdUnitName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteString(fmt.Sprintf("\\x%02x", r))
		}
	}
	return b.String()
}

func IsLinuxWaylandSession() bool {
	return strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland") || os.Getenv("WAYLAND_DISPLAY") != ""
}

func IsLinuxLaunchedFromStableDesktopEntry() bool {
	return stableDesktopLaunchConfirmedByWox() || stableDesktopLaunchEnvMatches() || stableDesktopLaunchCgroupMatches()
}

// stableDesktopLaunchConfirmedByWox covers desktop-entry relaunches where the
// launcher or AppImage wrapper does not preserve a useful GIO PID or cgroup.
func stableDesktopLaunchConfirmedByWox() bool {
	confirmedDesktopFile := os.Getenv(linuxDesktopLaunchConfirmedEnv)
	if confirmedDesktopFile == "" {
		return false
	}

	stableDesktopFile, err := LinuxDesktopEntryPath()
	if err != nil {
		return filepath.Base(confirmedDesktopFile) == LinuxDesktopFileName()
	}

	return confirmedDesktopFile == stableDesktopFile
}

func stableDesktopLaunchEnvMatches() bool {
	if launchedDesktopFilePID := os.Getenv("GIO_LAUNCHED_DESKTOP_FILE_PID"); launchedDesktopFilePID != "" {
		if launchedDesktopFilePID != strconv.Itoa(os.Getpid()) {
			return false
		}
	}

	launchedDesktopFile := os.Getenv("GIO_LAUNCHED_DESKTOP_FILE")
	if launchedDesktopFile == "" {
		return false
	}
	if filepath.Base(launchedDesktopFile) != LinuxDesktopFileName() {
		return false
	}

	stableDesktopFile, err := LinuxDesktopEntryPath()
	if err != nil {
		return true
	}

	return launchedDesktopFile == stableDesktopFile
}

func stableDesktopLaunchCgroupMatches() bool {
	cgroupContent, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return false
	}

	// systemd escapes dots in unit names as \x2e, so the cgroup may contain
	// either the raw app id (e.g. when launched via a .scope with unescaped
	// name) or the systemd-escaped form. Match both.
	raw := "app-" + LinuxDesktopAppID + "-"
	escaped := "app-" + escapeSystemdUnitName(LinuxDesktopAppID) + "-"
	content := string(cgroupContent)
	return strings.Contains(content, raw) || strings.Contains(content, escaped)
}
