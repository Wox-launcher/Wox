//go:build !darwin

package speech

func validateNativeLibraryPlatformSignatures(libDir string, names []string) error {
	return nil
}
