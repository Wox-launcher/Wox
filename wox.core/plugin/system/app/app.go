package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system"
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/shell"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/tidwall/pretty"
)

var appIcon = plugin.PluginAppIcon

type AppType = string

const (
	AppTypeDesktop AppType = "desktop"
	AppTypeUWP     AppType = "uwp"
)

type appInfo struct {
	Name string
	Path string
	Icon common.WoxImage
	Type AppType

	LastModifiedUnix int64 `json:"last_modified_unix,omitempty"`

	Pid int `json:"-"`
}

type appContextData struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

func (a *appInfo) GetDisplayPath() string {
	if a.Type == AppTypeUWP {
		return ""
	}
	return a.Path
}

func (a *appInfo) IsRunning() bool {
	return a.Pid > 0
}

type appDirectory struct {
	Path              string
	Recursive         bool
	RecursiveDepth    int
	RecursiveExcludes []string
	excludeAbsPaths   []string // internal: absolute paths of excluded directories
}

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
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
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
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureMRU,
			},
		},
		SettingDefinitions: []definition.PluginSettingDefinitionItem{
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key: "AppDirectories",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "Path",
							Label: "Path",
							Type:  definition.PluginSettingValueTableColumnTypeDirPath,
						},
					},
				},
			},
		},
	}
}

func (a *ApplicationPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	a.api = initParams.API
	a.pluginDirectory = initParams.PluginDirectory
	a.retriever = a.getRetriever(ctx)
	a.retriever.UpdateAPI(a.api)

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
	util.Go(ctx, "update app process", func() {
		for range time.NewTicker(time.Second * 3).C {
			for i := range a.apps {
				a.apps[i].Pid = a.retriever.GetPid(ctx, a.apps[i])
			}
		}
	})

	a.api.OnSettingChanged(ctx, func(key string, value string) {
		if key == "AppDirectories" {
			a.indexApps(ctx)
		}
	})

	a.api.OnMRURestore(ctx, a.handleMRURestore)
}

func (a *ApplicationPlugin) pathCacheKey(appPath string) string {
	if a.retriever.GetPlatform() == util.PlatformWindows {
		// Windows paths are case-insensitive; normalize to avoid duplicate cache entries.
		return strings.ToLower(appPath)
	}
	return appPath
}

func (a *ApplicationPlugin) populateAppMetadata(appPath string, info *appInfo, fileInfo os.FileInfo) {
	if fileInfo == nil {
		stat, err := os.Stat(appPath)
		if err == nil {
			fileInfo = stat
		} else {
			info.LastModifiedUnix = 0
			info.Pid = 0
			return
		}
	}

	info.LastModifiedUnix = fileInfo.ModTime().UnixNano()
	info.Pid = 0
}

func (a *ApplicationPlugin) reuseAppFromCache(ctx context.Context, appPath string, fileInfo os.FileInfo, cache map[string]appInfo) (appInfo, bool) {
	key := a.pathCacheKey(appPath)
	cached, ok := cache[key]
	if !ok {
		return appInfo{}, false
	}

	if cached.LastModifiedUnix == 0 || fileInfo == nil {
		return appInfo{}, false
	}

	if cached.LastModifiedUnix != fileInfo.ModTime().UnixNano() {
		return appInfo{}, false
	}

	if cached.Icon.ImageType == common.WoxImageTypeAbsolutePath && cached.Icon.ImageData != "" {
		if _, err := os.Stat(cached.Icon.ImageData); err != nil {
			a.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("cached icon missing for %s, reindexing", appPath))
			return appInfo{}, false
		}
	}

	cached.Pid = 0
	return cached, true
}

