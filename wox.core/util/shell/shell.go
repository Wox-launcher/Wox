package shell

import (
	"os"
	"path/filepath"
)

// getWorkingDirectory returns the appropriate working directory for a command.
// If name is a file path, returns the directory containing that file.
// Otherwise, returns the user's home directory.
func getWorkingDirectory(name string) string {
	if info, err := os.Stat(name); err == nil && !info.IsDir() {
		return filepath.Dir(name)
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		return homeDir
	}
	return ""
}
