package util

import (
	"runtime"
	"strings"
)

const (
	PlatformWindows = "windows"
	PlatformMacOS   = "darwin"
	PlatformLinux   = "linux"
)

func IsWindows() bool {
	return strings.ToLower(runtime.GOOS) == PlatformWindows
}

func IsMacOS() bool {
	return strings.ToLower(runtime.GOOS) == PlatformMacOS
}

func IsLinux() bool {
	return strings.ToLower(runtime.GOOS) == PlatformLinux
}

func GetCurrentPlatform() string {
	return strings.ToLower(runtime.GOOS)
}
