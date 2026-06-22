//go:build linux

package sys

import (
	"fmt"
	"os/exec"
	"wox/util/shell"
)

func isVolumeCommandAvailable() bool {
	return commandExists("pactl")
}

func runSetVolumeCommand(percent int) (*exec.Cmd, error) {
	return shell.Run("pactl", "set-sink-volume", "@DEFAULT_SINK@", fmt.Sprintf("%d%%", percent))
}

func runVolumeUpCommand() (*exec.Cmd, error) {
	return shell.Run("pactl", "set-sink-volume", "@DEFAULT_SINK@", "+5%")
}

func runVolumeDownCommand() (*exec.Cmd, error) {
	return shell.Run("pactl", "set-sink-volume", "@DEFAULT_SINK@", "-5%")
}

func runToggleMuteCommand() (*exec.Cmd, error) {
	return shell.Run("pactl", "set-sink-mute", "@DEFAULT_SINK@", "toggle")
}

func isSleepCommandAvailable() bool {
	return commandExists("systemctl")
}

func runSleepCommand() (*exec.Cmd, error) {
	return shell.Run("systemctl", "suspend")
}

func isSleepDisplaysCommandAvailable() bool {
	return commandExists("xset")
}

func runSleepDisplaysCommand() (*exec.Cmd, error) {
	return shell.Run("xset", "dpms", "force", "off")
}

func isLogoutCommandAvailable() bool {
	return commandExists("gnome-session-quit")
}

func runLogoutCommand() (*exec.Cmd, error) {
	return shell.Run("gnome-session-quit", "--logout", "--no-prompt")
}

func isEjectAllDisksCommandAvailable() bool {
	return false
}

func runEjectAllDisksCommand() (*exec.Cmd, error) {
	return nil, fmt.Errorf("eject all disks is not supported on Linux")
}

func isShowDesktopCommandAvailable() bool {
	return commandExists("wmctrl")
}

func runShowDesktopCommand() (*exec.Cmd, error) {
	return shell.Run("wmctrl", "-k", "on")
}

func isShowTaskViewCommandAvailable() bool {
	return false
}

func runShowTaskViewCommand() (*exec.Cmd, error) {
	return nil, fmt.Errorf("task view is not supported on Linux")
}

func isShowScreenSaverCommandAvailable() bool {
	return commandExists("xdg-screensaver")
}

func runShowScreenSaverCommand() (*exec.Cmd, error) {
	return shell.Run("xdg-screensaver", "activate")
}

func isQuitAllApplicationsCommandAvailable() bool {
	return false
}

func runQuitAllApplicationsCommand() (*exec.Cmd, error) {
	return nil, fmt.Errorf("quit all applications is not supported on Linux")
}

func runHideAllAppsExceptFrontmostCommand() (*exec.Cmd, error) {
	return nil, fmt.Errorf("hide all apps except frontmost is not supported on Linux")
}

func runUnhideAllHiddenAppsCommand() (*exec.Cmd, error) {
	return nil, fmt.Errorf("unhide all hidden apps is not supported on Linux")
}

func isToggleSystemAppearanceCommandAvailable() bool {
	return commandExists("gsettings")
}

func runToggleSystemAppearanceCommand() (*exec.Cmd, error) {
	return shell.Run("sh", "-c", `current=$(gsettings get org.gnome.desktop.interface color-scheme 2>/dev/null || echo "'default'"); if [ "$current" = "'prefer-dark'" ]; then gsettings set org.gnome.desktop.interface color-scheme default; else gsettings set org.gnome.desktop.interface color-scheme prefer-dark; fi`)
}

func isToggleHiddenFilesCommandAvailable() bool {
	return commandExists("gsettings")
}

func runToggleHiddenFilesCommand() (*exec.Cmd, error) {
	return shell.Run("sh", "-c", `current=$(gsettings get org.gnome.nautilus.preferences show-hidden-files 2>/dev/null || echo false); if [ "$current" = "true" ]; then gsettings set org.gnome.nautilus.preferences show-hidden-files false; else gsettings set org.gnome.nautilus.preferences show-hidden-files true; fi`)
}

func runLockCommand() (*exec.Cmd, error) {
	return shell.Run("loginctl", "lock-session")
}

func runEmptyTrashCommand() (*exec.Cmd, error) {
	return shell.Run("gio", "trash", "--empty")
}

func isOpenSystemSettingsCommandAvailable() bool {
	return commandExists("gnome-control-center")
}

func runOpenSystemSettingsCommand() (*exec.Cmd, error) {
	if commandExists("gnome-control-center") {
		return shell.Run("gnome-control-center")
	}
	return nil, fmt.Errorf("system settings is not supported on this Linux desktop")
}

func runPlatformShutdownCommand() (*exec.Cmd, error) {
	return shell.Run("systemctl", "poweroff")
}

func runPlatformRestartCommand() (*exec.Cmd, error) {
	return shell.Run("systemctl", "reboot")
}
