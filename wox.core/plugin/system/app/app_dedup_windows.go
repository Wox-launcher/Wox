package app

import (
	"cmp"
	"context"
	"path/filepath"
	"slices"
	"strings"
	"wox/util"
	"wox/util/shell"

	"golang.org/x/sys/windows"
)

// populateAppLaunchKey reuses the launcher's conservative link reader during indexing, never while typing.
func populateAppLaunchKey(ctx context.Context, info *appInfo) {
	info.launchKey = ""
	info.shortcutTarget = ""
	if info.Type == AppTypeDesktop {
		info.launchKey = shell.DesktopLaunchKey(info.Path)
		if strings.EqualFold(filepath.Ext(info.Path), ".lnk") {
			// Hiding the bare exe does not require flattening the shortcut: its original
			// arguments, directory, elevation and Shell metadata remain in the retained .lnk.
			if target, err := resolveShortcutTarget(ctx, info.Path); err == nil && filepath.IsAbs(target) && strings.EqualFold(filepath.Ext(target), ".exe") {
				info.shortcutTarget = appPathMatchKey(target)
			}
		}
	}
}

// deduplicateAppLaunches uses the actual known folders, including redirected user desktops/start menus.
func deduplicateAppLaunches(apps []appInfo) []appInfo {
	roots := make([]string, 4)
	for i, id := range []*windows.KNOWNFOLDERID{windows.FOLDERID_Programs, windows.FOLDERID_CommonPrograms, windows.FOLDERID_Desktop, windows.FOLDERID_PublicDesktop} {
		if root, err := windows.KnownFolderPath(id, windows.KF_FLAG_DONT_VERIFY); err == nil {
			roots[i] = strings.ToLower(filepath.Clean(root))
		}
	}
	return deduplicateWindowsApps(apps, roots)
}

// deduplicateWindowsApps merges equivalent shortcuts and lets a shortcut replace its bare exe.
// Opaque shortcuts only merge with the same entry path, never with another shortcut by target alone.
// Sorting fixes the representative and alias order regardless of parallel indexing completion order.
func deduplicateWindowsApps(apps []appInfo, roots []string) []appInfo {
	ordered := slices.Clone(apps)
	slices.SortFunc(ordered, func(a, b appInfo) int {
		if order := cmp.Compare(windowsAppEntryPriority(a.Path, roots), windowsAppEntryPriority(b.Path, roots)); order != 0 {
			return order
		}
		if order := strings.Compare(appPathMatchKey(a.Path), appPathMatchKey(b.Path)); order != 0 {
			return order
		}
		if order := strings.Compare(a.Name, b.Name); order != 0 {
			return order
		}
		return strings.Compare(a.Path, b.Path)
	})
	type pathKey struct{ appType, path string }
	byPath, byLaunch := make(map[pathKey]int), make(map[string]int)
	byShortcutTarget := make(map[string]int)
	result := make([]appInfo, 0, len(ordered))
	for _, info := range ordered {
		path := pathKey{info.Type, appPathMatchKey(info.Path)}
		index, found := byPath[path]
		if !found && info.Type == AppTypeDesktop && strings.EqualFold(filepath.Ext(info.Path), ".exe") {
			index, found = byShortcutTarget[path.path]
		}
		if !found && info.launchKey != "" {
			index, found = byLaunch[info.launchKey]
		}
		if !found {
			index = len(result)
			info.SearchableNames = slices.Clone(info.SearchableNames)
			result = append(result, info)
		} else {
			result[index].SearchableNames = append(result[index].SearchableNames, info.GetSearchCandidates(info.Name)...)
		}
		if path.path != "" {
			byPath[path] = index
		}
		if info.launchKey != "" {
			byLaunch[info.launchKey] = index
		}
		// Shortcuts sort before executables. Keep the highest-priority shortcut as the
		// replacement for a bare exe, without merging distinct shortcuts with each other.
		if info.shortcutTarget != "" {
			if _, exists := byShortcutTarget[info.shortcutTarget]; !exists {
				byShortcutTarget[info.shortcutTarget] = index
			}
		}
	}
	for i := range result {
		result[i].SearchableNames = util.UniqueStrings(result[i].SearchableNames)
		slices.Sort(result[i].SearchableNames)
	}
	return result
}

// windowsAppEntryPriority prefers user Start Menu, common Start Menu, desktop, other links, then bare exe.
func windowsAppEntryPriority(path string, roots []string) int {
	if strings.EqualFold(filepath.Ext(path), ".exe") {
		return 4
	}
	key := appPathMatchKey(path)
	for i, root := range roots {
		if root != "" && strings.HasPrefix(key, root+string(filepath.Separator)) {
			return min(i, 2)
		}
	}
	return 3
}
