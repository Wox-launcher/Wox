package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wox/analytics"
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/filesearch"
	"wox/util/nativecontextmenu"
	"wox/util/shell"
	"wox/util/window"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/tidwall/pretty"
)

var appIcon = common.PluginAppIcon

var errSkipAppIndexing = errors.New("skip app indexing")

type AppType = string

const (
	AppTypeDesktop        AppType = "desktop"
	AppTypeUWP            AppType = "uwp"
	AppTypeWindowsSetting AppType = "windows_setting"
)

type appInfo struct {
	Name string `json:"name"`
	// SearchableNames keeps extra aliases for matching when Name alone is not stable enough.
	// On macOS without Spotlight metadata, the searchable value may come from the localized bundle name,
	// Info.plist, or the .app filename, and those names can differ for non-Latin apps.
	SearchableNames []string        `json:"searchable_names,omitempty"`
	Identity        string          `json:"identity,omitempty"`
	Path            string          `json:"path"`
	Icon            common.WoxImage `json:"icon"`
	// IconSourcePath records the real file used to render Icon. Windows shortcuts
	// can keep their own mtime while the target executable changes, so cache reuse
	// must compare this source in addition to the indexed shortcut path.
	IconSourcePath         string  `json:"icon_source_path,omitempty"`
	IconSourceModifiedUnix int64   `json:"icon_source_modified_unix,omitempty"`
	Type                   AppType `json:"type,omitempty"`
	LastModifiedUnix       int64   `json:"last_modified_unix,omitempty"`

	Pid int `json:"-"`
	// IsDefaultIcon is persisted so launchpad can hide entries whose icon fell
	// back to a generic/default asset after a restart. Normal app search still
	// keeps these entries visible.
	IsDefaultIcon bool `json:"is_default_icon,omitempty"`
}

type appCacheFile struct {
	Version int       `json:"version"`
	Apps    []appInfo `json:"apps"`
}

// Bump this when cached appInfo fields or preprocessed icon semantics change.
// Version 5 refreshes macOS searchable_names after localized aliases started
// reading Finder display names plus every InfoPlist.loctable/InfoPlist.strings
// localization. Keeping v4 would leave existing caches without names such as
// Korean Calculator.
// Version 6 refreshes Windows Settings entries after replacing legacy Control
// Panel file icons with built-in SVGs and adding System subpages. Keeping v5
// would continue loading old cached absolute icon paths and omit the new
// ms-settings rows until the user manually reindexed apps.
// Version 7 refreshes Windows Settings searchable aliases. Version 6 caches
// created before the alias table was complete would keep opening the new pages
// but miss English terms such as "display" for localized Settings titles.
// Version 8 refreshes app entries with icon-source metadata. The old cache only
// tracked the shortcut/file path mtime, so updated executable icons behind
// unchanged .lnk files could keep showing stale cached PNGs.
const appCacheVersion = 8

const (
	appCommandReindex   = "reindex"
	appCommandLaunchpad = "launchpad"
)

const (
	// Optimization: broad global app queries used to return hundreds of
	// applications, making result action creation and manager/UI polish dominate
	// latency while adding little value to the aggregated result list. Plugin
	// context and launchpad remain full browsing surfaces.
	appQueryResultLimitInGloablQuery = 50
)

const (
	appChangeDebounceWindow = 3 * time.Second
	appChangeMaxWait        = 20 * time.Second
)

type appPendingChange struct {
	Path         string
	SemanticKind filesearch.ChangeSemanticKind
	PathIsDir    bool
}

type appContextData struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

// appQueryEntry keeps query-only derived data so typing does not rebuild the same
// candidate lists for every app on every keystroke.
type appQueryEntry struct {
	info             appInfo
	searchCandidates []string
	ignoreCandidates []string
}

// Query results usually shrink as the user extends the same search text.
// Reusing the previous matched subset avoids rescanning the full app list.
type appQuerySessionCache struct {
	generation uint64
	search     string
	matches    []int
	startedAt  int64
}

// appQueryMatch is a lightweight matched candidate. Query builds full
// QueryResult objects only after capping so broad searches do not pay action,
// icon, and result-cache costs for rows the UI will not need.
type appQueryMatch struct {
	entryIndex  int
	entry       appQueryEntry
	displayName string
	displayPath string
	score       int64
}

func (a *appInfo) GetDisplayPath() string {
	if a.Type == AppTypeUWP || a.Type == AppTypeWindowsSetting {
		return ""
	}
	return a.Path
}

func (a *appInfo) GetSearchCandidates(displayName string) []string {
	candidates := []string{displayName, a.Name}
	candidates = append(candidates, a.SearchableNames...)

	baseName := filepath.Base(a.Path)
	candidates = append(candidates, baseName)
	if ext := filepath.Ext(baseName); ext != "" {
		candidates = append(candidates, strings.TrimSuffix(baseName, ext))
	}

	var filtered []string
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		filtered = append(filtered, candidate)
	}

	return util.UniqueStrings(filtered)
}

func (a *appInfo) IsRunning() bool {
	return a.Pid > 0
}

func isMacPrefPanePath(appPath string) bool {
	return util.IsMacOS() && strings.HasSuffix(strings.ToLower(appPath), ".prefpane")
}

func isMacSystemSettingsPath(appPath string) bool {
	if !util.IsMacOS() {
		return false
	}
	if isMacPrefPanePath(appPath) {
		return true
	}
	return strings.HasPrefix(strings.ToLower(appPath), "x-apple.systempreferences:")
}

type appDirectory struct {
	Path              string
	Recursive         bool
	RecursiveDepth    int
	RecursiveExcludes []string
	excludeAbsPaths   []string // internal: absolute paths of excluded directories
	trackChanges      bool     // internal: true for roots whose precise file changes should update the app cache
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ApplicationPlugin{})
}

type ApplicationPlugin struct {
	api             plugin.API
	pluginDirectory string

	apps      []appInfo
	retriever Retriever

	hotkeyAppCandidates []setting.IgnoredHotkeyApp

	// These caches move stable work out of the query hot path.
	queryEntries           []appQueryEntry
	queryEntriesMutex      sync.RWMutex
	queryEntriesGeneration uint64
	querySessionCache      *util.HashMap[string, appQuerySessionCache]
	ignoreMatchers         []appIgnoreMatcher

	// Track results that need periodic refresh (running apps with CPU/memory stats)
	trackedResults *util.HashMap[string, appInfo] // resultId -> appInfo
}

