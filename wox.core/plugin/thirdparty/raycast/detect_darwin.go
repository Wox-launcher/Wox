//go:build darwin

package raycast

import (
	"os"
	"path/filepath"
	"strings"
)

func detectInstalledRaycast() (string, string) {
	homes := []string{"/Applications", filepath.Join(os.Getenv("HOME"), "Applications")}
	for _, root := range homes {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasSuffix(entry.Name(), ".app") {
				continue
			}
			app := filepath.Join(root, entry.Name())
			if bundleIDHasPrefix(app, "com.raycast") {
				return app, ""
			}
		}
	}
	return "", ""
}

func bundleIDHasPrefix(app, prefix string) bool {
	raw, err := os.ReadFile(filepath.Join(app, "Contents", "Info.plist"))
	if err != nil {
		return false
	}
	text := string(raw)
	return strings.Contains(text, "<string>"+prefix)
}
