package plugin

import (
	"strings"
	"wox/util"
)

type OS string

const (
	PLUGIN_OS_WINDOWS OS = "WINDOWS"
	PLUGIN_OS_DARWIN  OS = "DARWIN"
	PLUGIN_OS_LINUX   OS = "LINUX"
)

// Some published plugins use the product name macOS for the Darwin platform.
func canonicalPluginOS(name string) OS {
	if strings.EqualFold(name, "macOS") {
		return PLUGIN_OS_DARWIN
	}
	return OS(strings.ToUpper(name))
}

func IsAllSupportedOS(os []string) bool {
	for _, o := range os {
		switch canonicalPluginOS(o) {
		case PLUGIN_OS_WINDOWS, PLUGIN_OS_DARWIN, PLUGIN_OS_LINUX:
		default:
			return false
		}
	}

	return true
}

func IsAnySupportedInCurrentOS(supportedOS []string) bool {
	current := canonicalPluginOS(util.GetCurrentPlatform())
	for _, o := range supportedOS {
		if canonicalPluginOS(o) == current {
			return true
		}
	}

	return false
}
