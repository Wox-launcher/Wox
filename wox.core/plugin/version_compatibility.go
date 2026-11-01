package plugin

import (
	"fmt"
	"wox/updater"
	"wox/util"

	"github.com/Masterminds/semver/v3"
)

const defaultMinWoxVersion = "2.0.0"

// SingleFilePluginMinWoxVersion is the first Wox release that can load
// single-file SDK plugins. Store manifests and CI reject older floors so
// previous Wox builds never attempt to install this artifact type.
const SingleFilePluginMinWoxVersion = "2.4.2"

func normalizeMinWoxVersion(minWoxVersion string) string {
	if minWoxVersion == "" {
		return defaultMinWoxVersion
	}
	return minWoxVersion
}

func ensureWoxVersionSupported(pluginName string, minWoxVersion string) error {
	requiredVersionText := normalizeMinWoxVersion(minWoxVersion)
	currentVersion, currentErr := semver.NewVersion(updater.CURRENT_VERSION)
	if currentErr != nil {
		return fmt.Errorf("current Wox version %q is invalid: %w", updater.CURRENT_VERSION, currentErr)
	}

	requiredVersion, requiredErr := semver.NewVersion(requiredVersionText)
	if requiredErr != nil {
		return fmt.Errorf("plugin %s requires an invalid MinWoxVersion %q: %w", pluginName, requiredVersionText, requiredErr)
	}

	// Development builds may contain unreleased APIs even when their version has not been bumped.
	if util.IsProd() && requiredVersion.GreaterThan(currentVersion) {
		return fmt.Errorf("plugin %s requires Wox %s or later, current Wox version is %s", pluginName, requiredVersion.String(), currentVersion.String())
	}

	return nil
}