func (a *ApplicationPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "ea2b6859-14bc-4c89-9c88-627da7379141",
		Name:          "i18n:plugin_app_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_app_plugin_description",
		Icon:          appIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
			"app",
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
		Commands: []plugin.MetadataCommand{
			{
				Command:     appCommandReindex,
				Description: "i18n:plugin_app_command_reindex",
			},
			{
				Command:     appCommandLaunchpad,
				Description: "i18n:plugin_app_command_launchpad",
			},
		},
		SettingDefinitions: []definition.PluginSettingDefinitionItem{
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     "AppDirectories",
					Title:   "i18n:plugin_app_directories",
					Tooltip: "i18n:plugin_app_directories_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "Path",
							Label: "i18n:plugin_app_path",
							Type:  definition.PluginSettingValueTableColumnTypeDirPath,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     "IgnoreRules",
					Title:   "i18n:plugin_app_ignore_rules",
					Tooltip: "i18n:plugin_app_ignore_rules_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:     "Pattern",
							Label:   "i18n:plugin_app_ignore_rule_pattern",
							Tooltip: "i18n:plugin_app_ignore_rule_pattern_tooltip",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
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
	a.trackedResults = util.NewHashMap[string, appInfo]()
	a.querySessionCache = util.NewHashMap[string, appQuerySessionCache]()
	a.rebuildIgnoreRuleMatchers(ctx)

	appCache, cacheErr := a.loadAppCache(ctx)
	if cacheErr == nil {
		a.apps = appCache
		a.rebuildHotkeyAppCandidates(ctx)
		a.rebuildQueryEntries(ctx)
	}

	util.Go(ctx, "index apps", func() {
		a.indexApps(util.NewTraceContext())
	})
	util.Go(ctx, "watch app changes", func() {
		a.watchAppChanges(util.NewTraceContext())
	})
	util.Go(ctx, "refresh running apps", func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			a.refreshRunningApps(util.NewTraceContext())
		}
	})

	a.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key == "AppDirectories" {
			a.indexApps(callbackCtx)
			return
		}
		if key == "IgnoreRules" {
			a.rebuildIgnoreRuleMatchers(callbackCtx)
			a.rebuildQueryEntries(callbackCtx)
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

func (a *ApplicationPlugin) populateAppMetadata(ctx context.Context, appPath string, info *appInfo, fileInfo os.FileInfo) {
	if strings.TrimSpace(info.Path) == "" {
		info.Path = appPath
	}
	info.Identity = strings.TrimSpace(resolveAppIdentityForPlatform(ctx, *info))

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
	a.populateIconSourceMetadata(ctx, info)
}

func (a *ApplicationPlugin) populateIconSourceMetadata(ctx context.Context, info *appInfo) {
	iconSourcePath := strings.TrimSpace(info.IconSourcePath)
	if iconSourcePath == "" {
		info.IconSourceModifiedUnix = 0
		return
	}

	iconSourcePath = filepath.Clean(iconSourcePath)
	fileInfo, statErr := os.Stat(iconSourcePath)
	if statErr != nil {
		// Bug fix: keep missing icon sources from being treated as fresh cache
		// entries. Reindexing later can recover after installers finish moving
		// files into place, while pathless icons still keep the existing fast path.
		a.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("app icon source stat failed: path=%s err=%s", iconSourcePath, statErr.Error()))
		info.IconSourcePath = iconSourcePath
		info.IconSourceModifiedUnix = 0
		return
	}

	info.IconSourcePath = iconSourcePath
	info.IconSourceModifiedUnix = fileInfo.ModTime().UnixNano()
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

	if !a.isCachedIconSourceFresh(ctx, cached) {
		return appInfo{}, false
	}

	if cached.Icon.ImageType == common.WoxImageTypeAbsolutePath && cached.Icon.ImageData != "" {
		if _, err := os.Stat(cached.Icon.ImageData); err != nil {
			a.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("cached icon missing for %s, reindexing", appPath))
			return appInfo{}, false
		}
	}

	cached.Pid = 0
	if strings.TrimSpace(cached.Identity) == "" {
		cached.Identity = strings.TrimSpace(resolveAppIdentityForPlatform(ctx, cached))
	}
	return cached, true
}

func (a *ApplicationPlugin) isCachedIconSourceFresh(ctx context.Context, cached appInfo) bool {
	iconSourcePath := strings.TrimSpace(cached.IconSourcePath)
	if iconSourcePath == "" {
		return cached.IconSourceModifiedUnix == 0
	}

	fileInfo, statErr := os.Stat(iconSourcePath)
	if statErr != nil {
		a.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("cached icon source missing for %s, reindexing: %s", cached.Path, statErr.Error()))
		return false
	}

	if cached.IconSourceModifiedUnix == 0 || cached.IconSourceModifiedUnix != fileInfo.ModTime().UnixNano() {
		// Bug fix: shortcut entries can keep the same .lnk mtime after the target
		// app updates. Treat the icon source mtime as part of app cache freshness
		// so stale appInfo records are reparsed and the new fileicon cache key is used.
		a.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("cached icon source changed for %s, reindexing", cached.Path))
		return false
	}

	return true
}

func (a *ApplicationPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	// clean cache and reindex apps
	if query.Command == appCommandReindex {
		reindexId := uuid.NewString()
		return plugin.NewQueryResponse([]plugin.QueryResult{
			{
				Id:    reindexId,
				Title: "i18n:plugin_app_reindex",
				Icon:  appIcon,
				Actions: []plugin.QueryResultAction{
					{
						Name: "i18n:plugin_app_start_reindex",
						Icon: common.ExecuteRunIcon,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							util.Go(ctx, "reindex app", func() {
								// clean cache file first
								cachePath := a.getAppCachePath()
								if err := os.Remove(cachePath); err == nil {
									a.api.Log(ctx, plugin.LogLevelInfo, "app cache file removed")
								}
								imageCache := util.GetLocation().GetImageCacheDirectory()
								if err := os.RemoveAll(imageCache); err == nil {
									common.ClearConvertIconPathExistenceCache()
									a.api.Log(ctx, plugin.LogLevelInfo, "image cache directory removed")
								}
								// clear in-memory app list
								a.apps = []appInfo{}
								a.hotkeyAppCandidates = []setting.IgnoredHotkeyApp{}

								a.indexApps(ctx)
								a.api.Notify(ctx, "i18n:plugin_app_reindex_completed")
							})
						},
					},
				},
			},
		})
	}

	isLaunchpadQuery := query.Command == appCommandLaunchpad

	// Query against a stable snapshot so reindexing or settings changes do not
	// force extra work in the middle of a keystroke.
	entries, generation := a.getQueryEntriesSnapshot()
	startedAt := time.Now().UnixNano()
	queryStartedAt := util.GetSystemTimestamp()

	// When the user grows the same search prefix, most fuzzy searches can reuse
	// the previous matched subset. Pinyin has a separate guard inside
	// getReusableQueryMatches because syllable-boundary typing is not monotonic.
	cachedMatches, canReuseCachedMatches := a.getReusableQueryMatches(ctx, query, generation)

	matchedIndexes := make([]int, 0, len(entries))
	matchCapacity := len(entries)
	if matchCapacity > appQueryResultLimitInGloablQuery {
		matchCapacity = appQueryResultLimitInGloablQuery
	}
	matches := make([]appQueryMatch, 0, matchCapacity)

	matchEntry := func(entryIndex int) {
		entry := entries[entryIndex]
		if isLaunchpadQuery && !a.shouldShowInLaunchpad(entry.info) {
			return
		}

		displayName, displayPath, searchCandidates := a.resolveQueryEntryDisplay(ctx, entry)

		isMatch := false
		bestScore := int64(0)
		for _, candidate := range searchCandidates {
			matched, score := plugin.IsStringMatchScore(ctx, candidate, query.Search)
			if !matched {
				continue
			}

			if !isMatch || score > bestScore {
				isMatch = true
				bestScore = score
			}
		}

		if !isMatch {
			return
		}

		matches = append(matches, appQueryMatch{
			entryIndex:  entryIndex,
			entry:       entry,
			displayName: displayName,
			displayPath: displayPath,
			score:       bestScore,
		})
		matchedIndexes = append(matchedIndexes, entryIndex)
	}

	if canReuseCachedMatches {
		for _, entryIndex := range cachedMatches {
			if entryIndex < 0 || entryIndex >= len(entries) {
				continue
			}
			matchEntry(entryIndex)
		}
	} else {
		for entryIndex := range entries {
			matchEntry(entryIndex)
		}
	}

	limitGlobalQueryResults := query.IsGlobalQuery()
	selectedMatches, droppedDefaultIconCount := selectAppQueryMatches(matches, limitGlobalQueryResults)
	if limitGlobalQueryResults && len(matches) > len(selectedMatches) {
		a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf(
			"app global query capped results: matched=%d returned=%d limit=%d dropped_default_icon=%d cost=%dms",
			len(matches),
			len(selectedMatches),
			appQueryResultLimitInGloablQuery,
			droppedDefaultIconCount,
			util.GetSystemTimestamp()-queryStartedAt,
		))
	}

	results := make([]plugin.QueryResult, 0, len(selectedMatches))
	for _, match := range selectedMatches {
		entry := match.entry
		resultID := uuid.NewString()
		contextData := common.ContextData{
			"name": entry.info.Name,
			"path": entry.info.Path,
			"type": entry.info.Type,
		}
		actions := a.buildAppActions(entry.info, match.displayName, contextData)
		result := plugin.QueryResult{
			Id:       resultID,
			Title:    match.displayName,
			SubTitle: match.displayPath,
			Icon:     a.getQueryResultIcon(entry.info, isLaunchpadQuery),
			Score:    match.score,
			Actions:  actions,
		}

		// Launchpad mode is a static app grid that replaces macOS Launchpad's removed entry point.
		// The normal app query tracks visible rows so CPU/memory tails and terminate actions stay fresh,
		// but those running-state updates make a Launchpad-style grid noisy and can resize cells while browsing.
		if !isLaunchpadQuery {
			a.trackedResults.Store(result.Id, entry.info)
		}

		results = append(results, result)
	}

	a.storeQueryMatches(query, appQuerySessionCache{
		generation: generation,
		search:     query.Search,
		matches:    matchedIndexes,
		startedAt:  startedAt,
	})

	response := plugin.NewQueryResponse(results)
	if isLaunchpadQuery {
		gridLayout := plugin.MetadataFeatureParamsGridLayout{
			Columns:     7,
			ShowTitle:   true,
			ItemPadding: 10,
			ItemMargin:  4,
		}
		response.Layout = plugin.QueryLayout{GridLayout: &gridLayout}
	}
	return response
}

