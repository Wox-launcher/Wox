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

const linuxDesktopRelaunchAttemptedEnv = "WOX_LINUX_DESKTOP_RELAUNCH_ATTEMPTED"

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
		cmd.Env = append(os.Environ(), linuxDesktopRelaunchAttemptedEnv+"=1")
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
	if IsKDEDesktopSession() {
		commands = append(commands, []string{"kioclient", "exec", desktopFilePath})
	}

	return append(commands,
		[]string{"gio", "launch", desktopFilePath},
		[]string{"gtk-launch", strings.TrimSuffix(LinuxDesktopFileName(), ".desktop")},
	)
}

func IsLinuxWaylandSession() bool {
	return strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland") || os.Getenv("WAYLAND_DISPLAY") != ""
}

func IsLinuxLaunchedFromStableDesktopEntry() bool {
	return stableDesktopLaunchEnvMatches() || stableDesktopLaunchCgroupMatches()
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

	return strings.Contains(string(cgroupContent), "app-"+LinuxDesktopAppID+"-")
}
