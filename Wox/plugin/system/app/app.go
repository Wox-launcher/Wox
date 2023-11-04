package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
	"wox/plugin"
	"wox/plugin/system"
	"wox/setting"
	"wox/util"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ApplicationPlugin{})
}

type ApplicationPlugin struct {
	api  plugin.API
	apps []appInfo

	retriever Retriever
}

func (a *ApplicationPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "ea2b6859-14bc-4c89-9c88-627da7379141",
		Name:          "App",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Search app installed on your computer",
		Icon:          "",
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Settings: []setting.PluginSettingItem{
			{
				Type: setting.PluginSettingTypeCheckBox,
				Value: setting.PluginSettingValueCheckBox{
					Key:   "UsePinYin",
					Label: "Use pinyin to search",
					Value: "false",
				},
			},
		},
	}
}

func (a *ApplicationPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	a.api = initParams.API
	a.retriever = a.getRetriever(ctx)

	appCache, cacheErr := a.loadAppCache(ctx)
	if cacheErr == nil {
		a.apps = appCache
	}

	util.Go(ctx, "index apps", func() {
		a.indexApps(util.NewTraceContext())
	})
	util.Go(ctx, "watch app changes", func() {
		a.watchAppChanges(util.NewTraceContext())
	})
}

func (a *ApplicationPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	for _, infoShadow := range a.apps {
		// action will be executed in another go routine, so we need to copy the variable
		info := infoShadow
		if isMatch, score := system.IsStringMatchScore(ctx, info.Name, query.Search); isMatch {
			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    info.Name,
				SubTitle: info.Path,
				Icon:     info.Icon,
				Score:    score,
				Preview: plugin.WoxPreview{
					PreviewType: plugin.WoxPreviewTypeText,
					PreviewData: info.Path,
					PreviewProperties: map[string]string{
						"Path": info.Path,
					},
				},
				Actions: []plugin.QueryResultAction{
					{
						Name: "i18n:plugin_app_open",
						Action: func(actionContext plugin.ActionContext) {
							runErr := exec.Command("open", info.Path).Run()
							if runErr != nil {
								a.api.Log(ctx, fmt.Sprintf("error openning app %s: %s", info.Path, runErr.Error()))
							}
						},
					},
					{
						Name: "i18n:plugin_app_open_containing_folder",
						Action: func(actionContext plugin.ActionContext) {
							runErr := util.ShellOpen(path.Dir(info.Path))
							if runErr != nil {
								a.api.Log(ctx, fmt.Sprintf("error openning app %s: %s", info.Path, runErr.Error()))
							}
						},
					},
					{
						Name: "i18n:plugin_app_copy_path",
						Action: func(actionContext plugin.ActionContext) {
							util.ClipboardWriteText(info.Path)
						},
					},
				},
			})
		}
	}

	return results
}

func (a *ApplicationPlugin) getRetriever(ctx context.Context) Retriever {
	if util.IsMacOS() {
		return &MacRetriever{api: a.api}
	}
	if util.IsWindows() {
		return &WindowsRetriever{api: a.api}
	}

	return nil
}

func (a *ApplicationPlugin) watchAppChanges(ctx context.Context) {
	var appDirectories = a.retriever.GetAppDirectories(ctx)
	var appExtensions = a.retriever.GetAppExtensions(ctx)
	for _, d := range appDirectories {
		var directory = d
		util.WatchDirectories(ctx, directory, func(e fsnotify.Event) {
			var appPath = e.Name
			var isExtensionMatch = lo.ContainsBy(appExtensions, func(ext string) bool {
				return strings.HasSuffix(e.Name, fmt.Sprintf(".%s", ext))
			})
			if !isExtensionMatch {
				return
			}

			a.api.Log(ctx, fmt.Sprintf("app %s changed (%s)", appPath, e.Op))
			if e.Op == fsnotify.Remove || e.Op == fsnotify.Rename {
				for i, app := range a.apps {
					if app.Path == appPath {
						a.apps = append(a.apps[:i], a.apps[i+1:]...)
						a.api.Log(ctx, fmt.Sprintf("app %s removed", appPath))
						a.saveAppToCache(ctx)
						break
					}
				}
			} else if e.Op == fsnotify.Create {
				//check if already exist
				for _, app := range a.apps {
					if app.Path == e.Name {
						return
					}
				}

				//wait for file copy complete
				time.Sleep(time.Second * 2)

				info, getErr := a.retriever.ParseAppInfo(ctx, appPath)
				if getErr != nil {
					a.api.Log(ctx, fmt.Sprintf("error getting app info for %s: %s", e.Name, getErr.Error()))
					return
				}

				a.api.Log(ctx, fmt.Sprintf("app %s added", e.Name))
				a.apps = append(a.apps, info)
				a.saveAppToCache(ctx)
			}
		})
	}
}

