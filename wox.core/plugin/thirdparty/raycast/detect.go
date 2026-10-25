package raycast

import (
	"os"
	"path/filepath"
	"strings"

	"wox/util"
)

const installMarker = "installed"

// detectInstall reports a Raycast installation. Test mode never scans the developer machine.
func detectInstall() (location, version string) {
	if root := strings.TrimSpace(os.Getenv(util.TestRaycastInstallEnv)); root != "" || util.IsTestMode() {
		if root == "" {
			return "", ""
		}
		if _, err := os.Stat(filepath.Join(root, installMarker)); err != nil {
			return "", ""
		}
		return root, ""
	}
	return detectInstalledRaycast()
}
