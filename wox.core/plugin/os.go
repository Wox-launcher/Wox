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

func IsSupportedOS(os string) bool {
	osUpper := strings.ToUpper(os)
	if osUpper == string(PLUGIN_OS_WINDOWS) {
		return util.IsWindows()
	}
	if osUpper == string(PLUGIN_OS_DARWIN) {
		return util.IsMacOS()
	}
	if osUpper == string(PLUGIN_OS_LINUX) {
		return util.IsLinux()
	}

	return false
}

func IsSupportedOSAny(os []string) bool {
	for _, o := range os {
		if IsSupportedOS(o) {
			return true
		}
	}

	return false
}