func (a *ApplicationPlugin) indexApps(ctx context.Context) {
	startTimestamp := util.GetSystemTimestamp()
	a.api.Log(ctx, "start to get apps")

	var appInfos []appInfo
	for _, appPath := range a.getAppPaths(ctx) {
		info, getErr := a.retriever.ParseAppInfo(ctx, appPath)
		if getErr != nil {
			a.api.Log(ctx, fmt.Sprintf("error getting app info for %s: %s", appPath, getErr.Error()))
			continue
		}
		appInfos = append(appInfos, info)
	}

	if len(appInfos) > 0 {
		a.apps = appInfos
		a.saveAppToCache(ctx)
	}

	a.api.Log(ctx, fmt.Sprintf("indexed %d apps, cost %d ms", len(a.apps), util.GetSystemTimestamp()-startTimestamp))
}

func (a *ApplicationPlugin) getAppPaths(ctx context.Context) (appPaths []string) {
	var appDirectories = a.retriever.GetAppDirectories(ctx)
	var appExtensions = a.retriever.GetAppExtensions(ctx)
	for _, appDirectory := range appDirectories {
		appPath, readErr := os.ReadDir(appDirectory)
		if readErr != nil {
			a.api.Log(ctx, fmt.Sprintf("error reading directory %s: %s", appDirectory, readErr.Error()))
			continue
		}

		for _, entry := range appPath {
			isExtensionMatch := lo.ContainsBy(appExtensions, func(ext string) bool {
				return strings.HasSuffix(entry.Name(), fmt.Sprintf(".%s", ext))
			})
			if isExtensionMatch {
				appPaths = append(appPaths, path.Join(appDirectory, entry.Name()))
				continue
			}

			// check if it's a directory
			subDir := path.Join(appDirectory, entry.Name())
			isDirectory, dirErr := util.IsDirectory(subDir)
			if dirErr != nil || !isDirectory {
				continue
			}

			appSubDir, readSubDirErr := os.ReadDir(subDir)
			if readSubDirErr != nil {
				a.api.Log(ctx, fmt.Sprintf("error reading sub directory %s: %s", appDirectory, readSubDirErr.Error()))
				continue
			}

			for _, subEntry := range appSubDir {
				isExtensionMatch = lo.ContainsBy(appExtensions, func(ext string) bool {
					return strings.HasSuffix(subEntry.Name(), fmt.Sprintf(".%s", ext))
				})
				if isExtensionMatch {
					appPaths = append(appPaths, path.Join(appDirectory, entry.Name(), subEntry.Name()))
					continue
				}
			}
		}
	}

	return
}

func (a *ApplicationPlugin) saveAppToCache(ctx context.Context) {
	if len(a.apps) == 0 {
		return
	}

	var cachePath = a.getAppCachePath()
	cacheContent, marshalErr := json.Marshal(a.apps)
	if marshalErr != nil {
		a.api.Log(ctx, fmt.Sprintf("error marshalling app cache: %s", marshalErr.Error()))
		return
	}
	writeErr := os.WriteFile(cachePath, cacheContent, 0644)
	if writeErr != nil {
		a.api.Log(ctx, fmt.Sprintf("error writing app cache: %s", writeErr.Error()))
		return
	}
	a.api.Log(ctx, fmt.Sprintf("wrote app cache to %s", cachePath))
}

func (a *ApplicationPlugin) getAppCachePath() string {
	return path.Join(os.TempDir(), "wox-app-cache.json")
}

func (a *ApplicationPlugin) loadAppCache(ctx context.Context) ([]appInfo, error) {
	startTimestamp := util.GetSystemTimestamp()
	a.api.Log(ctx, "start to load app cache")
	var cachePath = a.getAppCachePath()
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		a.api.Log(ctx, "app cache file not found")
		return nil, err
	}

	cacheContent, readErr := os.ReadFile(cachePath)
	if readErr != nil {
		a.api.Log(ctx, fmt.Sprintf("error reading app cache file: %s", readErr.Error()))
		return nil, readErr
	}

	var apps []appInfo
	unmarshalErr := json.Unmarshal(cacheContent, &apps)
	if unmarshalErr != nil {
		a.api.Log(ctx, fmt.Sprintf("error unmarshalling app cache file: %s", unmarshalErr.Error()))
		return nil, unmarshalErr
	}

	a.api.Log(ctx, fmt.Sprintf("loaded %d apps from cache, cost %d ms", len(apps), util.GetSystemTimestamp()-startTimestamp))
	return apps, nil
}
