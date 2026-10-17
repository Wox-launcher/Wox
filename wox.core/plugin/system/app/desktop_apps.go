package app

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"wox/plugin"
)

var _ plugin.DesktopAppCatalog = (*ApplicationPlugin)(nil)

// DesktopApps lists indexed desktop applications so an importer can match them
// by the executable a shortcut actually launches.
func (a *ApplicationPlugin) DesktopApps() []plugin.DesktopApp {
	if a == nil {
		return nil
	}
	a.appIndexMu.Lock()
	apps := append([]appInfo(nil), a.apps...)
	a.appIndexMu.Unlock()

	rows := make([]plugin.DesktopApp, 0, len(apps))
	for _, info := range apps {
		if info.Type != "" && info.Type != AppTypeDesktop {
			continue
		}
		executable, args, ok := desktopLaunchTarget(info)
		if !ok {
			continue
		}
		name := strings.TrimSpace(info.Name)
		if name == "" {
			name = strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))
		}
		path := strings.TrimSpace(info.Path)
		if path == "" {
			path = executable
		}
		rows = append(rows, plugin.DesktopApp{
			Name: name, Path: path, Executable: executable, Arguments: args,
		})
	}
	return rows
}

// desktopLaunchTarget returns the executable and arguments used for a default open.
// The launch key is the JSON form of the shell request written while indexing.
func desktopLaunchTarget(info appInfo) (executable, args string, ok bool) {
	if file, params, found := launchKeyTarget(info.launchKey); found {
		return file, params, true
	}
	for _, candidate := range []string{info.shortcutTarget, info.IconSourcePath, info.Path} {
		if executablePath(candidate) {
			return strings.TrimSpace(candidate), "", true
		}
	}
	return "", "", false
}

func launchKeyTarget(key string) (file, args string, ok bool) {
	key = strings.TrimSpace(key)
	if key == "" || !strings.HasPrefix(key, "{") {
		return "", "", false
	}
	var request struct {
		File       string
		Parameters string
	}
	if err := json.Unmarshal([]byte(key), &request); err != nil {
		return "", "", false
	}
	if !executablePath(request.File) {
		return "", "", false
	}
	return strings.TrimSpace(request.File), strings.TrimSpace(request.Parameters), true
}

func executablePath(path string) bool {
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(path)), ".exe")
}
