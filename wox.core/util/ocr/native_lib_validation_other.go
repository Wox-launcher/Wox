//go:build !darwin

package ocr

func validateNativeLibraryPlatformSignaturesForPaths(sherpaLibDir string, onnxRuntimeLibDir string, names []string) error {
	return nil
}
