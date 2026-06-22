package plugin

import (
	"fmt"
	"wox/updater"

	"github.com/Masterminds/semver/v3"
)

const defaultMinWoxVersion = "2.0.0"

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

	// MinWoxVersion is a hard compatibility floor. Older install paths only stored
	// the field as metadata, so incompatible plugins could be installed and then
	// fail later in less obvious ways. Keep the comparison centralized so store,
	// local-package, startup, and dev reload paths all enforce the same rule.
	if requiredVersion.GreaterThan(currentVersion) {
		return fmt.Errorf("plugin %s requires Wox %s or later, current Wox version is %s", pluginName, requiredVersion.String(), currentVersion.String())
	}

	return nil
}
