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

func microphonePermissionStatus() OSPermissionStatus {
	return OSPermissionUnsupported
}

func requestMicrophonePermission(_ *OSPermissionPrompt) OSPermissionStatus {
	return OSPermissionUnsupported
}
