package ffmpeg

import (
	"os"
	"path/filepath"
	"strings"
)

// unwrapExecutableShim follows a Scoop shim to its real executable.
// The shim is itself a console program and can allocate a window after Wox hides it.
func unwrapExecutableShim(path string) string {
	if path == "" {
		return path
	}
	data, err := os.ReadFile(strings.TrimSuffix(path, filepath.Ext(path)) + ".shim")
	if err != nil {
		return path
	}
	target := shimTarget(string(data))
	if target == "" {
		return path
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return path
	}
	return target
}

func shimTarget(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "path" {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"`)
	}
	return ""
}
