//go:build wox_automation

package plugin

import (
	"os"
	"strings"
)

// automationStoreManifestURL keeps store smoke tests on a controlled local catalog.
func automationStoreManifestURL() string {
	return strings.TrimSpace(os.Getenv("WOX_AUTOMATION_PLUGIN_STORE_URL"))
}
