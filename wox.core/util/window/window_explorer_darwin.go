//go:build darwin

package window

import (
	"os/exec"
	"strings"
)

// GetActiveFileExplorerPath returns the filesystem path of the currently active
// Finder window on macOS, or an empty string if the foreground window is not
// Finder or the path cannot be determined.
func GetActiveFileExplorerPath() string {
	// Use AppleScript to get the current Finder window path
	script := `
tell application "System Events"
	set frontApp to name of first application process whose frontmost is true
end tell

if frontApp is "Finder" then
	tell application "Finder"
		if (count of Finder windows) > 0 then
			set currentWindow to window 1
			set currentPath to (POSIX path of (target of currentWindow as alias))
			return currentPath
		end if
	end tell
end if

return ""
`

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	path := strings.TrimSpace(string(output))
	return path
}
