package raycast

import (
	"strings"

	"github.com/google/uuid"

	"wox/common"
	"wox/plugin"
)

// raycastAppNamespace is the UUID v5 namespace Raycast uses for Windows application ids.
var raycastAppNamespace = uuid.MustParse("e972d3fc-244a-4623-8e22-ec5fd0188c32")

// desktopAppsForTest replaces the live app index. Tests have no settings database.
var desktopAppsForTest func() []plugin.DesktopApp

// settingsAppsForTest replaces the live Control Panel index.
var settingsAppsForTest func() []plugin.DesktopApp

// raycastShortcutID is the id Raycast stores for a desktop shortcut.
// A shortcut with arguments is "shortcut:" plus the lowercased "executable|arguments".
func raycastShortcutID(executable, args string) string {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return ""
	}
	body := executable
	if args = strings.TrimSpace(args); args != "" {
		body += "|" + args
	}
	return uuid.NewSHA1(raycastAppNamespace, []byte("shortcut:"+strings.ToLower(body))).String()
}

// raycastAppUUID reports the UUID embedded in a Windows Raycast application path.
func raycastAppUUID(path string) (string, bool) {
	const prefix = "/Applications/"
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	id := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parsed, err := uuid.Parse(id)
	if err != nil {
		return "", false
	}
	return parsed.String(), true
}

func raycastAppIndex() map[string]plugin.DesktopApp {
	apps := desktopApplications()
	if len(apps) == 0 {
		return nil
	}
	index := make(map[string]plugin.DesktopApp, len(apps))
	for _, app := range apps {
		id := raycastShortcutID(app.Executable, app.Arguments)
		if id == "" {
			continue
		}
		if _, exists := index[id]; exists {
			continue
		}
		index[id] = app
	}
	return index
}

func desktopApplications() []plugin.DesktopApp {
	if desktopAppsForTest != nil {
		return desktopAppsForTest()
	}
	if !pluginsQueryable() {
		return nil
	}
	manager := plugin.GetPluginManager()
	if manager == nil {
		return nil
	}
	instance := manager.GetPluginInstanceById(common.AppPluginID)
	if instance == nil || instance.Plugin == nil {
		return nil
	}
	catalog, ok := instance.Plugin.(plugin.DesktopAppCatalog)
	if !ok {
		return nil
	}
	return catalog.DesktopApps()
}

// settingsApplications lists indexed Control Panel items from the running app plugin.
func settingsApplications() []plugin.DesktopApp {
	if settingsAppsForTest != nil {
		return settingsAppsForTest()
	}
	if !pluginsQueryable() {
		return nil
	}
	manager := plugin.GetPluginManager()
	if manager == nil {
		return nil
	}
	instance := manager.GetPluginInstanceById(common.AppPluginID)
	if instance == nil || instance.Plugin == nil {
		logRaycastImport("raycast import: app plugin instance is not loaded")
		return nil
	}
	catalog, ok := instance.Plugin.(plugin.SettingsAppCatalog)
	if !ok {
		logRaycastImport("raycast import: app plugin does not expose control panel apps")
		return nil
	}
	return catalog.SettingsApps()
}

func matchIndexedRaycastApp(path string, index map[string]plugin.DesktopApp) (plugin.DesktopApp, bool) {
	id, ok := raycastAppUUID(path)
	if !ok || len(index) == 0 {
		return plugin.DesktopApp{}, false
	}
	app, ok := index[id]
	return app, ok
}
