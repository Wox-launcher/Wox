package permission

import "context"

func HasAccessibilityPermission(ctx context.Context) bool {
	return true
}

func GetFullDiskAccessPermissionState(ctx context.Context) MacOSPermissionState {
	return MacOSPermissionUnknown
}

func OpenPrivacySecuritySettings(ctx context.Context) {

}

// RequestMicrophonePermission is a no-op on Linux because audio capture handles access directly.
func RequestMicrophonePermission(ctx context.Context) bool {
	return true
}
