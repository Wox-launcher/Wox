//go:build !windows

package osvariant

// GetCurrentPlatformVariant returns an empty variant until non-Windows platforms define stable theme variants.
func GetCurrentPlatformVariant() string {
	return ""
}
