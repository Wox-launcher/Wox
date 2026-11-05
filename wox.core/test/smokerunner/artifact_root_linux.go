//go:build linux

package main

import (
	"os"
	"path/filepath"
)

// defaultSmokeArtifactRoot keeps isolated plugin data on the user's filesystem,
// where desktop trash services can uninstall it even when /tmp is a system mount.
func defaultSmokeArtifactRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".cache", "wox-smoke")
}
