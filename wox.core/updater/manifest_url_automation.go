//go:build wox_automation

package updater

import (
	"os"
	"strings"
)

// automationManifestURL allows smoke tests to use a controlled local update server.
func automationManifestURL() string {
	return strings.TrimSpace(os.Getenv("WOX_AUTOMATION_UPDATE_MANIFEST_URL"))
}