func (a *ApplicationPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	for _, info := range a.apps {
		isNameMatch, nameScore := system.IsStringMatchScore(ctx, info.Name, query.Search)
		isPathNameMatch, pathNameScore := system.IsStringMatchScore(ctx, filepath.Base(info.Path), query.Search)
		if isNameMatch || isPathNameMatch {
			displayPath := info.GetDisplayPath()

			contextData := appContextData{
				Name: info.Name,
				Path: info.Path,
				Type: info.Type,
			}
			contextDataJson, _ := json.Marshal(contextData)

			result := plugin.QueryResult{
				Id:          uuid.NewString(),
				Title:       info.Name,
				SubTitle:    displayPath,
				Icon:        info.Icon,
				Score:       util.MaxInt64(nameScore, pathNameScore),
				ContextData: string(contextDataJson),
				Actions: []plugin.QueryResultAction{
					{
						Name: "i18n:plugin_app_open",
						Icon: plugin.OpenIcon,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							runErr := shell.Open(info.Path)
							if runErr != nil {
								a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error opening app %s: %s", info.Path, runErr.Error()))
								a.api.Notify(ctx, fmt.Sprintf("i18n:plugin_app_open_failed_description: %s", runErr.Error()))
							}
						},
					},
					{
						Name: "i18n:plugin_app_open_containing_folder",
						Icon: plugin.OpenContainingFolderIcon,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							if err := a.retriever.OpenAppFolder(ctx, info); err != nil {
								a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error opening folder: %s", err.Error()))
							}
						},
					},
					{
						Name: "i18n:plugin_app_copy_path",
						Icon: plugin.CopyIcon,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							clipboard.WriteText(info.Path)
						},
					},
				},
			}

			if info.IsRunning() {
				result.Actions = append(result.Actions, plugin.QueryResultAction{
					Name: "i18n:plugin_app_terminate",
					Icon: plugin.TerminateAppIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						// peacefully kill the process
						p, getErr := os.FindProcess(info.Pid)
						if getErr != nil {
							a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error finding process %d: %s", info.Pid, getErr.Error()))
							return
						}

						killErr := p.Kill()
						if killErr != nil {
							a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error killing process %d: %s", info.Pid, killErr.Error()))
						}
					},
				})

				// refresh cpu and mem
				result.RefreshInterval = 1000
				result.OnRefresh = func(ctx context.Context, result plugin.RefreshableResult) plugin.RefreshableResult {
					result.Tails = a.getRunningProcessResult(info)
					return result
				}
			}

			results = append(results, result)
		}
	}

	return results
}

func (a *ApplicationPlugin) getRunningProcessResult(app appInfo) (tails []plugin.QueryResultTail) {
	ctx := context.Background()
	stat, err := a.retriever.GetProcessStat(ctx, app)
	if err != nil {
		a.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("error getting process stat for %s: %s", app.Name, err.Error()))
		return
	}

	// Show CPU usage
	cpuLabel := a.api.GetTranslation(ctx, "i18n:plugin_app_cpu")
	tails = append(tails, plugin.QueryResultTail{
		Type: plugin.QueryResultTailTypeText,
		Text: fmt.Sprintf("%s: %.1f%%", cpuLabel, stat.CPU),
	})

	// Format memory size
	memSize := stat.Memory
	unit := "B"
	if memSize > 1024*1024*1024 {
		memSize = memSize / 1024 / 1024 / 1024
		unit = "GB"
	} else if memSize > 1024*1024 {
		memSize = memSize / 1024 / 1024
		unit = "MB"
	} else if memSize > 1024 {
		memSize = memSize / 1024
		unit = "KB"
	}

	memLabel := a.api.GetTranslation(ctx, "i18n:plugin_app_memory")
	tails = append(tails, plugin.QueryResultTail{
		Type: plugin.QueryResultTailTypeText,
		Text: fmt.Sprintf("%s: %.1f %s", memLabel, memSize, unit),
	})

	return
}

func (a *ApplicationPlugin) getRetriever(ctx context.Context) Retriever {
	return appRetriever
}

