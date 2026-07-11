package speech

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// validateNativeLibraryPlatformSignatures prevents macOS from killing Wox during dlopen.
func validateNativeLibraryPlatformSignatures(libDir string, names []string) error {
	for _, name := range names {
		path := filepath.Join(libDir, name)
		if output, err := exec.Command("/usr/bin/codesign", "--verify", "--strict", "--verbose=2", path).CombinedOutput(); err != nil {
			return fmt.Errorf("invalid code signature for %s: %s: %w", name, string(output), err)
		}
	}
	return nil
}
