//go:build windows

package osvariant

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procRtlGetVersion = windows.NewLazySystemDLL("ntdll.dll").NewProc("RtlGetVersion")

type windowsOSVersionInfoEx struct {
	OSVersionInfoSize uint32
	MajorVersion      uint32
	MinorVersion      uint32
	BuildNumber       uint32
	PlatformId        uint32
	CSDVersion        [128]uint16
	ServicePackMajor  uint16
	ServicePackMinor  uint16
	SuiteMask         uint16
	ProductType       byte
	Reserved          byte
}

// GetCurrentPlatformVariant returns the current Windows theme variant, such as win10 or win11.
func GetCurrentPlatformVariant() string {
	versionInfo := windowsOSVersionInfoEx{
		OSVersionInfoSize: uint32(unsafe.Sizeof(windowsOSVersionInfoEx{})),
	}

	status, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&versionInfo)))
	if status != 0 {
		return ""
	}

	return windowsPlatformVariantForBuildNumber(versionInfo.BuildNumber)
}