func selectAppQueryMatches(matches []appQueryMatch, shouldLimit bool) ([]appQueryMatch, int) {
	if !shouldLimit || len(matches) <= appQueryResultLimitInGloablQuery {
		return matches, 0
	}

	selected := make([]appQueryMatch, len(matches))
	copy(selected, matches)
	sort.SliceStable(selected, func(i, j int) bool {
		leftDefaultIcon := selected[i].entry.info.IsDefaultIcon
		rightDefaultIcon := selected[j].entry.info.IsDefaultIcon
		if leftDefaultIcon != rightDefaultIcon {
			// Optimization: when broad searches must be capped, prefer keeping apps
			// with real icons because default-icon entries are usually lower-signal
			// executables discovered from broad Windows roots.
			return !leftDefaultIcon
		}
		if selected[i].score != selected[j].score {
			return selected[i].score > selected[j].score
		}
		if selected[i].displayName != selected[j].displayName {
			return selected[i].displayName < selected[j].displayName
		}
		return selected[i].displayPath < selected[j].displayPath
	})

	selected = selected[:appQueryResultLimitInGloablQuery]
	selectedDefaultIcons := 0
	for _, match := range selected {
		if match.entry.info.IsDefaultIcon {
			selectedDefaultIcons++
		}
	}

	totalDefaultIcons := 0
	for _, match := range matches {
		if match.entry.info.IsDefaultIcon {
			totalDefaultIcons++
		}
	}

	return selected, totalDefaultIcons - selectedDefaultIcons
}

func (a *ApplicationPlugin) buildAppActions(info appInfo, displayName string, contextData map[string]string) []plugin.QueryResultAction {
	actions := []plugin.QueryResultAction{
		{
			Name:        "i18n:plugin_app_open",
			Icon:        common.OpenIcon,
			ContextData: contextData,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				analytics.TrackAppLaunched(ctx, fmt.Sprintf("%s:%s", info.Type, info.Name), displayName)

				// Check if app is already running and try to activate its window
				// macos default behavior is to activate existing instance
				// windows needs special handling to activate existing window
				if util.IsWindows() {
					currentPid := a.retriever.GetPid(ctx, info)
					if currentPid > 0 {
						// App is running, try to activate its window
						if window.ActivateWindowByPid(currentPid) {
							a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Activated existing window for %s (PID: %d)", info.Name, currentPid))
							return
						}
						// If activation failed, fall through to launch new instance
						a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Could not activate window for %s, launching new instance", info.Name))
					}
				}

				// Bug fix: Linux app indexing now returns .desktop launcher files instead of raw
				// executables. Launching them through gio preserves the desktop entry's Exec
				// handling and environment wrappers; xdg-open remains the compatibility fallback.
				var runErr error
				if util.IsLinux() && strings.HasSuffix(strings.ToLower(info.Path), ".desktop") {
					_, runErr = shell.Run("gio", "launch", info.Path)
					if runErr != nil {
						a.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("gio launch failed for %s: %s", info.Path, runErr.Error()))
						runErr = shell.Open(info.Path)
					}
				} else {
					runErr = shell.Open(info.Path)
				}
				if runErr != nil {
					a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error opening app %s: %s", info.Path, runErr.Error()))
					a.api.Notify(ctx, fmt.Sprintf("i18n:plugin_app_open_failed_description: %s", runErr.Error()))
				}
			},
		},
	}

	if info.Type != AppTypeWindowsSetting {
		actions = append(actions, plugin.QueryResultAction{
			Name:        "i18n:plugin_app_open_containing_folder",
			Icon:        common.OpenContainingFolderIcon,
			ContextData: contextData,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if err := a.retriever.OpenAppFolder(ctx, info); err != nil {
					a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error opening folder: %s", err.Error()))
				}
			},
		})
	}

	actions = append(actions, plugin.QueryResultAction{
		Name:        "i18n:plugin_app_copy_path",
		Icon:        common.CopyIcon,
		ContextData: contextData,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			clipboard.WriteText(info.Path)
		},
	})

	// Bug fix: Linux cannot show the true system context menu behind this action,
	// so keep the entry only on platforms where nativecontextmenu can honor the
	// label instead of exposing a file-manager fallback as if it were equivalent.
	if info.Type != AppTypeUWP && info.Type != AppTypeWindowsSetting && nativecontextmenu.IsSupported() {
		actions = append(actions, plugin.QueryResultAction{
			Name:        "i18n:plugin_file_show_context_menu",
			Icon:        common.PluginMenusIcon,
			ContextData: contextData,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				a.api.Log(ctx, plugin.LogLevelInfo, "Showing context menu for: "+info.Path)
				err := nativecontextmenu.ShowContextMenu(info.Path)
				if err != nil {
					a.api.Log(ctx, plugin.LogLevelError, err.Error())
					a.api.Notify(ctx, err.Error())
				}
			},
			Hotkey:                 util.PrimaryHotkey("m"),
			PreventHideAfterAction: true,
		})
	}

	return actions
}

func (a *ApplicationPlugin) getQueryResultIcon(info appInfo, isLaunchpadQuery bool) common.WoxImage {
	if !isLaunchpadQuery {
		return info.Icon
	}

	// Launchpad uses the generic grid icon polish path. Return the app path as a file icon source
	// instead of the indexed list icon so Manager.PolishResult can resolve it at the core grid size.
	if strings.TrimSpace(info.Path) != "" && info.Type != AppTypeWindowsSetting && !isMacSystemSettingsPath(info.Path) {
		return common.NewWoxImageFileIcon(info.Path)
	}

	// Generated settings icons and pathless entries cannot be resolved from the file icon API,
	// so they fall back to the already indexed icon and use the same grid-size polish as other results.
	return info.Icon
}

