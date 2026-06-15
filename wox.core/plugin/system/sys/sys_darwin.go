//go:build darwin

package sys

import (
	"fmt"
	"os/exec"
	"wox/util/shell"
)

func runAppleScript(script string) (*exec.Cmd, error) {
	return shell.Run("osascript", "-e", script)
}

func isVolumeCommandAvailable() bool {
	return true
}

func runSetVolumeCommand(percent int) (*exec.Cmd, error) {
	return runAppleScript(fmt.Sprintf("set volume output volume %d", percent))
}

func runVolumeUpCommand() (*exec.Cmd, error) {
	return runAppleScript(`set currentVolume to output volume of (get volume settings)
set nextVolume to currentVolume + 6
if nextVolume > 100 then set nextVolume to 100
set volume output volume nextVolume`)
}

func runVolumeDownCommand() (*exec.Cmd, error) {
	return runAppleScript(`set currentVolume to output volume of (get volume settings)
set nextVolume to currentVolume - 6
if nextVolume < 0 then set nextVolume to 0
set volume output volume nextVolume`)
}

func runToggleMuteCommand() (*exec.Cmd, error) {
	return runAppleScript(`set currentMute to output muted of (get volume settings)
set volume output muted (not currentMute)`)
}

func isSleepCommandAvailable() bool {
	return true
}

func runSleepCommand() (*exec.Cmd, error) {
	return shell.Run("pmset", "sleepnow")
}

func isSleepDisplaysCommandAvailable() bool {
	return true
}

func runSleepDisplaysCommand() (*exec.Cmd, error) {
	return shell.Run("pmset", "displaysleepnow")
}

func isLogoutCommandAvailable() bool {
	return true
}

func runLogoutCommand() (*exec.Cmd, error) {
	return runAppleScript(`tell application "System Events" to log out`)
}

func isEjectAllDisksCommandAvailable() bool {
	return true
}

func runEjectAllDisksCommand() (*exec.Cmd, error) {
	return runAppleScript(`tell application "Finder" to eject (every disk whose ejectable is true)`)
}

func isShowDesktopCommandAvailable() bool {
	return true
}

func runShowDesktopCommand() (*exec.Cmd, error) {
	return runAppleScript(`tell application "System Events" to key code 103`)
}

func isShowTaskViewCommandAvailable() bool {
	return false
}

func runShowTaskViewCommand() (*exec.Cmd, error) {
	return nil, fmt.Errorf("task view is not supported on macOS")
}

func isShowScreenSaverCommandAvailable() bool {
	return true
}

func runShowScreenSaverCommand() (*exec.Cmd, error) {
	return shell.Run("open", "/System/Library/CoreServices/ScreenSaverEngine.app")
}

func isQuitAllApplicationsCommandAvailable() bool {
	return true
}

func runQuitAllApplicationsCommand() (*exec.Cmd, error) {
	return runAppleScript(`tell application "System Events"
	set visibleApps to name of every application process whose visible is true and name is not "Finder" and name is not "Wox"
end tell
repeat with appName in visibleApps
	try
		tell application appName to quit
	end try
end repeat`)
}

func runHideAllAppsExceptFrontmostCommand() (*exec.Cmd, error) {
	return runAppleScript(`tell application "System Events"
	set frontApp to name of first application process whose frontmost is true
	set visible of every application process whose visible is true and name is not frontApp to false
end tell`)
}

func runUnhideAllHiddenAppsCommand() (*exec.Cmd, error) {
	return runAppleScript(`tell application "System Events" to set visible of every application process to true`)
}

func isToggleSystemAppearanceCommandAvailable() bool {
	return true
}

func runToggleSystemAppearanceCommand() (*exec.Cmd, error) {
	return runAppleScript(`tell application "System Events" to tell appearance preferences to set dark mode to not dark mode`)
}

func isToggleHiddenFilesCommandAvailable() bool {
	return true
}

func runToggleHiddenFilesCommand() (*exec.Cmd, error) {
	return shell.Run("sh", "-c", `current=$(defaults read com.apple.finder AppleShowAllFiles 2>/dev/null || echo false); if [ "$current" = "1" ] || [ "$current" = "TRUE" ] || [ "$current" = "true" ]; then defaults write com.apple.finder AppleShowAllFiles -bool false; else defaults write com.apple.finder AppleShowAllFiles -bool true; fi; killall Finder`)
}

func runLockCommand() (*exec.Cmd, error) {
	return runAppleScript(`tell application "System Events" to keystroke "q" using {control down, command down}`)
}

func runEmptyTrashCommand() (*exec.Cmd, error) {
	return runAppleScript(`tell application "Finder" to empty trash`)
}

func isOpenSystemSettingsCommandAvailable() bool {
	return true
}

func runOpenSystemSettingsCommand() (*exec.Cmd, error) {
	return shell.Run("open", "x-apple.systempreferences:")
}

func runPlatformShutdownCommand() (*exec.Cmd, error) {
	return runAppleScript(`tell application "System Events" to shut down`)
}

func runPlatformRestartCommand() (*exec.Cmd, error) {
	return runAppleScript(`tell application "System Events" to restart`)
}
