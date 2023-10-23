package util

import (
	"runtime"
	"strings"
)

func IsWindows() bool {
	return strings.ToLower(runtime.GOOS) == "windows"
}

func IsMacOS() bool {
	return strings.ToLower(runtime.GOOS) == "darwin"
}

func IsLinux() bool {
	return strings.ToLower(runtime.GOOS) == "linux"
}
