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
	} else if target := windowsAppExecutablePath(*info); target != "" {
		// Known Folder entries name a real executable. Share its default launch key,
		// but retain the original Shell path when this entry represents the group.
		info.launchKey = shell.DesktopLaunchKey(target)
	}
}

// windowsAppExecutablePath resolves only bare executables and Known Folder Shell entries.
// Packaged apps, web apps and opaque AUMIDs must retain their own activation semantics.
func windowsAppExecutablePath(info appInfo) string {
	target := info.Path
	if info.Type == AppTypeAppsFolder && isAppsFolderIndexedApp(info) {
		target = resolveInboxAppsFolderPath(strings.TrimPrefix(info.Path, "shell:AppsFolder\\"))
	} else if info.Type != AppTypeDesktop {
		return ""
	}
	if !filepath.IsAbs(target) || !strings.EqualFold(filepath.Ext(target), ".exe") {
		return ""
	}
	return target
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

// deduplicateWindowsApps merges equivalent shortcuts and lets a shortcut replace its bare exe or Known Folder entry.
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
		if !found {
			if target := windowsAppExecutablePath(info); target != "" {
				index, found = byShortcutTarget[appPathMatchKey(target)]
			}
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
		// Shortcuts sort before Known Folder entries and executables. Keep the highest-priority
		// shortcut for their target without merging distinct shortcuts with each other.
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

// windowsAppEntryPriority prefers Start Menu and desktop links, other links, AppsFolder, then bare exe.
func windowsAppEntryPriority(path string, roots []string) int {
	if strings.HasPrefix(path, "shell:AppsFolder\\") {
		return 4
	}
	if strings.EqualFold(filepath.Ext(path), ".exe") {
		return 5
	}
	key := appPathMatchKey(path)
	for i, root := range roots {
		if root != "" && strings.HasPrefix(key, root+string(filepath.Separator)) {
			return min(i, 2)
		}
	}
	return 3
}
