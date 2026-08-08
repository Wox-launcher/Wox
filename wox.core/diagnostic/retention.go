package diagnostic

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const retainedCrashArtifacts = 5

func (m *Manager) retainNewestCrashFiles(directory string, extension string, count int) {
	// Reports may contain memory fragments, so bound disk usage and exposure even
	// when Wox crashes repeatedly before the user files an issue.
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	type candidate struct {
		path    string
		modTime time.Time
	}
	files := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), extension) {
			continue
		}
		info, statErr := entry.Info()
		if statErr == nil {
			files = append(files, candidate{path: filepath.Join(directory, entry.Name()), modTime: info.ModTime()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.After(files[j].modTime) })
	if len(files) <= count {
		return
	}
	for _, file := range files[count:] {
		_ = os.Remove(file.path)
	}
}