func (a *ApplicationPlugin) shouldShowInLaunchpad(info appInfo) bool {
	// Launchpad is a visual app grid, so entries with no usable icon create
	// noise and look broken. Keep the regular app query unchanged because users
	// may still need to find and open those apps by name.
	if info.IsDefaultIcon || info.Icon.IsEmpty() {
		return false
	}
	return true
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
	directories := a.getAppDirectories(ctx)
	appExtensions := a.retriever.GetAppExtensions(ctx)
	a.watchRootOnlyAppChanges(ctx, directories, appExtensions)

	roots := a.getAppChangeRoots(ctx)
	if len(roots) == 0 {
		a.api.Log(ctx, plugin.LogLevelInfo, "app change feed skipped: no tracked app directories")
		return
	}

	feed := filesearch.NewChangeFeed()
	defer feed.Close()
	if refreshErr := feed.Refresh(ctx, roots); refreshErr != nil {
		a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to start app change feed: %s", refreshErr.Error()))
		return
	}
	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app change feed started: roots=%d mode=%s", len(roots), feed.Mode()))
	rootPaths := map[string]string{}
	for _, root := range roots {
		rootPaths[root.ID] = root.Path
	}

	pending := map[string]appPendingChange{}
	var firstPendingAt time.Time
	var timer *time.Timer
	var timerC <-chan time.Time

	stopTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer = nil
		timerC = nil
	}

	scheduleFlush := func(now time.Time) {
		if len(pending) == 0 {
			stopTimer()
			firstPendingAt = time.Time{}
			return
		}
		if firstPendingAt.IsZero() {
			firstPendingAt = now
		}
		delay := appChangeDebounceWindow
		if elapsed := now.Sub(firstPendingAt); elapsed >= appChangeMaxWait {
			delay = 0
		} else if remaining := appChangeMaxWait - elapsed; remaining < delay {
			delay = remaining
		}
		if delay <= 0 {
			stopTimer()
			a.applyPendingAppChanges(ctx, pending)
			pending = map[string]appPendingChange{}
			firstPendingAt = time.Time{}
			return
		}
		stopTimer()
		timer = time.NewTimer(delay)
		timerC = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			stopTimer()
			return
		case <-timerC:
			a.applyPendingAppChanges(ctx, pending)
			pending = map[string]appPendingChange{}
			firstPendingAt = time.Time{}
			stopTimer()
		case signal, ok := <-feed.Signals():
			if !ok {
				stopTimer()
				return
			}

			a.logAppChangeFeedSignal(ctx, signal, rootPaths)
			change, ok := a.getActionableAppChange(signal, appExtensions, rootPaths)
			if !ok {
				continue
			}
			pending[a.appChangeTaskKey(change)] = change
			a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app change feed accepted: path=%s semantic=%s isDir=%t pending=%d", change.Path, change.SemanticKind, change.PathIsDir, len(pending)))
			scheduleFlush(time.Now())
		}
	}
}

func (a *ApplicationPlugin) logAppChangeFeedSignal(ctx context.Context, signal filesearch.ChangeSignal, rootPaths map[string]string) {
	rootPath := rootPaths[signal.RootID]
	if rootPath == "" {
		rootPath = "<unknown>"
	}
	at := ""
	if !signal.At.IsZero() {
		at = signal.At.Format(time.RFC3339Nano)
	}
	// Diagnostic logging only: the app change feed crosses from filesearch into
	// the app plugin here, so log the raw signal before filtering to verify
	// installer/uninstaller flows without changing indexing behavior.
	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf(
		"app change feed signal: kind=%s semantic=%s feed=%s root=%s rootPath=%s path=%s isDir=%t pathTypeKnown=%t reason=%s cursor=%s at=%s",
		signal.Kind,
		signal.SemanticKind,
		signal.FeedType,
		signal.RootID,
		rootPath,
		signal.Path,
		signal.PathIsDir,
		signal.PathTypeKnown,
		signal.Reason,
		signal.Cursor,
		at,
	))
}

func (a *ApplicationPlugin) watchRootOnlyAppChanges(ctx context.Context, appDirectories []appDirectory, appExtensions []string) {
	for _, d := range appDirectories {
		if d.trackChanges {
			continue
		}

		directory := d
		util.WatchDirectoryChanges(ctx, directory.Path, func(e fsnotify.Event) {
			appPath := filepath.Clean(e.Name)
			if !a.isAppPathExtensionMatch(appPath, appExtensions) {
				return
			}

			a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app %s changed (%s)", appPath, e.Op))
			changed := false
			if e.Op&fsnotify.Remove == fsnotify.Remove || e.Op&fsnotify.Rename == fsnotify.Rename {
				changed = a.removeIndexedAppByPath(ctx, appPath)
			} else if e.Op&fsnotify.Create == fsnotify.Create || e.Op&fsnotify.Write == fsnotify.Write {
				// Bug fix: keep the legacy root-only watcher as a compatibility path for
				// untracked roots, but apply the same local update strategy as the precise
				// change feed so root-level changes never trigger a full app index.
				time.Sleep(appChangeDebounceWindow)
				changed = a.upsertIndexedAppByPath(ctx, appPath)
			}

			if changed {
				a.rebuildHotkeyAppCandidates(ctx)
				a.rebuildQueryEntries(ctx)
				a.saveAppToCache(ctx)
				a.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			}
		})
	}
}

