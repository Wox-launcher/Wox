package permission

import "context"

func HasAccessibilityPermission(ctx context.Context) bool {
	return true
}

func GrantAccessibilityPermission(ctx context.Context) {

}

func OpenPrivacySecuritySettings(ctx context.Context) {

}

// RequestMicrophonePermission is a no-op on Windows because audio capture handles access directly.
func RequestMicrophonePermission(ctx context.Context) bool {
	return true
}
