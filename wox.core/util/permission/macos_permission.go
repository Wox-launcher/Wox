package permission

import (
	"context"
	"os"
	"runtime"
)

const macOSPermissionProbeEnvironment = "WOX_MACOS_PERMISSION_PROBE"

var probeMacOSPermissionStatusPlatform = func(ctx context.Context) (MacOSPermissionStatus, error) {
	return GetMacOSPermissionStatusDirect(ctx), nil
}

type MacOSPermissionType string

const (
	MacOSPermissionAccessibility  MacOSPermissionType = "accessibility"
	MacOSPermissionFullDiskAccess MacOSPermissionType = "fullDiskAccess"
)

type MacOSPermissionState string

const (
	MacOSPermissionGranted    MacOSPermissionState = "granted"
	MacOSPermissionNotGranted MacOSPermissionState = "notGranted"
	MacOSPermissionUnknown    MacOSPermissionState = "unknown"
)

type MacOSPermissionStatus struct {
	Accessibility  MacOSPermissionState `json:"accessibility"`
	FullDiskAccess MacOSPermissionState `json:"fullDiskAccess"`
}

// GetMacOSPermissionStatusDirect performs passive checks in the current process without triggering a system prompt.
func GetMacOSPermissionStatusDirect(ctx context.Context) MacOSPermissionStatus {
	status := MacOSPermissionStatus{
		Accessibility:  MacOSPermissionUnknown,
		FullDiskAccess: MacOSPermissionUnknown,
	}
	if runtime.GOOS != "darwin" {
		return status
	}
	status.Accessibility = MacOSPermissionNotGranted
	status.FullDiskAccess = GetFullDiskAccessPermissionState(ctx)
	if HasAccessibilityPermission(ctx) {
		status.Accessibility = MacOSPermissionGranted
	}
	return status
}

// ProbeMacOSPermissionStatus reads permission state through the platform probe so cached process state cannot hide a newly granted permission.
func ProbeMacOSPermissionStatus(ctx context.Context) (MacOSPermissionStatus, error) {
	return probeMacOSPermissionStatusPlatform(ctx)
}

// IsMacOSPermissionProbeProcess identifies the short-lived permission probe before normal Wox startup begins.
func IsMacOSPermissionProbeProcess() bool {
	return runtime.GOOS == "darwin" && os.Getenv(macOSPermissionProbeEnvironment) == "1"
}

// IsValidMacOSPermissionType rejects unknown values before they reach the native permission bridge.
func IsValidMacOSPermissionType(permissionType MacOSPermissionType) bool {
	switch permissionType {
	case MacOSPermissionAccessibility, MacOSPermissionFullDiskAccess:
		return true
	default:
		return false
	}
}
