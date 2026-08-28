//go:build !windows && !linux

package osvariant

// GetCurrentPlatformVariant returns an empty variant until this platform defines stable theme variants.
func GetCurrentPlatformVariant() string {
	return ""
}
