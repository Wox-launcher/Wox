package omarchytheme

import (
	"os"
	"strings"
)

// availableIn reports whether an Omarchy install directory or a resolved palette file is present.
func availableIn(shareDir, colorsFile string) bool {
	if dirExists(shareDir) {
		return true
	}
	info, err := os.Stat(colorsFile)
	return err == nil && info.Mode().IsRegular()
}

func dirExists(path string) bool {
	info, err := os.Stat(strings.TrimSpace(path))
	return err == nil && info.IsDir()
}