func (a *ApplicationPlugin) watchAppChanges(ctx context.Context) {
	var appDirectories = a.getAppDirectories(ctx)
	var appExtensions = a.retriever.GetAppExtensions(ctx)
	for _, d := range appDirectories {
		var directory = d
		util.WatchDirectoryChanges(ctx, directory.Path, func(e fsnotify.Event) {
			var appPath = e.Name
			var isExtensionMatch = lo.ContainsBy(appExtensions, func(ext string) bool {
				return strings.HasSuffix(e.Name, fmt.Sprintf(".%s", ext))
			})
			if !isExtensionMatch {
				return
			}

			a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app %s changed (%s)", appPath, e.Op))
			if e.Op == fsnotify.Remove || e.Op == fsnotify.Rename {
				for i, app := range a.apps {
					if app.Path == appPath {
						a.apps = append(a.apps[:i], a.apps[i+1:]...)
						a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app %s removed", appPath))
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
					a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error getting app info for %s: %s", e.Name, getErr.Error()))
					return
				}

				a.populateAppMetadata(appPath, &info, nil)
				info.Icon = common.ConvertIcon(ctx, info.Icon, a.pluginDirectory)

				a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app %s added", e.Name))
				a.apps = append(a.apps, info)
				a.saveAppToCache(ctx)
			}
		})
	}
}

func (a *ApplicationPlugin) indexApps(ctx context.Context) {
	startTimestamp := util.GetSystemTimestamp()
	a.api.Log(ctx, plugin.LogLevelInfo, "start to get apps")

	appInfos := a.indexAppsByDirectory(ctx)
	extraApps := a.indexExtraApps(ctx)

	//merge extra apps
	for _, extraApp := range extraApps {
		var isExist = false
		for _, app := range appInfos {
			if app.Path == extraApp.Path {
				isExist = true
				break
			}
		}
		if !isExist {
			appInfos = append(appInfos, extraApp)
		}
	}

	// Remove duplicates with same Name and Path
	appInfos = a.removeDuplicateApps(ctx, appInfos)

	a.apps = appInfos
	a.saveAppToCache(ctx)

	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("indexed %d apps, cost %d ms", len(a.apps), util.GetSystemTimestamp()-startTimestamp))
}

func (a *ApplicationPlugin) getUserAddedPaths(ctx context.Context) []appDirectory {
	userAddedPaths := a.api.GetSetting(ctx, "AppDirectories")
	if userAddedPaths == "" {
		return []appDirectory{}
	}

	var appDirectories []appDirectory
	unmarshalErr := json.Unmarshal([]byte(userAddedPaths), &appDirectories)
	if unmarshalErr != nil {
		return []appDirectory{}
	}

	for i := range appDirectories {
		appDirectories[i].Recursive = true
		appDirectories[i].RecursiveDepth = 3
	}

	return appDirectories
}

func (a *ApplicationPlugin) getAppDirectories(ctx context.Context) []appDirectory {
	return append(a.getUserAddedPaths(ctx), a.getRetriever(ctx).GetAppDirectories(ctx)...)
}

