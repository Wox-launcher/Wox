package plugin

import (
	"runtime"
	"strings"
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
		return strings.ToLower(runtime.GOOS) == "windows"
	}
	if osUpper == string(PLUGIN_OS_DARWIN) {
		return strings.ToLower(runtime.GOOS) == "darwin"
	}
	if osUpper == string(PLUGIN_OS_LINUX) {
		return strings.ToLower(runtime.GOOS) == "linux"
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
