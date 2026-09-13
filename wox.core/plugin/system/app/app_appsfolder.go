package app

import "strings"

type appsFolderEntry struct {
	Name     string
	AppID    string
	Packaged bool
}

// Windows Known Folder IDs used by AppsFolder for inbox tools that are not
// ordinary Start Menu shortcuts. Program Files GUIDs are omitted because the
// directory scanner already indexes those executables.
const (
	appsFolderSystem32GUID = "{1ac14e77-02e7-4e5d-b744-2eb1ae5198b7}\\"
	appsFolderSysWOW64GUID = "{d65231b0-b2f1-4857-a4ce-a8e7c6ea7d27}\\"
	appsFolderWindowsGUID  = "{f38bf404-1d43-42f2-9305-67de0b28fc23}\\"
	appsFolderWindowsAUMID = "microsoft.windows."
)

// shouldIndexAppsFolderAppID keeps packaged Store/UWP apps, browser-installed
// web apps, and Windows inbox tools. Ordinary desktop programs stay out so the
// directory scanner does not produce duplicates.
func shouldIndexAppsFolderAppID(appID string) bool {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return false
	}
	return isPackagedAppsFolderAppID(appID) || isAppsFolderWebAppID(appID) || isWindowsInboxAppsFolderAppID(appID)
}

func isPackagedAppsFolderAppID(appID string) bool {
	return strings.Contains(appID, "!")
}

func isAppsFolderWebAppID(appID string) bool {
	lower := strings.ToLower(strings.TrimSpace(appID))
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return true
	}
	return strings.Contains(lower, "msedgepwa") || strings.Contains(lower, "_crx_")
}

func isAppsFolderIndexedApp(info appInfo) bool {
	return strings.HasPrefix(info.Path, "shell:AppsFolder\\")
}

func extraAppPath(appID string) string {
	return "shell:AppsFolder\\" + appID
}

func appsFolderEntryType(entry appsFolderEntry) AppType {
	if entry.Packaged {
		return AppTypeUWP
	}
	return AppTypeAppsFolder
}

// extraAppsFromFolderEntries reuses cached AppsFolder entries and only creates
// new ones for IDs that just appeared. Incremental reconcile must stay cheap
// because Edge PWAs never write a Start Menu shortcut for the file watcher.
func extraAppsFromFolderEntries(entries []appsFolderEntry, existingByPath map[string]appInfo, pathKey func(string) string) []appInfo {
	apps := make([]appInfo, 0, len(entries))
	for _, entry := range entries {
		if !shouldIndexAppsFolderAppID(entry.AppID) {
			continue
		}
		path := extraAppPath(entry.AppID)
		if current, ok := existingByPath[pathKey(path)]; ok {
			if entry.Name != "" && current.Name != entry.Name {
				current.Name = entry.Name
			}
			apps = append(apps, current)
			continue
		}
		apps = append(apps, appInfo{
			Name:          entry.Name,
			Path:          path,
			Icon:          appIcon,
			Type:          appsFolderEntryType(entry),
			IsDefaultIcon: true,
		})
	}
	return apps
}

func isWindowsInboxAppsFolderAppID(appID string) bool {
	lower := strings.ToLower(strings.TrimSpace(appID))
	if strings.HasPrefix(lower, appsFolderWindowsAUMID) && !strings.Contains(lower, "!") {
		return true
	}
	return strings.HasPrefix(lower, appsFolderSystem32GUID) ||
		strings.HasPrefix(lower, appsFolderSysWOW64GUID) ||
		strings.HasPrefix(lower, appsFolderWindowsGUID)
}