func (a *ApplicationPlugin) getAppChangeRoots(ctx context.Context) []filesearch.RootRecord {
	directories := a.getAppDirectories(ctx)
	roots := make([]filesearch.RootRecord, 0, len(directories))
	now := util.GetSystemTimestamp()
	for _, directory := range directories {
		if !directory.trackChanges || strings.TrimSpace(directory.Path) == "" {
			continue
		}
		cleanPath := filepath.Clean(directory.Path)
		info, statErr := os.Stat(cleanPath)
		if statErr != nil {
			a.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("skip app change root %s: %s", cleanPath, statErr.Error()))
			continue
		}
		if !info.IsDir() {
			continue
		}
		roots = append(roots, filesearch.RootRecord{
			ID:        uuid.NewString(),
			Path:      cleanPath,
			Kind:      filesearch.RootKindDefault,
			Status:    filesearch.RootStatusIdle,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	return roots
}

func (a *ApplicationPlugin) appChangeTaskKey(change appPendingChange) string {
	prefix := "file:"
	if change.PathIsDir {
		prefix = "dir:"
	}
	return prefix + a.pathCacheKey(change.Path)
}

func (a *ApplicationPlugin) getActionableAppChange(signal filesearch.ChangeSignal, appExtensions []string, rootPaths map[string]string) (appPendingChange, bool) {
	if signal.Kind != filesearch.ChangeSignalKindDirtyPath {
		if signal.Kind == filesearch.ChangeSignalKindDirtyRoot && signal.SemanticKind == filesearch.ChangeSemanticKindRemove && strings.TrimSpace(signal.Path) != "" {
			changePath := filepath.Clean(signal.Path)
			if rootPath := rootPaths[signal.RootID]; rootPath != "" && a.pathCacheKey(filepath.Clean(rootPath)) == a.pathCacheKey(changePath) {
				// Bug fix: fallback feeds may mark a whole root dirty when precision is
				// insufficient. Updating every cached app under that root would be a
				// hidden full reconcile, so only path-level remove signals are allowed.
				a.api.Log(context.Background(), plugin.LogLevelInfo, fmt.Sprintf("app change feed skipped: dirty root remove points at root path=%s", changePath))
				return appPendingChange{}, false
			}

			// Bug fix: Windows fallback notifications report deletes as dirty_root with
			// pathTypeKnown=false, but still carry the changed path. Treat that as a
			// local remove only for concrete app files or a cached subdirectory, avoiding
			// the expensive full indexApps fallback while keeping uninstall cleanup live.
			if a.isAppPathExtensionMatch(changePath, appExtensions) {
				return appPendingChange{Path: changePath, SemanticKind: signal.SemanticKind}, true
			}
			if a.hasIndexedAppsUnderDirectory(changePath) {
				return appPendingChange{Path: changePath, SemanticKind: signal.SemanticKind, PathIsDir: true}, true
			}
			a.api.Log(context.Background(), plugin.LogLevelInfo, fmt.Sprintf("app change feed skipped: dirty root remove did not match cached app path=%s", changePath))
			return appPendingChange{}, false
		}
		if signal.Kind == filesearch.ChangeSignalKindRequiresRootReconcile || signal.Kind == filesearch.ChangeSignalKindFeedUnavailable {
			// Diagnostic logging only: app indexing intentionally avoids fallback full
			// reindexing because broad app scans are expensive. Keep the skip visible
			// while testing install/uninstall flows so we can tell whether filesearch
			// produced a precise path or only a root-level reconciliation request.
			a.api.Log(context.Background(), plugin.LogLevelInfo, fmt.Sprintf("app change feed skipped: kind=%s reason=%s path=%s", signal.Kind, signal.Reason, signal.Path))
		}
		return appPendingChange{}, false
	}
	if signal.Path == "" {
		a.api.Log(context.Background(), plugin.LogLevelInfo, fmt.Sprintf("app change feed skipped: empty path semantic=%s reason=%s", signal.SemanticKind, signal.Reason))
		return appPendingChange{}, false
	}
	changePath := filepath.Clean(signal.Path)
	if signal.PathIsDir {
		switch signal.SemanticKind {
		case filesearch.ChangeSemanticKindCreate,
			filesearch.ChangeSemanticKindModify,
			filesearch.ChangeSemanticKindRename,
			filesearch.ChangeSemanticKindRemove:
			// Bug fix: installers often create a vendor folder under Start Menu and the
			// fallback feed only reports that directory, not each nested .lnk. Reconcile
			// just this directory so nested shortcuts update without a full app scan.
			return appPendingChange{Path: changePath, SemanticKind: signal.SemanticKind, PathIsDir: true}, true
		default:
			a.api.Log(context.Background(), plugin.LogLevelInfo, fmt.Sprintf("app change feed skipped: directory path=%s semantic=%s reason=%s", signal.Path, signal.SemanticKind, signal.Reason))
			return appPendingChange{}, false
		}
	}
	if !a.isAppPathExtensionMatch(changePath, appExtensions) {
		a.api.Log(context.Background(), plugin.LogLevelInfo, fmt.Sprintf("app change feed skipped: extension mismatch path=%s semantic=%s reason=%s", signal.Path, signal.SemanticKind, signal.Reason))
		return appPendingChange{}, false
	}

	switch signal.SemanticKind {
	case filesearch.ChangeSemanticKindCreate,
		filesearch.ChangeSemanticKindRemove,
		filesearch.ChangeSemanticKindRename,
		filesearch.ChangeSemanticKindModify:
		return appPendingChange{Path: changePath, SemanticKind: signal.SemanticKind}, true
	default:
		a.api.Log(context.Background(), plugin.LogLevelInfo, fmt.Sprintf("app change feed skipped: semantic=%s path=%s reason=%s", signal.SemanticKind, signal.Path, signal.Reason))
		return appPendingChange{}, false
	}
}

func (a *ApplicationPlugin) applyPendingAppChanges(ctx context.Context, pending map[string]appPendingChange) {
	if len(pending) == 0 {
		return
	}

	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app change feed flushing pending changes: count=%d", len(pending)))
	changed := false
	for _, change := range pending {
		if change.PathIsDir {
			switch change.SemanticKind {
			case filesearch.ChangeSemanticKindRemove:
				changed = a.removeIndexedAppsUnderDirectory(ctx, change.Path) || changed
			case filesearch.ChangeSemanticKindCreate, filesearch.ChangeSemanticKindModify, filesearch.ChangeSemanticKindRename:
				changed = a.reconcileIndexedAppsInDirectory(ctx, change.Path) || changed
			}
			continue
		}

		switch change.SemanticKind {
		case filesearch.ChangeSemanticKindRemove:
			changed = a.removeIndexedAppByPath(ctx, change.Path) || changed
		case filesearch.ChangeSemanticKindCreate, filesearch.ChangeSemanticKindModify, filesearch.ChangeSemanticKindRename:
			if _, statErr := os.Stat(change.Path); statErr != nil {
				a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app change feed stat failed, removing cached entry if present: path=%s err=%s", change.Path, statErr.Error()))
				changed = a.removeIndexedAppByPath(ctx, change.Path) || changed
				continue
			}
			changed = a.upsertIndexedAppByPath(ctx, change.Path) || changed
		}
	}

	if !changed {
		return
	}

	// Bug fix: runtime app changes previously updated a.apps/cache only for root-level
	// fsnotify events, while searches read the prebuilt queryEntries snapshot. Rebuilding
	// once per debounced batch keeps newly installed apps searchable without a full scan.
	a.rebuildHotkeyAppCandidates(ctx)
	a.rebuildQueryEntries(ctx)
	a.saveAppToCache(ctx)
}

func (a *ApplicationPlugin) removeIndexedAppByPath(ctx context.Context, appPath string) bool {
	for i, app := range a.apps {
		if a.pathCacheKey(app.Path) != a.pathCacheKey(appPath) {
			continue
		}
		a.apps = append(a.apps[:i], a.apps[i+1:]...)
		a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app %s removed by change feed", appPath))
		return true
	}
	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app %s remove requested by change feed but no cached entry matched", appPath))
	return false
}

func (a *ApplicationPlugin) hasIndexedAppsUnderDirectory(directoryPath string) bool {
	for _, app := range a.apps {
		if a.isPathAtOrUnderDirectory(app.Path, directoryPath) {
			return true
		}
	}
	return false
}

func (a *ApplicationPlugin) removeIndexedAppsUnderDirectory(ctx context.Context, directoryPath string) bool {
	kept := make([]appInfo, 0, len(a.apps))
	removed := 0
	for _, app := range a.apps {
		if a.isPathAtOrUnderDirectory(app.Path, directoryPath) {
			removed++
			continue
		}
		kept = append(kept, app)
	}
	if removed == 0 {
		a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app directory %s remove requested by change feed but no cached entries matched", directoryPath))
		return false
	}

	a.apps = kept
	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app directory %s removed %d cached apps by change feed", directoryPath, removed))
	return true
}

func (a *ApplicationPlugin) reconcileIndexedAppsInDirectory(ctx context.Context, directoryPath string) bool {
	info, statErr := os.Stat(directoryPath)
	if statErr != nil {
		a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app change feed directory stat failed, removing cached entries if present: path=%s err=%s", directoryPath, statErr.Error()))
		return a.removeIndexedAppsUnderDirectory(ctx, directoryPath)
	}
	if !info.IsDir() {
		return false
	}

	// Bug fix: directory-only notifications from Start Menu installers need a
	// bounded local reconciliation. Scanning just the changed directory preserves
	// the user's no-full-index requirement while discovering nested shortcuts.
	paths := a.getAppPaths(ctx, []appDirectory{a.getLocalAppDirectoryForChange(ctx, directoryPath)})
	currentPaths := make(map[string]bool, len(paths))
	changed := false
	for _, appPath := range paths {
		currentPaths[a.pathCacheKey(appPath)] = true
		changed = a.upsertIndexedAppByPath(ctx, appPath) || changed
	}

	for _, app := range append([]appInfo(nil), a.apps...) {
		if !a.isPathAtOrUnderDirectory(app.Path, directoryPath) {
			continue
		}
		if currentPaths[a.pathCacheKey(app.Path)] {
			continue
		}
		changed = a.removeIndexedAppByPath(ctx, app.Path) || changed
	}

	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app directory %s reconciled by change feed: paths=%d changed=%t", directoryPath, len(paths), changed))
	return changed
}

func (a *ApplicationPlugin) getLocalAppDirectoryForChange(ctx context.Context, directoryPath string) appDirectory {
	cleanDirectory := filepath.Clean(directoryPath)
	for _, root := range a.getAppDirectories(ctx) {
		if !root.trackChanges || strings.TrimSpace(root.Path) == "" {
			continue
		}
		cleanRoot := filepath.Clean(root.Path)
		if !a.isPathAtOrUnderDirectory(cleanDirectory, cleanRoot) {
			continue
		}

		remainingDepth := root.RecursiveDepth
		if rel, relErr := filepath.Rel(cleanRoot, cleanDirectory); relErr == nil && rel != "." {
			remainingDepth -= len(strings.Split(rel, string(os.PathSeparator)))
			if remainingDepth < 0 {
				remainingDepth = 0
			}
		}
		return appDirectory{
			Path:              cleanDirectory,
			Recursive:         root.Recursive && remainingDepth > 0,
			RecursiveDepth:    remainingDepth,
			RecursiveExcludes: root.RecursiveExcludes,
		}
	}

	return appDirectory{Path: cleanDirectory, Recursive: true, RecursiveDepth: 1}
}

func (a *ApplicationPlugin) upsertIndexedAppByPath(ctx context.Context, appPath string) bool {
	// Feature change: precise change-feed paths let us refresh one app entry instead
	// of calling indexApps(). This keeps installer bursts cheap while still reusing
	// the existing platform parser and icon conversion behavior.
	var info appInfo
	var getErr error
	for i := 0; i < 3; i++ {
		info, getErr = a.retriever.ParseAppInfo(ctx, appPath)
		if getErr == nil {
			break
		}
		time.Sleep(time.Second * time.Duration(i+1))
	}
	if getErr != nil {
		if !errors.Is(getErr, errSkipAppIndexing) {
			a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error getting app info for %s: %s", appPath, getErr.Error()))
		}
		// If a path is still locked or otherwise unreadable after a short retry,
		// skip the local update rather than falling back to the expensive full index.
		return false
	}

	a.populateAppMetadata(ctx, appPath, &info, nil)
	info.Icon = common.ConvertIcon(ctx, info.Icon, a.pluginDirectory)

	for i, app := range a.apps {
		if a.pathCacheKey(app.Path) != a.pathCacheKey(appPath) {
			continue
		}
		a.apps[i] = info
		a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app %s updated by change feed", appPath))
		return true
	}

	a.apps = append(a.apps, info)
	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app %s added by change feed", appPath))
	return true
}

func (a *ApplicationPlugin) isAppPathExtensionMatch(appPath string, appExtensions []string) bool {
	lowerPath := strings.ToLower(filepath.Clean(appPath))
	return lo.ContainsBy(appExtensions, func(ext string) bool {
		return strings.HasSuffix(lowerPath, fmt.Sprintf(".%s", strings.ToLower(ext)))
	})
}

func (a *ApplicationPlugin) isPathAtOrUnderDirectory(appPath string, directoryPath string) bool {
	cleanAppPath := a.pathCacheKey(filepath.Clean(appPath))
	cleanDirectoryPath := a.pathCacheKey(filepath.Clean(directoryPath))
	if cleanAppPath == cleanDirectoryPath {
		return true
	}
	if !strings.HasSuffix(cleanDirectoryPath, string(os.PathSeparator)) {
		cleanDirectoryPath += string(os.PathSeparator)
	}
	return strings.HasPrefix(cleanAppPath, cleanDirectoryPath)
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
			// Check both Name and Path to support aliases (same path, different name)
			if app.Name == extraApp.Name && app.Path == extraApp.Path {
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
	a.rebuildHotkeyAppCandidates(ctx)
	a.rebuildQueryEntries(ctx)
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
		// Feature change: user-added app directories are explicit, bounded roots.
		// Track their precise file changes so custom app folders update locally
		// without falling back to the expensive full app index.
		appDirectories[i].trackChanges = true
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
					if errors.Is(getErr, errSkipAppIndexing) {
						a.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("skip indexing app %s: %s", appPath, getErr.Error()))
						continue
					}
					a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error getting app info for %s: %s", appPath, getErr.Error()))
					continue
				}

				a.populateAppMetadata(ctx, appPath, &info, fileInfo)

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
		a.populateAppMetadata(ctx, apps[i].Path, &apps[i], nil)
		apps[i].Icon = common.ConvertIcon(ctx, apps[i].Icon, a.pluginDirectory)
	}

	return apps
}

