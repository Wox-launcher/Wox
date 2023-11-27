package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/tidwall/pretty"
	"os"
	"path"
	"strings"
	"sync"
	"time"
	"wox/plugin"
	"wox/plugin/system"
	"wox/setting"
	"wox/util"
	"wox/util/clipboard"
)

var appIcon = plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48"><path fill="#0091ea" d="M14.1,42h19.8c4.474,0,8.1-3.627,8.1-8.1V27H6v6.9C6,38.373,9.626,42,14.1,42z"></path><rect width="36" height="11" x="6" y="16" fill="#00b0ff"></rect><path fill="#40c4ff" d="M33.9,6H14.1C9.626,6,6,9.626,6,14.1V16h36v-1.9C42,9.626,38.374,6,33.9,6z"></path><path fill="#fff" d="M22.854,18.943l1.738-2.967l-1.598-2.727c-0.418-0.715-1.337-0.954-2.052-0.536	c-0.715,0.418-0.955,1.337-0.536,2.052L22.854,18.943z"></path><path fill="#fff" d="M26.786,12.714c-0.716-0.419-1.635-0.179-2.052,0.536L16.09,28h3.477l7.754-13.233	C27.74,14.052,27.5,13.133,26.786,12.714z"></path><path fill="#fff" d="M34.521,32.92l-7.611-12.987l-0.763,1.303c-0.444,0.95-0.504,2.024-0.185,3.011l5.972,10.191	c0.279,0.476,0.78,0.741,1.295,0.741c0.257,0,0.519-0.066,0.757-0.206C34.701,34.554,34.94,33.635,34.521,32.92z"></path><path fill="#fff" d="M25.473,27.919l-0.171-0.289c-0.148-0.224-0.312-0.434-0.498-0.621H12.3	c-0.829,0-1.5,0.665-1.5,1.484s0.671,1.484,1.5,1.484h13.394C25.888,29.324,25.835,28.595,25.473,27.919z"></path><path fill="#fff" d="M16.66,32.961c-0.487-0.556-1.19-0.934-2.03-0.959l-0.004,0c-0.317-0.009-0.628,0.026-0.932,0.087	l-0.487,0.831c-0.419,0.715-0.179,1.634,0.536,2.053c0.238,0.14,0.5,0.206,0.757,0.206c0.515,0,1.017-0.266,1.295-0.741	L16.66,32.961z"></path><path fill="#fff" d="M30.196,27.009H35.7c0.829,0,1.5,0.665,1.5,1.484s-0.671,1.484-1.5,1.484h-5.394	C30.112,29.324,30.01,27.196,30.196,27.009z"></path></svg>`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ApplicationPlugin{})
}

type ApplicationPlugin struct {
	api             plugin.API
	pluginDirectory string

	apps      []appInfo
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
		Icon:          appIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		SettingDefinitions: []setting.PluginSettingDefinitionItem{
			{
				Type: setting.PluginSettingDefinitionTypeCheckBox,
				Value: &setting.PluginSettingValueCheckBox{
					Key:          "UsePinYin",
					Label:        "Use pinyin to search",
					DefaultValue: "false",
				},
			},
		},
	}
}

