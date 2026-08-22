package window

import (
	"os"
	"strings"
)

// existingFilesystemDirectory returns path only when it names a real directory.
// Virtual shell folders, missing paths, and files are rejected.
func existingFilesystemDirectory(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "::") {
		return ""
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return ""
	}
	return path
}