func (a *ApplicationPlugin) indexAppsByDirectory(ctx context.Context) []appInfo {
	cacheByPath := make(map[string]appInfo, len(a.apps))
	for _, cached := range a.apps {
		cacheByPath[a.pathCacheKey(cached.Path)] = cached
	}

	appDirectories := a.getAppDirectories(ctx)
	appPaths := a.getAppPaths(ctx, appDirectories)

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
	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("found %d apps in %d groups", len(appPaths), len(appPathGroups)))

	var appInfos []appInfo
	var waitGroup sync.WaitGroup
	var lock sync.Mutex
	var cacheHits int64
	var parsedCount int64
	waitGroup.Add(len(appPathGroups))
	for groupIndex := range appPathGroups {
		var appPathGroup = appPathGroups[groupIndex]
		util.Go(ctx, fmt.Sprintf("index app group: %d", groupIndex), func() {
			for _, appPath := range appPathGroup {
				fileInfo, statErr := os.Stat(appPath)
				if statErr != nil {
					a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error stating %s: %s", appPath, statErr.Error()))
					continue
				}

				if cachedInfo, ok := a.reuseAppFromCache(ctx, appPath, fileInfo, cacheByPath); ok {
					atomic.AddInt64(&cacheHits, 1)
					lock.Lock()
					appInfos = append(appInfos, cachedInfo)
					lock.Unlock()
					continue
				}

				info, getErr := a.retriever.ParseAppInfo(ctx, appPath)
				if getErr != nil {
					a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error getting app info for %s: %s", appPath, getErr.Error()))
					continue
				}

				a.populateAppMetadata(appPath, &info, fileInfo)

				// preprocess icon
				info.Icon = common.ConvertIcon(ctx, info.Icon, a.pluginDirectory)
				atomic.AddInt64(&parsedCount, 1)

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

	totalProcessed := cacheHits + parsedCount
	var cacheRatio float64
	if totalProcessed > 0 {
		cacheRatio = float64(cacheHits) / float64(totalProcessed) * 100
	}
	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf(
		"app indexing stats: total=%d cache_hits=%d parsed=%d cache_ratio=%.2f%%",
		totalProcessed,
		cacheHits,
		parsedCount,
		cacheRatio,
	))

	return appInfos
}

func (a *ApplicationPlugin) indexExtraApps(ctx context.Context) []appInfo {
	apps, err := a.retriever.GetExtraApps(ctx)
	if err != nil {
		return []appInfo{}
	}

	//preprocess icon
	for i := range apps {
		apps[i].Icon = common.ConvertIcon(ctx, apps[i].Icon, a.pluginDirectory)
	}

	return apps
}

func (a *ApplicationPlugin) getAppPaths(ctx context.Context, appDirectories []appDirectory) (appPaths []string) {
	var appExtensions = a.retriever.GetAppExtensions(ctx)
	for _, dir := range appDirectories {
		// Initialize excludeAbsPaths on first call
		if len(dir.RecursiveExcludes) > 0 && len(dir.excludeAbsPaths) == 0 {
			dir.excludeAbsPaths = make([]string, 0, len(dir.RecursiveExcludes))
			for _, exclude := range dir.RecursiveExcludes {
				excludeAbsPath := filepath.Join(dir.Path, exclude)
				cleanExcludePath := filepath.Clean(excludeAbsPath)
				dir.excludeAbsPaths = append(dir.excludeAbsPaths, strings.ToLower(cleanExcludePath))
			}
		}

		appPath, readErr := os.ReadDir(dir.Path)
		if readErr != nil {
			a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error reading directory %s: %s", dir.Path, readErr.Error()))
			continue
		}

		for _, entry := range appPath {
			isExtensionMatch := lo.ContainsBy(appExtensions, func(ext string) bool {
				return strings.HasSuffix(strings.ToLower(entry.Name()), fmt.Sprintf(".%s", ext))
			})
			if isExtensionMatch {
				appPaths = append(appPaths, path.Join(dir.Path, entry.Name()))
				continue
			}

			// check if it's a directory
			subDir := path.Join(dir.Path, entry.Name())
			isDirectory, dirErr := util.IsDirectory(subDir)
			if dirErr != nil || !isDirectory {
				continue
			}

			// Check if this directory should be excluded
			if len(dir.excludeAbsPaths) > 0 {
				cleanSubDir := strings.ToLower(filepath.Clean(subDir))
				isExcluded := lo.ContainsBy(dir.excludeAbsPaths, func(excludePath string) bool {
					// Check if subDir starts with the exclude path (case-insensitive)
					return strings.HasPrefix(cleanSubDir, excludePath)
				})
				if isExcluded {
					a.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("skipping excluded directory: %s", subDir))
					continue
				}
			}

			if dir.Recursive && dir.RecursiveDepth > 0 {
				appPaths = append(appPaths, a.getAppPaths(ctx, []appDirectory{{
					Path:            subDir,
					Recursive:       true,
					RecursiveDepth:  dir.RecursiveDepth - 1,
					excludeAbsPaths: dir.excludeAbsPaths,
				}})...)
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
		a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error marshalling app cache: %s", marshalErr.Error()))
		return
	}
	writeErr := os.WriteFile(cachePath, pretty.Pretty(cacheContent), 0644)
	if writeErr != nil {
		a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error writing app cache: %s", writeErr.Error()))
		return
	}
	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("wrote app cache to %s", cachePath))
}

func (a *ApplicationPlugin) getAppCachePath() string {
	return path.Join(util.GetLocation().GetCacheDirectory(), "wox-app-cache.json")
}

func (a *ApplicationPlugin) loadAppCache(ctx context.Context) ([]appInfo, error) {
	startTimestamp := util.GetSystemTimestamp()
	a.api.Log(ctx, plugin.LogLevelInfo, "start to load app cache")
	var cachePath = a.getAppCachePath()
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		a.api.Log(ctx, plugin.LogLevelWarning, "app cache file not found")
		return nil, err
	}

	cacheContent, readErr := os.ReadFile(cachePath)
	if readErr != nil {
		a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error reading app cache file: %s", readErr.Error()))
		return nil, readErr
	}

	var apps []appInfo
	unmarshalErr := json.Unmarshal(cacheContent, &apps)
	if unmarshalErr != nil {
		a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error unmarshalling app cache file: %s", unmarshalErr.Error()))
		return nil, unmarshalErr
	}

	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("loaded %d apps from cache, cost %d ms", len(apps), util.GetSystemTimestamp()-startTimestamp))
	return apps, nil
}

