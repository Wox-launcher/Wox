package app

import (
	"os"
	"path/filepath"
	"strings"

	"wox/util/filesearch"
)

// appWatchDirectoryAllowed applies the union of scan scopes, including overlapping
// user and system roots. A watch is needed only where the scanner reads entries.
func (a *ApplicationPlugin) appWatchDirectoryAllowed(path string, directories []appDirectory, extensions []string) bool {
	for _, directory := range directories {
		directory.Path = filepath.Clean(directory.Path)
		if !directory.trackChanges {
			continue
		}
		if a.pathCacheKey(path) == a.pathCacheKey(filepath.Clean(directory.Path)) {
			return true
		}
		if !a.isAppPathInDirectoryScope(filepath.Join(path, ".wox-watch-entry"), directory) {
			continue
		}
		// Application bundles are leaf entries, not subtrees to enumerate.
		insideApp := false
		for parent := path; a.isPathAtOrUnderDirectory(parent, directory.Path) && a.pathCacheKey(parent) != a.pathCacheKey(directory.Path); parent = filepath.Dir(parent) {
			if a.isAppPathExtensionMatch(parent, extensions) {
				insideApp = true
				break
			}
		}
		if insideApp {
			continue
		}
		matchers := a.getIgnoreRuleMatchersSnapshot()
		ignored := false
		for parent := path; a.isPathAtOrUnderDirectory(parent, directory.Path) && a.pathCacheKey(parent) != a.pathCacheKey(directory.Path); parent = filepath.Dir(parent) {
			if _, matched := a.matchIgnoreRuleCandidates(parent, []string{parent}, matchers); matched {
				ignored = true
				break
			}
		}
		if !ignored {
			return true
		}
	}
	return false
}

// scopedAppSignal rejects events outside scan policy. FSEvents reports bundle
// contents individually; those edits refresh the owning .app, never its Contents
// directory. Removing a resource is a bundle modification, not an app uninstall.
func (a *ApplicationPlugin) scopedAppSignal(signal filesearch.ChangeSignal, directories []appDirectory, extensions []string) (filesearch.ChangeSignal, bool) {
	if signal.Kind != filesearch.ChangeSignalKindDirtyPath {
		return signal, true
	}
	if signal.Path == "" {
		return signal, false
	}
	// A scoped FSEvents loss marker names a directory but has no precise
	// operation. It still requires the same bounded repair as a directory edit.
	if signal.FeedType == filesearch.RootFeedTypeFSEvents && signal.PathIsDir && signal.SemanticKind == filesearch.ChangeSemanticKindUnknown {
		signal.SemanticKind = filesearch.ChangeSemanticKindModify
	}
	path := filepath.Clean(signal.Path)
	for _, directory := range directories {
		directory.Path = filepath.Clean(directory.Path)
		if !directory.trackChanges || !a.isPathAtOrUnderDirectory(path, directory.Path) {
			continue
		}
		candidate := path
		for parent := path; a.isPathAtOrUnderDirectory(parent, directory.Path) && a.pathCacheKey(parent) != a.pathCacheKey(directory.Path); parent = filepath.Dir(parent) {
			// Only directory bundles have meaningful nested file events.
			if strings.EqualFold(filepath.Ext(parent), ".app") && a.isAppPathExtensionMatch(parent, extensions) {
				candidate = parent
			}
		}
		if a.isAppPathExtensionMatch(candidate, extensions) {
			if !a.isAppPathInDirectoryScope(candidate, directory) || !a.appWatchDirectoryAllowed(filepath.Dir(candidate), directories, extensions) {
				continue
			}
			if _, ignored := a.matchIgnoreRuleCandidates(candidate, []string{candidate}, a.getIgnoreRuleMatchersSnapshot()); ignored {
				continue
			}
			signal.Path = candidate
			signal.PathIsDir = false
			if candidate != path || signal.SemanticKind == filesearch.ChangeSemanticKindMetadata {
				signal.SemanticKind = filesearch.ChangeSemanticKindModify
				signal.PathTypeKnown = true
			}
			return signal, true
		}
		if a.appWatchDirectoryAllowed(path, directories, extensions) {
			return signal, true
		}
	}
	return signal, false
}

// getLocalAppDirectoriesForChange preserves every overlapping scan policy. A
// shallow system root must not shadow a deeper user root, and exclusions remain
// anchored at their original root when reconciling a child directory.
func (a *ApplicationPlugin) getLocalAppDirectoriesForChange(directoryPath string, directories []appDirectory) []appDirectory {
	cleanDirectory := filepath.Clean(directoryPath)
	var scopes []appDirectory
	for _, root := range directories {
		if !root.trackChanges {
			continue
		}
		cleanRoot := filepath.Clean(root.Path)
		if !a.isPathAtOrUnderDirectory(cleanDirectory, cleanRoot) {
			continue
		}
		if cleanDirectory != cleanRoot && !a.isAppPathInDirectoryScope(filepath.Join(cleanDirectory, ".wox-scan-entry"), root) {
			continue
		}
		remainingDepth := root.RecursiveDepth
		if rel, err := filepath.Rel(cleanRoot, cleanDirectory); err == nil && rel != "." {
			remainingDepth -= len(strings.Split(rel, string(os.PathSeparator)))
		}
		local := appDirectory{Path: cleanDirectory, Recursive: root.Recursive && remainingDepth > 0, RecursiveDepth: max(0, remainingDepth)}
		for _, exclude := range root.RecursiveExcludes {
			local.excludeAbsPaths = append(local.excludeAbsPaths, filepath.Join(cleanRoot, exclude))
		}
		scopes = append(scopes, local)
	}
	return scopes
}
