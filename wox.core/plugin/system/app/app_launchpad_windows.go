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

// populateLaunchpadVisibility applies directory scope to the cached icon metadata.
// Directory edits must not repeat native icon extraction for unchanged apps.
func populateLaunchpadVisibility(ctx context.Context, info *appInfo, userDirectories ...appDirectory) {
	roots := make([]string, 0, 2)
	for _, id := range []*windows.KNOWNFOLDERID{windows.FOLDERID_StartMenu, windows.FOLDERID_CommonStartMenu} {
		if root, err := windows.KnownFolderPath(id, windows.KF_FLAG_DONT_VERIFY); err == nil {
			roots = append(roots, root)
		}
	}
	info.launchpadHidden = !info.HasDedicatedIcon || !windowsLaunchpadEntry(*info, roots, userDirectories...)
}

// populateLaunchpadIconMetadata classifies native artwork once when an app is parsed.
// The existing app and icon-source freshness checks invalidate this persisted result.
func populateLaunchpadIconMetadata(ctx context.Context, info *appInfo) {
	info.HasDedicatedIcon = false
	if info.IsDefaultIcon || info.Icon.IsEmpty() {
		return
	}
	if info.ResolvedIconPath != "" {
		// Classify the same pixels shown in the grid. A resource DLL's first icon
		// need not match the indexed shortcut artwork, and can even be generic.
		dedicated, err := fileicon.HasDedicatedImageIcon(ctx, info.ResolvedIconPath)
		info.HasDedicatedIcon = dedicated && err == nil
		if err != nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("launchpad resolved icon classification failed: path=%s err=%v", info.Path, err))
		}
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
		info.HasDedicatedIcon = dedicated && err == nil
		if err != nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("launchpad icon classification failed: path=%s err=%v", source, err))
		}
	} else {
		// Only shell-provided artwork without a file source bypasses extraction.
		// AppsFolder .msc entries have real sources and must pass the check above.
		info.HasDedicatedIcon = isAppsFolderIndexedApp(*info)
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