// removeDuplicateApps removes duplicate apps with same Name and Path, keeping only one
func (a *ApplicationPlugin) removeDuplicateApps(ctx context.Context, apps []appInfo) []appInfo {
	seen := make(map[string]bool)
	var result []appInfo

	for _, app := range apps {
		// Create a unique key combining Name and Path
		key := app.Name + "|" + app.Path
		if !seen[key] {
			seen[key] = true
			result = append(result, app)
		} else {
			a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("removed duplicate app: %s (%s)", app.Name, app.Path))
		}
	}

	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("removed %d duplicate apps, %d apps remaining", len(apps)-len(result), len(result)))
	return result
}

func (a *ApplicationPlugin) handleMRURestore(mruData plugin.MRUData) (*plugin.QueryResult, error) {
	var contextData appContextData
	if err := json.Unmarshal([]byte(mruData.ContextData), &contextData); err != nil {
		return nil, fmt.Errorf("failed to parse context data: %w", err)
	}

	var appInfo *appInfo
	for _, info := range a.apps {
		if info.Name == contextData.Name && info.Path == contextData.Path {
			appInfo = &info
			break
		}
	}

	if appInfo == nil {
		return nil, fmt.Errorf("app not found: %s", contextData.Name)
	}

	displayPath := appInfo.GetDisplayPath()
	result := &plugin.QueryResult{
		Id:          uuid.NewString(),
		Title:       appInfo.Name,
		SubTitle:    displayPath,
		Icon:        appInfo.Icon, // Use current icon instead of cached MRU icon to handle cache invalidation
		ContextData: mruData.ContextData,
		Actions: []plugin.QueryResultAction{
			{
				Name: "i18n:plugin_app_open",
				Icon: plugin.OpenIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					runErr := shell.Open(appInfo.Path)
					if runErr != nil {
						a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error opening app %s: %s", appInfo.Path, runErr.Error()))
						a.api.Notify(ctx, fmt.Sprintf("i18n:plugin_app_open_failed_description: %s", runErr.Error()))
					}
				},
			},
			{
				Name: "i18n:plugin_app_open_containing_folder",
				Icon: plugin.OpenContainingFolderIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					if err := a.retriever.OpenAppFolder(ctx, *appInfo); err != nil {
						a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error opening folder: %s", err.Error()))
					}
				},
			},
			{
				Name: "i18n:plugin_app_copy_path",
				Icon: plugin.CopyIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					clipboard.WriteText(appInfo.Path)
				},
			},
		},
	}

	if appInfo.IsRunning() {
		result.Actions = append(result.Actions, plugin.QueryResultAction{
			Name: "i18n:plugin_app_terminate",
			Icon: plugin.TerminateAppIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				p, getErr := os.FindProcess(appInfo.Pid)
				if getErr != nil {
					a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error finding process %d: %s", appInfo.Pid, getErr.Error()))
					return
				}

				killErr := p.Kill()
				if killErr != nil {
					a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error killing process %d: %s", appInfo.Pid, killErr.Error()))
				}
			},
		})

		result.RefreshInterval = 1000
		result.OnRefresh = func(ctx context.Context, result plugin.RefreshableResult) plugin.RefreshableResult {
			result.Tails = a.getRunningProcessResult(*appInfo)
			return result
		}
	}

	return result, nil
}
