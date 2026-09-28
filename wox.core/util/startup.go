package util

import (
	"os"
	"slices"
)

const ArgNoThirdPartyPlugins = "--no-third-party-plugins"

// IsThirdPartyPluginsDisabled reports the process-local troubleshooting mode.
func IsThirdPartyPluginsDisabled() bool {
	return slices.Contains(os.Args[1:], ArgNoThirdPartyPlugins)
}