func (a *ApplicationPlugin) rebuildQueryEntries(ctx context.Context) {
	// Ignore rules depend on app metadata and current settings, not on the user's
	// search text, so filter them once here instead of on every query.
	entries := make([]appQueryEntry, 0, len(a.apps))
	ignoreMatchers := a.getIgnoreRuleMatchersSnapshot()
	for _, info := range a.apps {
		entry := a.buildQueryEntry(ctx, info)
		if _, ignored := a.matchIgnoreRuleCandidates(entry.ignoreCandidates, ignoreMatchers); ignored {
			continue
		}
		entries = append(entries, entry)
	}

	a.queryEntriesMutex.Lock()
	a.queryEntries = entries
	a.queryEntriesGeneration++
	a.queryEntriesMutex.Unlock()
	a.clearQuerySessionCache()

}

func (a *ApplicationPlugin) buildQueryEntry(ctx context.Context, info appInfo) appQueryEntry {
	entry := appQueryEntry{
		info: info,
	}

	if strings.HasPrefix(info.Name, "i18n:") {
		// Translated titles can change with locale, so keep the translated form in
		// the entry used by matching and ignore rules.
		displayName := a.api.GetTranslation(ctx, info.Name)
		entry.searchCandidates = info.GetSearchCandidates(displayName)
		entry.ignoreCandidates = buildIgnoreRuleCandidates(info, displayName)
		return entry
	}

	entry.searchCandidates = info.GetSearchCandidates(info.Name)
	entry.ignoreCandidates = buildIgnoreRuleCandidates(info, info.Name)
	return entry
}

func (a *ApplicationPlugin) getQueryEntriesSnapshot() ([]appQueryEntry, uint64) {
	a.queryEntriesMutex.RLock()
	entries := a.queryEntries
	generation := a.queryEntriesGeneration
	a.queryEntriesMutex.RUnlock()
	return entries, generation
}

