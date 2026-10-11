package app

import (
	"context"
	"fmt"
	"time"

	"wox/util"
)

// refreshUserAppDirectories updates custom roots without enumerating system applications again.
func (a *ApplicationPlugin) refreshUserAppDirectories(ctx context.Context) {
	a.appIndexMu.Lock()
	defer a.appIndexMu.Unlock()
	a.stopAppChangeWatcher()
	if ctx.Err() != nil {
		return
	}
	defer a.startAppChangeWatcher()
	started := time.Now()
	directories := a.getUserAddedPaths(ctx)
	paths := a.deduplicateAppPaths(a.getAppPaths(ctx, directories))
	apps := a.indexAppsByPaths(ctx, paths, nil)
	if ctx.Err() != nil {
		return
	}
	a.apps = a.mergeUserDirectoryApps(apps, directories, a.retriever.GetAppDirectories(ctx))
	a.indexedUserDirectories = directories
	a.rebuildHotkeyAppCandidates(ctx)
	a.rebuildQueryEntries(ctx)
	a.saveAppToCache(ctx)
	util.GetLogger().Info(ctx, fmt.Sprintf("app directories updated: roots=%d apps=%d cost=%d ms", len(directories), len(apps), time.Since(started).Milliseconds()))
}

// mergeUserDirectoryApps replaces custom-root entries while preserving other discovery sources.
func (a *ApplicationPlugin) mergeUserDirectoryApps(apps []appInfo, directories, defaultDirectories []appDirectory) []appInfo {
	updatedPaths := make(map[string]bool, len(apps))
	for _, app := range apps {
		updatedPaths[a.pathCacheKey(app.Path)] = true
	}
	scopes := append(append([]appDirectory(nil), a.indexedUserDirectories...), directories...)
	merged := make([]appInfo, 0, len(a.apps)+len(apps))
	for _, app := range a.apps {
		key := a.pathCacheKey(app.Path)
		if updatedPaths[key] {
			continue
		}
		inUserScope := false
		for _, directory := range scopes {
			inUserScope = inUserScope || a.isAppPathInDirectoryScope(app.Path, directory)
		}
		preserve := !inUserScope || a.extraAppPathKeys[key] || isAppsFolderIndexedApp(app) || app.Type == AppTypeWindowsSetting
		for _, directory := range defaultDirectories {
			preserve = preserve || a.isAppPathInDirectoryScope(app.Path, directory)
		}
		if preserve {
			merged = append(merged, app)
		}
	}
	return append(merged, apps...)
}
