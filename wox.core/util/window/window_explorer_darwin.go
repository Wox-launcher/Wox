//go:build darwin

package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa
#include <Cocoa/Cocoa.h>

int isFinder(int pid) {
    @autoreleasepool {
        NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
        if (app && [[app bundleIdentifier] isEqualToString:@"com.apple.finder"]) {
            return 1;
        }
        return 0;
    }
}
*/
import "C"
import (
	"os/exec"
	"strings"
)

// IsFileExplorer checks if the given PID belongs to Finder.
func IsFileExplorer(pid int) (bool, error) {
	if C.isFinder(C.int(pid)) == 1 {
		return true, nil
	}
	return false, nil
}

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

// NavigateActiveFileExplorer navigates the currently active Finder window to the specified path.
// Returns true if successful, false otherwise.
func NavigateActiveFileExplorer(targetPath string) bool {
	script := `
tell application "System Events"
	set frontApp to name of first application process whose frontmost is true
end tell

if frontApp is "Finder" then
	tell application "Finder"
		if (count of Finder windows) > 0 then
			set target of window 1 to (POSIX file "` + targetPath + `" as alias)
			return "ok"
		end if
	end tell
end if

return ""
`

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == "ok"
}
