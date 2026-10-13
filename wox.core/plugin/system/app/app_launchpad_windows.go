package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"wox/util"
	"wox/util/fileicon"

	"golang.org/x/sys/windows"
)

// populateLaunchpadVisibility classifies entry points when rebuilding the query
// index, so directory edits also update restored and previously indexed apps.
func populateLaunchpadVisibility(ctx context.Context, info *appInfo, userDirectories ...appDirectory) {
	roots := make([]string, 0, 2)
	for _, id := range []*windows.KNOWNFOLDERID{windows.FOLDERID_StartMenu, windows.FOLDERID_CommonStartMenu} {
		if root, err := windows.KnownFolderPath(id, windows.KF_FLAG_DONT_VERIFY); err == nil {
			roots = append(roots, root)
		}
	}
	info.launchpadHidden = !windowsLaunchpadEntry(*info, roots, userDirectories...)
	if info.launchpadHidden || info.IsDefaultIcon || info.Icon.IsEmpty() {
		return
	}
	source := strings.TrimSpace(info.IconSourcePath)
	if source == "" && strings.EqualFold(filepath.Ext(info.Path), ".exe") {
		source = info.Path
	}
	if source != "" {
		// A successful shell lookup can still return the generic Windows executable
		// icon. IsDefaultIcon alone only detects Wox's own fallback artwork.
		dedicated, err := fileicon.HasDedicatedFileIcon(ctx, source)
		info.launchpadHidden = !dedicated || err != nil
		if err != nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("launchpad icon classification failed: path=%s err=%v", source, err))
		}
	} else if !isAppsFolderIndexedApp(*info) {
		// Only shell-provided artwork without a file source bypasses extraction.
		// AppsFolder .msc entries have real sources and must pass the check above.
		info.launchpadHidden = true
	}
}

// defaultAppDirectories exposes the actual redirected desktop as an editable root.
func defaultAppDirectories() string {
	desktop, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, windows.KF_FLAG_DONT_VERIFY)
	if err != nil {
		return "[]"
	}
	data, _ := json.Marshal([]map[string]string{{"Path": desktop}})
	return string(data)
}

// windowsLaunchpadEntry accepts Start Menu, AppsFolder and explicitly configured
// roots, using the same depth and exclusion rules as the directory scanner.
func windowsLaunchpadEntry(info appInfo, startMenus []string, userDirectories ...appDirectory) bool {
	if isAppsFolderIndexedApp(info) {
		return true
	}
	if info.Type != AppTypeDesktop {
		return false
	}
	for _, directory := range userDirectories {
		if (&ApplicationPlugin{}).isAppPathInDirectoryScope(info.Path, directory) {
			return true
		}
	}
	key := appPathMatchKey(info.Path)
	for _, root := range startMenus {
		if root != "" && strings.HasPrefix(key, appPathMatchKey(root)+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