func (a *ApplicationPlugin) resolveQueryEntryDisplay(ctx context.Context, entry appQueryEntry) (displayName string, displayPath string, searchCandidates []string) {
	displayName = entry.info.Name
	if strings.HasPrefix(displayName, "i18n:") {
		displayName = a.api.GetTranslation(ctx, displayName)
		searchCandidates = entry.info.GetSearchCandidates(displayName)
	} else {
		searchCandidates = entry.searchCandidates
	}

	displayPath = entry.info.GetDisplayPath()
	if entry.info.Type == AppTypeWindowsSetting {
		displayPath = a.api.GetTranslation(ctx, "i18n:plugin_app_windows_settings_subtitle")
	} else if isMacSystemSettingsPath(entry.info.Path) {
		displayPath = a.api.GetTranslation(ctx, "i18n:plugin_app_macos_system_settings_subtitle")
	}

	return displayName, displayPath, searchCandidates
}

func normalizeQueryCacheKey(search string) string {
	// Bug fix: query-cache reuse must preserve whitespace because fuzzy matching
	// treats spaces as real pattern characters. The previous TrimSpace-based key
	// made "qqyy " and "qqyy" identical, so deleting the trailing space reused the
	// empty match set from the spaced query instead of rescanning apps. Lowercase
	// normalization keeps case-insensitive reuse while respecting real search text.
	return strings.ToLower(search)
}

func (a *ApplicationPlugin) getReusableQueryMatches(ctx context.Context, query plugin.Query, generation uint64) ([]int, bool) {
	if query.SessionId == "" {
		return nil, false
	}

	cached, ok := a.querySessionCache.Load(query.SessionId)
	if !ok || cached.generation != generation {
		return nil, false
	}

	currentSearch := normalizeQueryCacheKey(query.Search)
	previousSearch := normalizeQueryCacheKey(cached.search)
	if previousSearch == "" {
		return nil, false
	}
	if shouldBypassPinyinQueryCache(ctx, previousSearch, currentSearch) {
		return nil, false
	}
	// Reuse only when the current query keeps growing from the same prefix and
	// still points at the same entry snapshot.
	if currentSearch == previousSearch || strings.HasPrefix(currentSearch, previousSearch) {
		return cached.matches, true
	}

	return nil, false
}

func shouldBypassPinyinQueryCache(ctx context.Context, previousSearch string, currentSearch string) bool {
	if !setting.GetSettingManager().GetWoxSetting(ctx).UsePinYin.Get() {
		return false
	}
	if !isAsciiLetterSearch(previousSearch) || !isAsciiLetterSearch(currentSearch) {
		return false
	}

	// Bug fix: pinyin matching is intentionally non-monotonic while the user is
	// typing across syllable boundaries. A Chinese title can match "xian", miss
	// "xians", and match again at "xianshi". Reusing the empty subset from the
	// intermediate query would hide the valid final match, so plain letter
	// searches must rescan when pinyin is enabled.
	return true
}

func isAsciiLetterSearch(search string) bool {
	if search == "" {
		return false
	}
	for i := 0; i < len(search); i++ {
		ch := search[i]
		if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') {
			return false
		}
	}
	return true
}

func (a *ApplicationPlugin) storeQueryMatches(query plugin.Query, cache appQuerySessionCache) {
	if query.SessionId == "" {
		return
	}

	existing, ok := a.querySessionCache.Load(query.SessionId)
	if ok && existing.startedAt > cache.startedAt {
		return
	}

	a.querySessionCache.Store(query.SessionId, cache)
}

