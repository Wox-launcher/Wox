//go:build !windows

package column

import (
	"os"
	"path/filepath"
)

// replaceManifest persists the directory entry after the immutable files have been synced.
func replaceManifest(source, destination string) (bool, error) {
	if err := os.Rename(source, destination); err != nil {
		return false, err
	}
	dir, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return true, err
	}
	defer dir.Close()
	return true, dir.Sync()
}