func (a *ApplicationPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	a.api = initParams.API
	a.pluginDirectory = initParams.PluginDirectory
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
						Icon: plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="64" height="64" viewBox="0 0 32 32"><polygon fill="#0f518c" points="30,30 2,30 2,2 17,2 17,6 6,6 6,26 26,26 26,15 30,15"></polygon><polygon fill="#ed0049" points="19,2 19,6 23.172,6 14.586,14.586 17.414,17.414 26,8.828 26,13 30,13 30,2"></polygon></svg>`),
						Action: func(actionContext plugin.ActionContext) {
							runErr := util.ShellOpen(info.Path)
							if runErr != nil {
								a.api.Log(ctx, fmt.Sprintf("error openning app %s: %s", info.Path, runErr.Error()))
							}
						},
					},
					{
						Name: "i18n:plugin_app_open_containing_folder",
						Icon: plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48"><path fill="#FFA000" d="M40,12H22l-4-4H8c-2.2,0-4,1.8-4,4v8h40v-4C44,13.8,42.2,12,40,12z"></path><path fill="#FFCA28" d="M40,12H8c-2.2,0-4,1.8-4,4v20c0,2.2,1.8,4,4,4h32c2.2,0,4-1.8,4-4V16C44,13.8,42.2,12,40,12z"></path></svg>`),
						Action: func(actionContext plugin.ActionContext) {
							runErr := util.ShellOpen(path.Dir(info.Path))
							if runErr != nil {
								a.api.Log(ctx, fmt.Sprintf("error openning app %s: %s", info.Path, runErr.Error()))
							}
						},
					},
					{
						Name: "i18n:plugin_app_copy_path",
						Icon: plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAAA+0lEQVR4nO3VLQ7CQBAF4L3Eih6HOQScAIfmDMhq0NVgcegaCAq7GoMpBJKyBEc2/GyZaafTzkue36/7sjVGo9FoICs8toOsmLF9Sezhh8szLwIL2B5LP1oxIrAAV9x5ERQAR41I86vHdrK+VAI4SgT28PPdLRrhXgBkCCzgcCr9IhLhAgAJAgt4HiIW4d4AQgQLoAoCfpQNQIUwnAAKhOEGYBGmDYAQAW0GxNSONx+rgLSBGwDpEwLpAPtl87UAkun+73YSANInBNIBtun/AHYynQOA9AmBdICV/oxa6QCQPiFQQN+e0UQ6ICWuqTMKyPUGej4hjUZjROQBwgDUDcPYwFwAAAAASUVORK5CYII=`),
						Action: func(actionContext plugin.ActionContext) {
							clipboard.WriteText(info.Path)
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

	appPaths := a.getAppPaths(ctx)

	// split into groups, so we can index apps in parallel
	var appPathGroups [][]string
	var groupSize = 25
	for i := 0; i < len(appPaths); i += groupSize {
		var end = i + groupSize
		if end > len(appPaths) {
			end = len(appPaths)
		}
		appPathGroups = append(appPathGroups, appPaths[i:end])
	}
	a.api.Log(ctx, fmt.Sprintf("found %d apps in %d groups", len(appPaths), len(appPathGroups)))

	var appInfos []appInfo
	var waitGroup sync.WaitGroup
	var lock sync.Mutex
	waitGroup.Add(len(appPathGroups))
	for groupIndex := range appPathGroups {
		var appPathGroup = appPathGroups[groupIndex]
		util.Go(ctx, fmt.Sprintf("index app group: %d", groupIndex), func() {
			for _, appPath := range appPathGroup {
				info, getErr := a.retriever.ParseAppInfo(ctx, appPath)
				if getErr != nil {
					a.api.Log(ctx, fmt.Sprintf("error getting app info for %s: %s", appPath, getErr.Error()))
					continue
				}

				//preprocess icon
				info.Icon = plugin.ConvertIcon(ctx, info.Icon, a.pluginDirectory)

				lock.Lock()
				appInfos = append(appInfos, info)
				lock.Unlock()
			}
			waitGroup.Done()
		}, func() {
			waitGroup.Done()
		})
	}

	waitGroup.Wait()

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
	writeErr := os.WriteFile(cachePath, pretty.Pretty(cacheContent), 0644)
	if writeErr != nil {
		a.api.Log(ctx, fmt.Sprintf("error writing app cache: %s", writeErr.Error()))
		return
	}
	a.api.Log(ctx, fmt.Sprintf("wrote app cache to %s", cachePath))
}

func (a *ApplicationPlugin) getAppCachePath() string {
	return path.Join(util.GetLocation().GetCacheDirectory(), "wox-app-cache.json")
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