func (a *ApplicationPlugin) clearQuerySessionCache() {
	if a.querySessionCache == nil {
		return
	}
	a.querySessionCache.Clear()
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
		ignoreMatchers := a.getIgnoreRuleMatchersSnapshot()

		matchCount := 0
		for _, entry := range appPath {
			isExtensionMatch := lo.ContainsBy(appExtensions, func(ext string) bool {
				return strings.HasSuffix(strings.ToLower(entry.Name()), fmt.Sprintf(".%s", strings.ToLower(ext)))
			})
			if isExtensionMatch {
				fullPath := filepath.Join(dir.Path, entry.Name())
				if _, ignored := a.matchIgnoreRuleCandidates([]string{fullPath}, ignoreMatchers); ignored {
					// Bug fix: IgnoreRules previously filtered query results only after the full app
					// crawl had already parsed every ignored file. Skipping matching paths here keeps
					// deterministic smoke fixtures from waiting on large default Windows directories
					// while preserving the same pattern contract used by query filtering.
					continue
				}
				appPaths = append(appPaths, fullPath)
				matchCount++

				continue
			}

			// check if it's a directory
			subDir := filepath.Join(dir.Path, entry.Name())
			isDirectory, dirErr := util.IsDirectory(subDir)
			if dirErr != nil || !isDirectory {
				continue
			}
			if _, ignored := a.matchIgnoreRuleCandidates([]string{subDir}, ignoreMatchers); ignored {
				// Bug fix: directory-wide ignore patterns such as "C:\Program Files\*" should stop
				// recursion before expensive default roots are walked. Filtering only leaf apps was
				// correct functionally but not enough for bounded smoke-test indexing.
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
	cacheContent, marshalErr := json.Marshal(appCacheFile{
		Version: appCacheVersion,
		Apps:    a.apps,
	})
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

func parseAppCacheContent(cacheContent []byte) ([]appInfo, error) {
	var cacheFile appCacheFile
	if err := json.Unmarshal(cacheContent, &cacheFile); err != nil {
		return nil, fmt.Errorf("error unmarshalling app cache file: %w", err)
	}

	if cacheFile.Version != appCacheVersion {
		return nil, fmt.Errorf("app cache version mismatch: got %d want %d", cacheFile.Version, appCacheVersion)
	}

	return cacheFile.Apps, nil
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

	apps, err := parseAppCacheContent(cacheContent)
	if err != nil {
		a.api.Log(ctx, plugin.LogLevelWarning, err.Error())
		return nil, err
	}

	for i := range apps {
		apps[i].Pid = 0
		if strings.TrimSpace(apps[i].Identity) == "" {
			apps[i].Identity = strings.TrimSpace(resolveAppIdentityForPlatform(ctx, apps[i]))
		}
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

func (a *ApplicationPlugin) handleMRURestore(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	contextData := appContextData{
		Name: mruData.ContextData["name"],
		Path: mruData.ContextData["path"],
		Type: mruData.ContextData["type"],
	}
	if contextData.Path == "" && contextData.Name == "" {
		return nil, fmt.Errorf("empty app context data")
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

	displayName := appInfo.Name
	if strings.HasPrefix(displayName, "i18n:") {
		displayName = a.api.GetTranslation(ctx, displayName)
	}

	displayPath := appInfo.GetDisplayPath()
	if appInfo.Type == AppTypeWindowsSetting {
		displayPath = a.api.GetTranslation(ctx, "i18n:plugin_app_windows_settings_subtitle")
	} else if isMacSystemSettingsPath(appInfo.Path) {
		displayPath = a.api.GetTranslation(ctx, "i18n:plugin_app_macos_system_settings_subtitle")
	}
	result := &plugin.QueryResult{
		Id:       uuid.NewString(),
		Title:    displayName,
		SubTitle: displayPath,
		Icon:     appInfo.Icon, // Use current icon instead of cached MRU icon to handle cache invalidation
		Actions:  a.buildAppActions(*appInfo, displayName, mruData.ContextData),
	}

	// Track this result for periodic refresh (refreshRunningApps will handle running state)
	a.trackedResults.Store(result.Id, *appInfo)

	return result, nil
}

func (a *ApplicationPlugin) refreshRunningApps(ctx context.Context) {
	// Skip refresh if window is hidden (for periodic updates like CPU/memory)
	if !a.api.IsVisible(ctx) {
		return
	}

	// Copy tracked results first so query writes are not blocked by the refresh walk.
	trackedResultsSnapshot := a.trackedResults.ToMap()
	if len(trackedResultsSnapshot) == 0 {
		return
	}

	type updateItem struct {
		resultId string
		app      appInfo
	}

	var toRemove []string
	var toUpdate []updateItem

	for resultId, appInfo := range trackedResultsSnapshot {
		// Try to get the result, if it returns nil, the result is no longer visible
		updatableResult := a.api.GetUpdatableResult(ctx, resultId)
		if updatableResult == nil {
			// Mark for removal from tracking queue
			toRemove = append(toRemove, resultId)
			continue
		}

		// Update Pid first (app may have been restarted with a new Pid, or started for the first time)
		currentPid := a.retriever.GetPid(ctx, appInfo)
		pidChanged := currentPid != appInfo.Pid
		appInfo.Pid = currentPid

		if pidChanged {
			// Don't call Store here (would cause deadlock), collect for later update
			toUpdate = append(toUpdate, updateItem{resultId, appInfo})
		}

		// Track if we need to update the UI
		needsUpdate := false

		// Update CPU/memory data and actions based on running state
		if appInfo.Pid > 0 {
			// App is running - update CPU/memory tails
			tails := a.getRunningProcessResult(appInfo)
			updatableResult.Tails = &tails
			needsUpdate = true // Always update when running (CPU/memory changes)

			// Add terminate action if not exists
			hasTerminateAction := false
			if updatableResult.Actions != nil {
				for _, action := range *updatableResult.Actions {
					if action.ContextData["action"] == "terminate" {
						hasTerminateAction = true
						break
					}
				}
			}

			if !hasTerminateAction {
				// Capture current Pid for the closure
				currentAppPid := appInfo.Pid
				*updatableResult.Actions = append(*updatableResult.Actions, plugin.QueryResultAction{
					Name: "i18n:plugin_app_terminate",
					Icon: common.TerminateAppIcon,
					ContextData: common.ContextData{
						"name":   appInfo.Name,
						"path":   appInfo.Path,
						"type":   appInfo.Type,
						"action": "terminate",
					},
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						// peacefully kill the process
						p, getErr := os.FindProcess(currentAppPid)
						if getErr != nil {
							a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error finding process %d: %s", currentAppPid, getErr.Error()))
							return
						}

						killErr := p.Kill()
						if killErr != nil {
							a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error killing process %d: %s", currentAppPid, killErr.Error()))
						}
					},
				})
			}
		} else if pidChanged {
			// App just stopped running - clear tails and remove terminate action
			emptyTails := []plugin.QueryResultTail{}
			updatableResult.Tails = &emptyTails
			needsUpdate = true

			// Remove terminate action if exists
			if updatableResult.Actions != nil {
				originalLen := len(*updatableResult.Actions)
				*updatableResult.Actions = lo.Filter(*updatableResult.Actions, func(action plugin.QueryResultAction, _ int) bool {
					return action.ContextData["action"] != "terminate"
				})
				// Only mark as needing update if we actually removed an action
				if len(*updatableResult.Actions) != originalLen {
					needsUpdate = true
				}
			}
		}

		// Only push update to UI if something actually changed
		if needsUpdate {
			// If UpdateResult returns false, the result is no longer visible in UI
			if !a.api.UpdateResult(ctx, *updatableResult) {
				toRemove = append(toRemove, resultId)
			}
		}
	}

	// Write back after the snapshot walk so query-time stores are not blocked by
	// refresh work.
	for _, item := range toUpdate {
		a.trackedResults.Store(item.resultId, item.app)
	}

	// Clean up results that are no longer visible
	for _, resultId := range toRemove {
		a.trackedResults.Delete(resultId)
	}
}

func GetHotkeyAppCandidates(ctx context.Context) []setting.IgnoredHotkeyApp {
	manager := plugin.GetPluginManager()
	if manager == nil {
		return []setting.IgnoredHotkeyApp{}
	}

	for _, instance := range manager.GetPluginInstances() {
		appPlugin, ok := instance.Plugin.(*ApplicationPlugin)
		if !ok {
			continue
		}

		return appPlugin.getHotkeyAppCandidates(ctx)
	}

	return []setting.IgnoredHotkeyApp{}
}

func GetUsageAppIcons(ctx context.Context, usageSubjectIds []string) map[string]common.WoxImage {
	manager := plugin.GetPluginManager()
	if manager == nil || len(usageSubjectIds) == 0 {
		return map[string]common.WoxImage{}
	}

	neededIds := map[string]struct{}{}
	for _, id := range usageSubjectIds {
		id = strings.TrimSpace(id)
		if id != "" {
			neededIds[id] = struct{}{}
		}
	}
	if len(neededIds) == 0 {
		return map[string]common.WoxImage{}
	}

	for _, instance := range manager.GetPluginInstances() {
		appPlugin, ok := instance.Plugin.(*ApplicationPlugin)
		if !ok {
			continue
		}

		icons := map[string]common.WoxImage{}
		for _, info := range appPlugin.apps {
			// Analytics stores app launches as "<type>:<name>" instead of duplicating icon payloads
			// into every event. Resolve those stable subject ids against the current app index so
			// the usage dashboard can show fresh icons without growing the analytics table.
			usageSubjectId := fmt.Sprintf("%s:%s", info.Type, info.Name)
			if _, ok := neededIds[usageSubjectId]; !ok || info.Icon.IsEmpty() {
				continue
			}
			icons[usageSubjectId] = info.Icon
		}
		return icons
	}

	return map[string]common.WoxImage{}
}

func (a *ApplicationPlugin) getHotkeyAppCandidates(ctx context.Context) []setting.IgnoredHotkeyApp {
	if len(a.hotkeyAppCandidates) == 0 && len(a.apps) > 0 {
		a.rebuildHotkeyAppCandidates(ctx)
	}

	candidates := make([]setting.IgnoredHotkeyApp, len(a.hotkeyAppCandidates))
	copy(candidates, a.hotkeyAppCandidates)
	return candidates
}

func (a *ApplicationPlugin) rebuildHotkeyAppCandidates(ctx context.Context) {
	candidates := make([]setting.IgnoredHotkeyApp, 0, len(a.apps))
	seen := make(map[string]bool)

	for _, info := range a.apps {
		if strings.TrimSpace(info.Identity) == "" {
			info.Identity = strings.TrimSpace(resolveAppIdentityForPlatform(ctx, info))
		}

		candidate, ok := a.toIgnoredHotkeyApp(info)
		if !ok {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(candidate.Identity))
		if key == "" || seen[key] {
			continue
		}

		seen[key] = true
		candidates = append(candidates, candidate)
	}

	sort.Slice(candidates, func(i, j int) bool {
		leftName := strings.ToLower(candidates[i].Name)
		rightName := strings.ToLower(candidates[j].Name)
		if leftName == rightName {
			return strings.ToLower(candidates[i].Identity) < strings.ToLower(candidates[j].Identity)
		}
		return leftName < rightName
	})

	a.hotkeyAppCandidates = candidates
}

func (a *ApplicationPlugin) toIgnoredHotkeyApp(info appInfo) (setting.IgnoredHotkeyApp, bool) {
	identity := strings.TrimSpace(info.Identity)
	if identity == "" {
		return setting.IgnoredHotkeyApp{}, false
	}

	name := strings.TrimSpace(info.Name)
	if name == "" {
		name = strings.TrimSpace(info.Path)
	}
	if name == "" {
		name = identity
	}

	icon := info.Icon
	if icon.IsEmpty() {
		icon = common.PluginAppIcon
	}

	return setting.IgnoredHotkeyApp{
		Name:     name,
		Identity: identity,
		Path:     info.Path,
		Icon:     icon,
	}, true
}
