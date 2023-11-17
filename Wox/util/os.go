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

func IsArm64() bool {
	return strings.ToLower(runtime.GOARCH) == "arm64"
}

func IsAmd64() bool {
	return strings.ToLower(runtime.GOARCH) == "amd64"
}

func IsLinux() bool {
	return strings.ToLower(runtime.GOOS) == PlatformLinux
}

func GetCurrentPlatform() string {
	return strings.ToLower(runtime.GOOS)
}
