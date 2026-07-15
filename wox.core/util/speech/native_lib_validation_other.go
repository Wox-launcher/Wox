//go:build !darwin

package speech

func validateNativeLibraryPlatformSignaturesForPaths(sherpaLibDir string, onnxRuntimeLibDir string, names []string) error {
	return nil
}
