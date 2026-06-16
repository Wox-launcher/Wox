package filesearch

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type rootPathMatcher struct {
	entries []rootPathMatchEntry
}

type rootPathMatchEntry struct {
	root        RootRecord
	matchPath   string
	matchPrefix string
}

func newRootPathMatcher(roots []RootRecord) rootPathMatcher {
	entries := make([]rootPathMatchEntry, 0, len(roots))
	for _, root := range roots {
		cleanRootPath := filepath.Clean(root.Path)
		matchPath := normalizeRootMatcherPath(cleanRootPath)
		entries = append(entries, rootPathMatchEntry{
			root:        root,
			matchPath:   matchPath,
			matchPrefix: rootMatcherPrefix(matchPath),
		})
	}

	// Optimization: change-feed callbacks may receive large event bursts. Sorting
	// the immutable matcher once lets the event path stop at the first hit while
	// preserving the existing longest-root ownership rule for dynamic roots.
	sort.SliceStable(entries, func(left int, right int) bool {
		return len(entries[left].matchPath) > len(entries[right].matchPath)
	})

	return rootPathMatcher{entries: entries}
}

func (m rootPathMatcher) empty() bool {
	return len(m.entries) == 0
}

func (m rootPathMatcher) findClean(cleanPath string) (RootRecord, bool) {
	matchPath := normalizeRootMatcherPath(cleanPath)
	for _, entry := range m.entries {
		if pathWithinCleanRootEntry(entry, matchPath) {
			return entry.root, true
		}
	}
	return RootRecord{}, false
}

func pathWithinCleanRootEntry(entry rootPathMatchEntry, cleanCandidate string) bool {
	if cleanCandidate == entry.matchPath {
		return true
	}
	return strings.HasPrefix(cleanCandidate, entry.matchPrefix)
}

func rootMatcherPrefix(matchPath string) string {
	separator := string(os.PathSeparator)
	if strings.HasSuffix(matchPath, separator) {
		return matchPath
	}
	return matchPath + separator
}

func normalizeRootMatcherPath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	// Bug fix: Windows root matching must not depend on path casing from USN or
	// fallback events. Normalizing once per candidate keeps matching cheap while
	// retaining Windows' case-insensitive ownership semantics.
	return strings.ToLower(path)
}

func cleanPathsOverlap(leftPath string, rightPath string) bool {
	leftMatchPath := normalizeRootMatcherPath(leftPath)
	rightMatchPath := normalizeRootMatcherPath(rightPath)
	left := rootPathMatchEntry{
		matchPath:   leftMatchPath,
		matchPrefix: rootMatcherPrefix(leftMatchPath),
	}
	right := rootPathMatchEntry{
		matchPath:   rightMatchPath,
		matchPrefix: rootMatcherPrefix(rightMatchPath),
	}
	return pathWithinCleanRootEntry(left, right.matchPath) || pathWithinCleanRootEntry(right, left.matchPath)
}
