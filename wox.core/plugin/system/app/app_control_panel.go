package app

import "strings"

// Control Panel items use the All Control Panel Items parsing name. Searchable
// tasks live under the All Tasks namespace inside that same root.
const (
	controlPanelRootParsingName = `::{26EE0668-A00A-44D7-9371-BEB064C98683}`
	controlPanelAllTasksGUID    = `{ED7BA470-8E54-465E-825C-99712043E01C}`
)

type controlPanelEntry struct {
	Name string
	Path string
}

// isControlPanelAppPath reports Shell parsing names that open a Control Panel applet or task.
func isControlPanelAppPath(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	if lower == "" {
		return false
	}
	if strings.HasPrefix(lower, strings.ToLower(controlPanelRootParsingName)) {
		return true
	}
	return strings.Contains(lower, strings.ToLower(controlPanelAllTasksGUID))
}

func isControlPanelAppletPath(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	prefix := strings.ToLower(controlPanelRootParsingName + `\0\::{`)
	return strings.HasPrefix(lower, prefix) && !strings.Contains(lower, strings.ToLower(controlPanelAllTasksGUID))
}

func isControlPanelTaskPath(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.Contains(lower, strings.ToLower(controlPanelAllTasksGUID+`\{`))
}

// windowsResultSubtitleKey separates classic Control Panel rows from modern Settings pages.
func windowsResultSubtitleKey(info appInfo) string {
	if isControlPanelAppPath(info.Path) {
		return "i18n:plugin_app_windows_control_panel_subtitle"
	}
	if info.Type == AppTypeWindowsSetting {
		return "i18n:plugin_app_windows_settings_subtitle"
	}
	return ""
}

// mergeControlPanelEntries keeps applets and tasks that can be launched.
// A task whose visible name matches an applet opens the same page, so the applet wins.
func mergeControlPanelEntries(applets []controlPanelEntry, tasks []controlPanelEntry) []controlPanelEntry {
	merged := make([]controlPanelEntry, 0, len(applets)+len(tasks))
	seenPath := make(map[string]struct{}, len(applets)+len(tasks))
	appletNames := make(map[string]struct{}, len(applets))

	appendUnique := func(entries []controlPanelEntry, dropAppletNames bool) {
		for _, entry := range entries {
			name := strings.TrimSpace(entry.Name)
			path := strings.TrimSpace(entry.Path)
			if name == "" || !isControlPanelAppPath(path) {
				continue
			}
			if dropAppletNames && !isControlPanelTaskPath(path) {
				continue
			}
			if !dropAppletNames && !isControlPanelAppletPath(path) {
				continue
			}
			pathKey := strings.ToLower(path)
			if _, ok := seenPath[pathKey]; ok {
				continue
			}
			nameKey := strings.ToLower(name)
			if dropAppletNames {
				if _, ok := appletNames[nameKey]; ok {
					continue
				}
			}
			seenPath[pathKey] = struct{}{}
			if !dropAppletNames {
				appletNames[nameKey] = struct{}{}
			}
			merged = append(merged, controlPanelEntry{Name: name, Path: path})
		}
	}

	appendUnique(applets, false)
	appendUnique(tasks, true)
	return merged
}
