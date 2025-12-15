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

func IsAllSupportedOS(os []string) bool {
	for _, o := range os {
		if !strings.EqualFold(o, string(PLUGIN_OS_WINDOWS)) && !strings.EqualFold(o, string(PLUGIN_OS_DARWIN)) && !strings.EqualFold(o, string(PLUGIN_OS_LINUX)) {
			return false
		}
	}

	return true
}

func IsAnySupportedInCurrentOS(supportedOS []string) bool {
	for _, o := range supportedOS {
		if strings.EqualFold(o, string(util.GetCurrentPlatform())) {
			return true
		}
	}

	return false
}
