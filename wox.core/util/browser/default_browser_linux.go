//go:build linux

package browser

import (
	"os/exec"
	"strings"
)

func detectDefaultBrowserID() string {
	output, err := exec.Command("xdg-settings", "get", "default-web-browser").Output()
	if err != nil {
		return ""
	}
	return browserIDFromLinuxDesktopFile(strings.TrimSpace(string(output)))
}
