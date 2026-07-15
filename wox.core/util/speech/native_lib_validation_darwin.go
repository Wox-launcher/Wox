package speech

import (
	"fmt"
	"os/exec"
)

// validateNativeLibraryPlatformSignaturesForPaths prevents macOS from killing Wox during dlopen.
func validateNativeLibraryPlatformSignaturesForPaths(sherpaLibDir string, onnxRuntimeLibDir string, names []string) error {
	for _, name := range names {
		path := nativeLibraryPath(sherpaLibDir, onnxRuntimeLibDir, name)
		if output, err := exec.Command("/usr/bin/codesign", "--verify", "--strict", "--verbose=2", path).CombinedOutput(); err != nil {
			return fmt.Errorf("invalid code signature for %s: %s: %w", name, string(output), err)
		}
	}
	return nil
}
