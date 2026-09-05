package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	filesearchplugin "wox/plugin/system/file_search"
	notesplugin "wox/plugin/system/notes"
	shellplugin "wox/plugin/system/shell"
	"wox/setting"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/ui"
	"wox/util"
	"wox/util/filesearch"
	"wox/util/overlay"
	"wox/util/overlay/textoverlay"
	"wox/util/shell"
	"wox/util/window"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

type openSaveFolder struct {
	titleKey   string
	path       string
	scoreBoost int64
}

type openSaveHistoryEntry struct {
	Path   string `json:"path"`
	UsedAt int64  `json:"used_at"`
	Count  int    `json:"count"`
}

type quickJumpPathEntry struct {
	Path string `json:"Path"`
}

const (
	openSaveHistorySettingKey    = "openSaveHistory"
	enableTypeToSearchSettingKey = "enableTypeToSearch"
	quickJumpPathsSettingKey     = "quickJumpPaths"

	explorerPluginID                = "6cde8bec-3f19-44f6-8a8b-d3ba3712d04e"
	explorerCommandAdd              = "add"
	explorerDialogHintOverlayName   = "explorer_dialog_hint"
	explorerDialogHintCloseDelay    = 250 * time.Millisecond
	explorerDialogPathCacheDuration = 30 * time.Second

	// Current-folder hits keep a higher tier than global indexed scores
	// (typically a few thousand). Type-to-search still searches beyond the
	// active folder, but files the user is already looking at should rank first.
	currentDirectoryScoreBoost int64 = 100000
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ExplorerPlugin{})
}

type integrationRuntime struct {
	stopCh              chan struct{}
	typeToSearchEnabled atomic.Bool
	onExplorerActivated func(pid int)
	onDialogActivated   func(OpenSaveDialogActivatedEvent)
	onDeactivated       func()
	onExplorerKey       func(string)
	onDialogKey         func(string)
}

type openSaveDialogPathCache struct {
	pid       int
	title     string
	windowId  string
	path      string
	expiresAt time.Time
}

type ExplorerPlugin struct {
	api                    plugin.API
	integrationRuntime     atomic.Pointer[integrationRuntime]
	dialogPathCacheMu      sync.Mutex
	dialogPathCache        openSaveDialogPathCache
	dialogPathResolveGroup singleflight.Group
	dialogNavigateMu       sync.Mutex
	quickSwitch            *quickSwitchCoordinator
}

func (c *ExplorerPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "6cde8bec-3f19-44f6-8a8b-d3ba3712d04e",
		Name:          "i18n:plugin_explorer_plugin_name",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_explorer_plugin_description",
		Icon:          "emoji:📂",
		TriggerKeywords: []string{
			"explorer",
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     explorerCommandAdd,
				Description: "i18n:plugin_explorer_command_add",
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
		},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type:               definition.PluginSettingDefinitionTypeCheckBox,
				IsPlatformSpecific: true,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          enableTypeToSearchSettingKey,
					Label:        "i18n:plugin_explorer_setting_enable_type_to_search",
					Tooltip:      "i18n:plugin_explorer_setting_enable_type_to_search_tips",
					DefaultValue: "false",
				},
			},
			{
				Type:               definition.PluginSettingDefinitionTypeTable,
				IsPlatformSpecific: true,
				Value: &definition.PluginSettingValueTable{
					Key:     quickJumpPathsSettingKey,
					Title:   "i18n:plugin_explorer_setting_quick_jump_paths",
					Tooltip: "i18n:plugin_explorer_setting_quick_jump_paths_tips",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "Path",
							Label: "i18n:plugin_explorer_setting_quick_jump_path",
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
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureQueryEnv,
				Params: map[string]any{
					"requireActiveWindowName":                         true,
					"requireActiveWindowPid":                          true,
					"requireActiveWindowId":                           true,
					"requireActiveWindowIsOpenSaveDialog":             true,
					"requireActiveWindowIsOpenSaveDialogSelectFolder": true,
				},
			},
		},
	}
}

func (c *ExplorerPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
	c.quickSwitch = newQuickSwitchCoordinator(c.newQuickSwitchDeps())

	// Quick Switch always listens for Explorer/Finder → dialog transitions.
	// Type-to-search only adds raw keys and the dialog hint on top of that.
	c.startIntegrationRuntime(ctx)
	c.applyTypeToSearchSetting(ctx, c.api.GetSetting(ctx, enableTypeToSearchSettingKey) == "true")

	c.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key == enableTypeToSearchSettingKey {
			c.applyTypeToSearchSetting(callbackCtx, value == "true")
		}
	})
	c.api.OnUnload(ctx, func(context.Context) {
		c.stopIntegrationRuntime()
	})
}

func (c *ExplorerPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if !c.shouldHandleQuery(ctx, query) {
		return plugin.QueryResponse{}
	}

	if strings.EqualFold(query.Command, explorerCommandAdd) {
		return plugin.NewQueryResponse(c.queryAddQuickJumpPath(ctx, query))
	}

	return plugin.NewQueryResponse(c.queryExplorerResults(ctx, query))
}

func (c *ExplorerPlugin) shouldHandleQuery(ctx context.Context, query plugin.Query) bool {
	if !query.IsGlobalQuery() {
		return true
	}

	if query.Env.ActiveWindowIsOpenSaveDialog {
		return true
	}

	isFileExplorer, err := window.IsFileExplorer(query.Env.ActiveWindowPid)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, "Failed to check if active app is file explorer: "+err.Error())
		return false
	}
	return isFileExplorer
}

func (c *ExplorerPlugin) queryExplorerResults(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	search := strings.TrimSpace(query.Search)
	var directoryResults []plugin.QueryResult
	if search == "" {
		directoryResults = c.queryCurrentDirectoryEntries(ctx, query)
	} else if indexedResults, ok := c.queryFileSearchResults(ctx, query, search); ok {
		directoryResults = c.prioritizeCurrentDirectoryHits(ctx, query, indexedResults)
	} else {
		directoryResults = c.queryCurrentDirectoryEntries(ctx, query)
	}
	jumpResults := c.queryJumpFolders(ctx, query)

	results := make([]plugin.QueryResult, 0, len(directoryResults)+len(jumpResults))
	results = append(results, directoryResults...)
	results = append(results, jumpResults...)
	return results
}

// prioritizeCurrentDirectoryHits boosts and backfills matches that live in the
// active Explorer/Finder folder so they outrank otherwise-equal global hits.
func (c *ExplorerPlugin) prioritizeCurrentDirectoryHits(ctx context.Context, query plugin.Query, indexedResults []plugin.QueryResult) []plugin.QueryResult {
	currentPath := c.getCurrentFileExplorerPath(ctx, query.Env)
	var localResults []plugin.QueryResult
	if currentPath != "" {
		localResults = c.queryDirectoryEntriesAtPath(ctx, query, currentPath, strings.TrimSpace(query.Search))
	}
	return mergeCurrentDirectoryResults(indexedResults, localResults, currentPath, c.normalizePathKey)
}

// queryFileSearchResults converts global indexed results into Explorer-specific actions.
func (c *ExplorerPlugin) queryFileSearchResults(ctx context.Context, query plugin.Query, search string) ([]plugin.QueryResult, bool) {
	folderOnly := query.Env.ActiveWindowIsOpenSaveDialogSelectFolder
	commandData := common.ContextData{
		filesearchplugin.PluginCommandDataQuery: search,
	}
	// Prefer source-side filtering so folder-only dialogs do not pull file hits.
	if folderOnly {
		commandData[filesearchplugin.PluginCommandDataEntryType] = filesearchplugin.PluginCommandEntryTypeFolder
	}
	c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer global file search: search=%q isOpenSaveDialogSelectFolder=%v", search, folderOnly))
	commandResult, err := c.api.InvokePluginCommand(ctx, plugin.PluginCommandRequest{
		PluginId: filesearchplugin.PluginID,
		Command:  filesearchplugin.PluginCommandSearch,
		Data:     commandData,
	})
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, "Explorer global file search failed: "+err.Error())
		return nil, false
	}
	if !commandResult.Handled || commandResult.Message != "" {
		if commandResult.Message != "" {
			c.api.Log(ctx, plugin.LogLevelWarning, "Explorer global file search failed: "+commandResult.Message)
		}
		return nil, false
	}

	var indexedResults []filesearch.SearchResult
	if err := json.Unmarshal([]byte(commandResult.Data[filesearchplugin.PluginCommandResultDataResults]), &indexedResults); err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, "Explorer global file search result decode failed: "+err.Error())
		return nil, false
	}

	results := make([]plugin.QueryResult, 0, len(indexedResults))
	for _, item := range indexedResults {
		if folderOnly && !item.IsDir {
			continue
		}
		icon := common.NewWoxImageLazyLoadCandidate(common.NewWoxImageFileIcon(item.Path), common.ResultListIconSize)
		if item.IsDir {
			icon = common.FolderIcon
			if util.IsMacOS() && strings.HasSuffix(strings.ToLower(item.Name), ".app") {
				icon = common.NewWoxImageLazyLoadCandidate(common.NewWoxImageFileIcon(item.Path), common.ResultListIconSize)
			}
		}
		results = append(results, c.buildDirectoryEntryResult(query, item.Name, item.Path, item.IsDir, icon, item.Score, true))
	}
	return results, true
}

func (c *ExplorerPlugin) queryAddQuickJumpPath(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	path := c.resolveAddPath(ctx, query)
	if path == "" {
		c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Explorer add skipped: no resolvable path (pid=%d, title=%q, isOpenSaveDialog=%v, isOpenSaveDialogSelectFolder=%v)", query.Env.ActiveWindowPid, query.Env.ActiveWindowTitle, query.Env.ActiveWindowIsOpenSaveDialog, query.Env.ActiveWindowIsOpenSaveDialogSelectFolder))
		return []plugin.QueryResult{}
	}

	path = filepath.Clean(path)
	if !c.isDirPath(path) {
		return []plugin.QueryResult{}
	}

	return []plugin.QueryResult{
		{
			Title:    "i18n:plugin_explorer_add_quick_jump_title",
			SubTitle: path,
			Icon:     common.FolderIcon,
			Score:    200,
			Actions: []plugin.QueryResultAction{
				{
					Name:      "i18n:ui_add",
					IsDefault: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						if c.addQuickJumpPath(ctx, path) {
							c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
						}
					},
				},
			},
		},
	}
}

func (c *ExplorerPlugin) resolveAddPath(ctx context.Context, query plugin.Query) string {
	if query.Command != "" {
		if commandPath := strings.TrimSpace(query.Search); commandPath != "" {
			return commandPath
		}
	}

	return c.getCurrentFileExplorerPath(ctx, query.Env)
}

func (c *ExplorerPlugin) queryCurrentDirectoryEntries(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	currentPath := c.getCurrentFileExplorerPath(ctx, query.Env)
	if currentPath == "" {
		c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Explorer current directory query skipped: path not found (search=%q, pid=%d, title=%q, isOpenSaveDialog=%v, isOpenSaveDialogSelectFolder=%v)", query.Search, query.Env.ActiveWindowPid, query.Env.ActiveWindowTitle, query.Env.ActiveWindowIsOpenSaveDialog, query.Env.ActiveWindowIsOpenSaveDialogSelectFolder))
		return []plugin.QueryResult{}
	}
	return c.queryDirectoryEntriesAtPath(ctx, query, currentPath, strings.TrimSpace(query.Search))
}

func (c *ExplorerPlugin) queryDirectoryEntriesAtPath(ctx context.Context, query plugin.Query, dirPath string, search string) []plugin.QueryResult {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to read directory: path=%q err=%s", dirPath, err.Error()))
		return []plugin.QueryResult{}
	}

	folderOnly := query.Env.ActiveWindowIsOpenSaveDialogSelectFolder
	results := make([]plugin.QueryResult, 0, len(entries))
	for _, entry := range entries {
		isDir := entry.IsDir()
		// Folder-pick dialogs only need directories; skip files at the source.
		if folderOnly && !isDir {
			continue
		}

		isMatch := true
		var matchScore int64

		if search != "" {
			isMatch, matchScore = plugin.IsStringMatchScore(ctx, entry.Name(), search)
		}

		if !isMatch {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		var icon common.WoxImage
		if isDir {
			icon = common.FolderIcon

			// On macOS, use the .app icon for application bundles
			if util.IsMacOS() && strings.HasSuffix(strings.ToLower(entry.Name()), ".app") {
				icon = common.NewWoxImageFileIcon(fullPath)
			}
		} else {
			icon = common.NewWoxImageFileIcon(fullPath)
		}

		results = append(results, c.buildDirectoryEntryResult(query, entry.Name(), fullPath, isDir, icon, matchScore, false))
	}

	return results
}

func (c *ExplorerPlugin) buildDirectoryEntryResult(query plugin.Query, title string, fullPath string, isDir bool, icon common.WoxImage, score int64, isGlobalResult bool) plugin.QueryResult {
	defaultAction := plugin.QueryResultAction{
		Name:                   "i18n:plugin_explorer_reveal_in_explorer",
		IsDefault:              true,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			c.revealEntry(ctx, query.Env, fullPath, isDir, isGlobalResult)
		},
	}
	if isDir {
		defaultAction.Name = "i18n:plugin_explorer_jump_to"
		defaultAction.Action = func(ctx context.Context, actionContext plugin.ActionContext) {
			c.jumpToFolder(ctx, query.Env, fullPath)
		}
	}

	return plugin.QueryResult{
		Title:    title,
		SubTitle: fullPath,
		Icon:     icon,
		Score:    score,
		Actions: []plugin.QueryResultAction{
			{
				Name: "i18n:plugin_explorer_open",
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					shell.Open(fullPath)
				},
			},
			c.buildExecuteCommandAtLocationAction(fullPath, isDir),
			notesplugin.CreateNoteAction(c.api, "", "", fullPath),
			defaultAction,
		},
	}
}

// buildExecuteCommandAtLocationAction opens Shell with the selected location as its working directory.
func (c *ExplorerPlugin) buildExecuteCommandAtLocationAction(path string, isDir bool) plugin.QueryResultAction {
	return shellplugin.PrepareCommandAtDirectoryAction(c.api, path, isDir)
}

func (c *ExplorerPlugin) revealEntry(ctx context.Context, env plugin.QueryEnv, fullPath string, isDir bool, isGlobalResult bool) {
	if env.ActiveWindowIsOpenSaveDialog {
		entryPath := strings.TrimSpace(fullPath)
		if entryPath == "" {
			c.api.Log(ctx, plugin.LogLevelError, "Reveal entry in open/save failed: empty entry path")
			return
		}

		if isGlobalResult && !isDir && !c.navigateFileDialog(ctx, env, filepath.Dir(entryPath)) {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Navigate open/save dialog to entry parent failed: pid=%d entry=%q", env.ActiveWindowPid, entryPath))
			return
		}

		// Keep the exact dialog target after Wox changes foreground focus.
		if selectFileDialogItemWithHook(ctx, env.ActiveWindowId, env.ActiveWindowPid, entryPath, isGlobalResult && !isDir) {
			c.api.HideApp(ctx)
			return
		}
		if window.SelectInFileDialog(env.ActiveWindowId, env.ActiveWindowPid, entryPath) {
			util.Go(ctx, "highlight open/save dialog entry", func() {
				window.HighlightInFileDialog(env.ActiveWindowId, env.ActiveWindowPid, entryPath)
			})
			c.api.HideApp(ctx)
			return
		}

		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Select entry in open/save failed: pid=%d entry=%q isDir=%v", env.ActiveWindowPid, entryPath, isDir))
		return
	}

	if isGlobalResult && !isDir {
		if !window.NavigateInFileExplorer(env.ActiveWindowPid, filepath.Dir(fullPath), env.ActiveWindowTitle, env.ActiveWindowId) {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Navigate explorer to entry parent failed: pid=%d path=%s", env.ActiveWindowPid, fullPath))
			return
		}
	}

	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Select in explorer by pid: pid=%d path=%s", env.ActiveWindowPid, fullPath))
	selectionDeadline := time.Now()
	if isGlobalResult && !isDir {
		// ShellWindows updates the navigated tab asynchronously; select as soon as its new document is ready.
		selectionDeadline = selectionDeadline.Add(250 * time.Millisecond)
	}
	for {
		if window.SelectInFileExplorer(env.ActiveWindowPid, fullPath, env.ActiveWindowTitle, env.ActiveWindowId) {
			c.api.HideApp(ctx)
			return
		}
		if time.Now().After(selectionDeadline) {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Select in explorer by pid failed: pid=%d path=%s", env.ActiveWindowPid, fullPath))
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
}

func (c *ExplorerPlugin) queryJumpFolders(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	folders := c.getJumpFolderCandidates(ctx)
	if len(folders) == 0 {
		return []plugin.QueryResult{}
	}

	search := strings.TrimSpace(query.Search)
	var results []plugin.QueryResult
	for _, folder := range folders {
		title := i18n.GetI18nManager().TranslateWox(ctx, folder.titleKey)

		isMatch := true
		matchScore := int64(0)
		if search != "" {
			isMatch, matchScore = plugin.IsStringMatchScore(ctx, title, search)
			if !isMatch {
				// Try matching path
				isMatch, matchScore = plugin.IsStringMatchScore(ctx, folder.path, search)
			}
		}
		if !isMatch {
			continue
		}

		score := matchScore + folder.scoreBoost
		results = append(results, c.buildJumpFolderResult(query, title, folder.path, score))
	}

	return results
}

func (c *ExplorerPlugin) buildJumpFolderResult(query plugin.Query, title string, folderPath string, score int64) plugin.QueryResult {
	return plugin.QueryResult{
		Title:    title,
		SubTitle: folderPath,
		Icon:     common.FolderIcon,
		Score:    score,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_explorer_jump_to",
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					c.jumpToFolder(ctx, query.Env, folderPath)
				},
			},
			c.buildExecuteCommandAtLocationAction(folderPath, true),
			notesplugin.CreateNoteAction(c.api, "", "", folderPath),
		},
	}
}

func (c *ExplorerPlugin) getJumpFolderCandidates(ctx context.Context) []openSaveFolder {
	candidateIndex := make(map[string]int)
	candidates := make([]openSaveFolder, 0)

	addCandidate := func(candidate openSaveFolder) {
		candidate.path = strings.TrimSpace(candidate.path)
		if candidate.path == "" || !c.isDirPath(candidate.path) {
			return
		}
		candidate.path = filepath.Clean(candidate.path)
		if candidate.titleKey == "" {
			candidate.titleKey = filepath.Base(candidate.path)
		}

		key := c.normalizePathKey(candidate.path)
		if index, ok := candidateIndex[key]; ok {
			if candidate.scoreBoost > candidates[index].scoreBoost {
				candidates[index].scoreBoost = candidate.scoreBoost
			}
			if candidates[index].titleKey == "" && candidate.titleKey != "" {
				candidates[index].titleKey = candidate.titleKey
			}
			return
		}

		candidateIndex[key] = len(candidates)
		candidates = append(candidates, candidate)
	}

	// 1) User managed quick jump paths.
	for _, quickJumpPath := range c.loadQuickJumpPaths(ctx) {
		addCandidate(openSaveFolder{
			titleKey:   filepath.Base(quickJumpPath),
			path:       quickJumpPath,
			scoreBoost: 300,
		})
	}

	// 2) Common system folders.
	homeDir, err := os.UserHomeDir()
	if err == nil {
		systemFolders := []struct {
			titleKey string
			path     string
		}{
			{titleKey: "i18n:plugin_explorer_common_folder_home", path: homeDir},
			{titleKey: "i18n:plugin_explorer_common_folder_desktop", path: filepath.Join(homeDir, "Desktop")},
			{titleKey: "i18n:plugin_explorer_common_folder_documents", path: filepath.Join(homeDir, "Documents")},
			{titleKey: "i18n:plugin_explorer_common_folder_downloads", path: filepath.Join(homeDir, "Downloads")},
			{titleKey: "i18n:plugin_explorer_common_folder_pictures", path: filepath.Join(homeDir, "Pictures")},
			{titleKey: "i18n:plugin_explorer_common_folder_music", path: filepath.Join(homeDir, "Music")},
			{titleKey: "i18n:plugin_explorer_common_folder_videos", path: filepath.Join(homeDir, "Videos")},
		}
		for _, folder := range systemFolders {
			addCandidate(openSaveFolder{
				titleKey: folder.titleKey,
				path:     folder.path,
			})
		}
	}

	return candidates
}

// getCachedOpenSaveDialogPath returns a recently resolved dialog folder for fast typing in the same dialog query session.
func (c *ExplorerPlugin) getCachedOpenSaveDialogPath(pid int, title string, windowId string) (string, bool) {
	now := time.Now()
	c.dialogPathCacheMu.Lock()
	defer c.dialogPathCacheMu.Unlock()

	sameDialog := c.dialogPathCache.pid == pid && c.dialogPathCache.path != ""
	if sameDialog {
		if windowId != "" || c.dialogPathCache.windowId != "" {
			sameDialog = c.dialogPathCache.windowId == windowId
		} else {
			sameDialog = c.dialogPathCache.title == title
		}
	}
	if !sameDialog || now.After(c.dialogPathCache.expiresAt) {
		return "", false
	}

	info, err := os.Stat(c.dialogPathCache.path)
	if err != nil || !info.IsDir() {
		c.dialogPathCache = openSaveDialogPathCache{}
		return "", false
	}

	c.dialogPathCache.expiresAt = now.Add(explorerDialogPathCacheDuration)
	return c.dialogPathCache.path, true
}

// setCachedOpenSaveDialogPath stores the slow UIA fallback result so subsequent query changes avoid re-reading the dialog tree.
func (c *ExplorerPlugin) setCachedOpenSaveDialogPath(pid int, title string, windowId string, path string) {
	path = strings.TrimSpace(path)
	if pid <= 0 || path == "" {
		return
	}

	c.dialogPathCacheMu.Lock()
	defer c.dialogPathCacheMu.Unlock()
	c.dialogPathCache = openSaveDialogPathCache{
		pid:       pid,
		title:     title,
		windowId:  windowId,
		path:      path,
		expiresAt: time.Now().Add(explorerDialogPathCacheDuration),
	}
}

// clearOpenSaveDialogPathCache drops stale dialog paths when a new hint-driven query session starts.
func (c *ExplorerPlugin) clearOpenSaveDialogPathCache(pid int) {
	c.dialogPathCacheMu.Lock()
	defer c.dialogPathCacheMu.Unlock()
	if pid <= 0 || c.dialogPathCache.pid == pid {
		c.dialogPathCache = openSaveDialogPathCache{}
	}
}

func (c *ExplorerPlugin) resolveOpenSaveDialogPath(ctx context.Context, env plugin.QueryEnv) string {
	if cachedPath, ok := c.getCachedOpenSaveDialogPath(env.ActiveWindowPid, env.ActiveWindowTitle, env.ActiveWindowId); ok {
		return cachedPath
	}

	key := fmt.Sprintf("%d:%s:%s", env.ActiveWindowPid, env.ActiveWindowTitle, env.ActiveWindowId)
	value, _, _ := c.dialogPathResolveGroup.Do(key, func() (any, error) {
		if cachedPath, ok := c.getCachedOpenSaveDialogPath(env.ActiveWindowPid, env.ActiveWindowTitle, env.ActiveWindowId); ok {
			return cachedPath, nil
		}

		if strings.TrimSpace(env.ActiveWindowId) != "" {
			if dialogPath := strings.TrimSpace(window.GetFileDialogPathByWindowId(env.ActiveWindowId, env.ActiveWindowPid)); dialogPath != "" {
				c.setCachedOpenSaveDialogPath(env.ActiveWindowPid, env.ActiveWindowTitle, env.ActiveWindowId, dialogPath)
				return dialogPath, nil
			}
			return "", nil
		}

		if dialogPath := strings.TrimSpace(window.GetFileDialogPathByPid(env.ActiveWindowPid)); dialogPath != "" {
			c.setCachedOpenSaveDialogPath(env.ActiveWindowPid, env.ActiveWindowTitle, env.ActiveWindowId, dialogPath)
			return dialogPath, nil
		}

		if dialogPath := strings.TrimSpace(window.GetActiveFileDialogPath()); dialogPath != "" {
			c.setCachedOpenSaveDialogPath(env.ActiveWindowPid, env.ActiveWindowTitle, env.ActiveWindowId, dialogPath)
			return dialogPath, nil
		}
		return "", nil
	})

	dialogPath, _ := value.(string)
	dialogPath = strings.TrimSpace(dialogPath)
	return dialogPath
}

// prefetchOpenSaveDialogPath resolves the dialog folder while the hint is visible, hiding the slow fallback from the first typed query.
func (c *ExplorerPlugin) prefetchOpenSaveDialogPath(ctx context.Context, pid int, title string, windowId string) {
	if pid <= 0 {
		return
	}

	util.Go(ctx, "prefetch open/save dialog path", func() {
		c.resolveOpenSaveDialogPath(ctx, plugin.QueryEnv{
			ActiveWindowPid:              pid,
			ActiveWindowTitle:            title,
			ActiveWindowId:               windowId,
			ActiveWindowIsOpenSaveDialog: true,
		})
	})
}

func (c *ExplorerPlugin) getCurrentFileExplorerPath(ctx context.Context, env plugin.QueryEnv) string {
	isFileExplorerPid := false
	if env.ActiveWindowPid > 0 {
		if ok, err := window.IsFileExplorer(env.ActiveWindowPid); err != nil {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer failed to detect file explorer pid=%d: %s", env.ActiveWindowPid, err.Error()))
		} else {
			isFileExplorerPid = ok
		}
	}

	isOpenSaveDialog := env.ActiveWindowIsOpenSaveDialog
	shouldUseDialogPath := isOpenSaveDialog && !isFileExplorerPid
	if isOpenSaveDialog && isFileExplorerPid {
		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer ignored open/save flag for finder pid=%d", env.ActiveWindowPid))
	}

	if shouldUseDialogPath {
		if dialogPath := c.resolveOpenSaveDialogPath(ctx, env); dialogPath != "" {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer path resolved from file dialog by pid: pid=%d path=%q", env.ActiveWindowPid, dialogPath))
			return dialogPath
		}
	}

	if util.IsMacOS() {
		activePathProbe := strings.TrimSpace(window.GetActiveFileExplorerPath())
		pidPathProbe := strings.TrimSpace(window.GetFileExplorerPathByPid(env.ActiveWindowPid))

		if activePathProbe != "" {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer path resolved from active file explorer: path=%q", activePathProbe))
			return activePathProbe
		}
		if pidPathProbe != "" {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer path resolved from file explorer by pid: pid=%d path=%q", env.ActiveWindowPid, pidPathProbe))
			return pidPathProbe
		}
	}

	if util.IsWindows() {
		// Prefer the actual foreground tab path on Windows 11 (tabs may share the same HWND).
		if activePath := strings.TrimSpace(window.GetActiveFileExplorerPath()); activePath != "" {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer path resolved from active file explorer: path=%q", activePath))
			return activePath
		}

		if pidPath := strings.TrimSpace(window.GetFileExplorerPathByPidAndWindowTitle(env.ActiveWindowPid, env.ActiveWindowTitle)); pidPath != "" {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer path resolved from file explorer by pid/title: pid=%d title=%q path=%q", env.ActiveWindowPid, env.ActiveWindowTitle, pidPath))
			return pidPath
		}
	}

	// Compatibility fallback for edge cases where open/save detection flag is false
	// but the active PID still owns an open/save dialog.
	if util.IsWindows() && !shouldUseDialogPath {
		if dialogPath := c.resolveOpenSaveDialogPath(ctx, env); dialogPath != "" {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer path resolved from file dialog compatibility fallback: pid=%d path=%q", env.ActiveWindowPid, dialogPath))
			return dialogPath
		}
	}

	c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Explorer path resolve failed: pid=%d title=%q isOpenSaveDialog=%v", env.ActiveWindowPid, env.ActiveWindowTitle, isOpenSaveDialog))
	return ""
}

func (c *ExplorerPlugin) jumpToFolder(ctx context.Context, env plugin.QueryEnv, folderPath string) {
	startedAt := time.Now()
	if env.ActiveWindowIsOpenSaveDialog {
		if !c.navigateFileDialog(ctx, env, folderPath) {
			c.api.Log(ctx, plugin.LogLevelError, "Failed to navigate open/save dialog to path: "+folderPath)
			c.clearOpenSaveDialogPathCache(env.ActiveWindowPid)
			return
		}
		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Navigate open/save dialog succeeded: pid=%d windowId=%q path=%s elapsedMs=%d", env.ActiveWindowPid, env.ActiveWindowId, folderPath, time.Since(startedAt).Milliseconds()))
		c.setCachedOpenSaveDialogPath(env.ActiveWindowPid, env.ActiveWindowTitle, env.ActiveWindowId, folderPath)
		c.api.HideApp(ctx)
		return
	}

	if window.NavigateInFileExplorer(env.ActiveWindowPid, folderPath, env.ActiveWindowTitle, env.ActiveWindowId) {
		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Navigate explorer succeeded: pid=%d windowId=%q path=%s elapsedMs=%d", env.ActiveWindowPid, env.ActiveWindowId, folderPath, time.Since(startedAt).Milliseconds()))
		c.api.HideApp(ctx)
		return
	}

	if env.ActiveWindowPid > 0 && window.SelectInFileExplorer(env.ActiveWindowPid, folderPath, env.ActiveWindowTitle, env.ActiveWindowId) {
		c.api.HideApp(ctx)
		return
	}

	c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Explorer navigation and selection failed; opening folder through Shell: pid=%d windowId=%q path=%s", env.ActiveWindowPid, env.ActiveWindowId, folderPath))
	shell.Open(folderPath)
	c.api.HideApp(ctx)
}

// navigateFileDialog prefers the in-process Shell browser route and keeps the existing automation path as a compatibility fallback.
func (c *ExplorerPlugin) navigateFileDialog(ctx context.Context, env plugin.QueryEnv, folderPath string) bool {
	c.quickSwitch.Invalidate()
	return c.performFileDialogNavigation(ctx, env.ActiveWindowId, env.ActiveWindowPid, folderPath)
}

func (c *ExplorerPlugin) addQuickJumpPath(ctx context.Context, path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || !c.isDirPath(path) {
		return false
	}

	paths := c.loadQuickJumpPaths(ctx)
	targetKey := c.normalizePathKey(path)
	for _, item := range paths {
		if c.normalizePathKey(item) == targetKey {
			return false
		}
	}

	paths = append(paths, path)
	if !c.saveQuickJumpPaths(ctx, paths) {
		return false
	}

	return true
}

func (c *ExplorerPlugin) loadQuickJumpPaths(ctx context.Context) []string {
	raw := c.api.GetSetting(ctx, quickJumpPathsSettingKey)
	if raw == "" {
		return []string{}
	}

	var entries []quickJumpPathEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		c.api.Log(ctx, plugin.LogLevelError, "Failed to unmarshal quick jump paths: "+err.Error())
		return []string{}
	}

	result := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		path := strings.TrimSpace(entry.Path)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		key := c.normalizePathKey(path)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, path)
	}

	return result
}

func (c *ExplorerPlugin) saveQuickJumpPaths(ctx context.Context, paths []string) bool {
	entries := make([]quickJumpPathEntry, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		entries = append(entries, quickJumpPathEntry{
			Path: filepath.Clean(path),
		})
	}

	payload, err := json.Marshal(entries)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, "Failed to marshal quick jump paths: "+err.Error())
		return false
	}

	c.api.SaveSetting(ctx, quickJumpPathsSettingKey, string(payload), true)
	return true
}

func (c *ExplorerPlugin) isDirPath(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (c *ExplorerPlugin) normalizePathKey(path string) string {
	path = filepath.Clean(path)
	if util.IsWindows() {
		return strings.ToLower(path)
	}
	return path
}

// mergeCurrentDirectoryResults ranks direct children of currentDir above global
// indexed hits and inserts local matches the index missed. Nested paths keep
// their original global scores so only the folder the user is viewing is boosted.
func mergeCurrentDirectoryResults(indexedResults []plugin.QueryResult, localResults []plugin.QueryResult, currentDir string, normalize func(string) string) []plugin.QueryResult {
	localByPath := make(map[string]plugin.QueryResult, len(localResults))
	for _, item := range localResults {
		localByPath[normalize(item.SubTitle)] = item
	}

	seen := make(map[string]bool, len(indexedResults)+len(localResults))
	results := make([]plugin.QueryResult, 0, len(indexedResults)+len(localResults))
	for _, item := range indexedResults {
		pathKey := normalize(item.SubTitle)
		seen[pathKey] = true
		if !isDirectChildPath(item.SubTitle, currentDir, normalize) {
			results = append(results, item)
			continue
		}

		// Prefer the local row so reveal/select stays in the current folder.
		if localItem, ok := localByPath[pathKey]; ok {
			if item.Score > localItem.Score {
				localItem.Score = item.Score
			}
			localItem.Score += currentDirectoryScoreBoost
			results = append(results, markCurrentDirectoryResult(localItem))
			continue
		}

		item.Score += currentDirectoryScoreBoost
		results = append(results, markCurrentDirectoryResult(item))
	}

	for _, item := range localResults {
		pathKey := normalize(item.SubTitle)
		if seen[pathKey] {
			continue
		}
		item.Score += currentDirectoryScoreBoost
		results = append(results, markCurrentDirectoryResult(item))
	}
	return results
}

// markCurrentDirectoryResult labels current-folder hits so they stay visually
// distinct from global indexed results in the same list.
func markCurrentDirectoryResult(result plugin.QueryResult) plugin.QueryResult {
	result.Tails = append([]plugin.QueryResultTail{newCurrentDirectoryTail()}, result.Tails...)
	return result
}

func newCurrentDirectoryTail() plugin.QueryResultTail {
	tail := plugin.NewQueryResultTailText("i18n:plugin_explorer_result_tail_current_folder")
	tail.Tooltip = "i18n:plugin_explorer_result_tail_current_folder_tooltip"
	return tail
}

func isDirectChildPath(path string, dir string, normalize func(string) string) bool {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(dir) == "" {
		return false
	}
	parent := filepath.Dir(filepath.Clean(path))
	return normalize(parent) == normalize(filepath.Clean(dir))
}

func (c *ExplorerPlugin) typeToSearchDebugLog(ctx context.Context, format string, args ...any) {
	// c.api.Log(ctx, plugin.LogLevelDebug, "typeToSearch: "+fmt.Sprintf(format, args...))
}

// applyTypeToSearchSetting reconfigures keyboard and hint features without stopping Quick Switch.
func (c *ExplorerPlugin) applyTypeToSearchSetting(ctx context.Context, enabled bool) {
	runtime := c.integrationRuntime.Load()
	if runtime != nil {
		runtime.typeToSearchEnabled.Store(enabled)
	}
	// File selection still uses the Shell hook only while type-to-search can drive it.
	// Dialog folder navigation no longer depends on this flag.
	setExplorerDialogHookEnabled(enabled)
	c.registerIntegrationMonitors()
	if !enabled {
		overlay.Close(explorerDialogHintOverlayName)
		overlay.Close("explorer_hint")
	}
}

// registerIntegrationMonitors keeps foreground listeners on and attaches raw keys only for type-to-search.
func (c *ExplorerPlugin) registerIntegrationMonitors() {
	runtime := c.integrationRuntime.Load()
	if runtime == nil {
		return
	}

	var explorerKey func(string)
	var dialogKey func(string)
	if runtime.typeToSearchEnabled.Load() {
		explorerKey = runtime.onExplorerKey
		dialogKey = runtime.onDialogKey
	}
	StartExplorerMonitor(runtime.onExplorerActivated, runtime.onDeactivated, explorerKey)
	StartExplorerOpenSaveMonitor(runtime.onDialogActivated, runtime.onDeactivated, dialogKey)
}

// stopIntegrationRuntime releases listeners and cancels pending work when the plugin is disabled.
func (c *ExplorerPlugin) stopIntegrationRuntime() {
	runtime := c.integrationRuntime.Swap(nil)
	if runtime == nil {
		return
	}

	if c.quickSwitch != nil {
		c.quickSwitch.Invalidate()
	}
	StopExplorerMonitor()
	StopExplorerOpenSaveMonitor()
	setExplorerDialogHookEnabled(false)
	setExplorerMonitorLogger(nil)
	overlay.Close(explorerDialogHintOverlayName)
	overlay.Close("explorer_hint")
	c.clearOpenSaveDialogPathCache(0)
	close(runtime.stopCh)
}

// startIntegrationRuntime starts Explorer/Finder and dialog foreground listeners once.
func (c *ExplorerPlugin) startIntegrationRuntime(ctx context.Context) {
	if c.integrationRuntime.Load() != nil {
		return
	}

	setExplorerMonitorLogger(func(msg string) {
		c.typeToSearchDebugLog(ctx, "%s", msg)
	})
	c.typeToSearchDebugLog(ctx, "start integration runtime")

	runtime := &integrationRuntime{stopCh: make(chan struct{})}
	c.integrationRuntime.Store(runtime)

	type overlayEventType int
	const (
		overlayEventActivate overlayEventType = iota
		overlayEventDeactivate
		overlayEventKey
		overlayEventOpenDialogSearch
	)

	type overlayEvent struct {
		eventType        overlayEventType
		key              string
		ctx              context.Context
		pid              int
		windowID         string
		isDialog         bool
		previousExplorer *ExplorerWindowRef
	}

	events := make(chan overlayEvent, 64)
	pushEvent := func(ev overlayEvent) {
		select {
		case events <- ev:
		default:
		}
	}

	onActivated := func(pid int) {
		c.typeToSearchDebugLog(ctx, "activated pid=%d", pid)
		pushEvent(overlayEvent{eventType: overlayEventActivate, pid: pid})
	}
	onDialogActivated := func(event OpenSaveDialogActivatedEvent) {
		pushEvent(overlayEvent{
			eventType:        overlayEventActivate,
			pid:              event.Pid,
			windowID:         event.WindowID,
			isDialog:         true,
			previousExplorer: event.PreviousExplorer,
		})
	}
	onDeactivated := func() {
		c.typeToSearchDebugLog(ctx, "deactivated")
		pushEvent(overlayEvent{eventType: overlayEventDeactivate})
	}
	onKey := func(key string) {
		traceCtx := context.WithValue(ctx, util.ContextKeyTraceId, uuid.NewString())
		traceCtx = util.WithCoreSessionContext(traceCtx)
		pushEvent(overlayEvent{eventType: overlayEventKey, key: key, ctx: traceCtx})
	}
	onDialogKey := func(key string) {
		if key != explorerOpenSearchEventKey {
			onKey(key)
			return
		}
		traceCtx := context.WithValue(ctx, util.ContextKeyTraceId, uuid.NewString())
		traceCtx = util.WithCoreSessionContext(traceCtx)
		pushEvent(overlayEvent{eventType: overlayEventOpenDialogSearch, ctx: traceCtx})
	}

	go func() {
		var (
			active       bool
			activePid    int
			handoffUntil time.Time
			pending      string
			pendingCtx   context.Context
			explorerShow *common.ShowContext
		)

		inHandoff := func() bool {
			return !handoffUntil.IsZero() && time.Now().Before(handoffUntil)
		}

		changeExplorerQuery := func(localCtx context.Context) {
			if pending == "" {
				return
			}
			if explorerShow == nil {
				return
			}
			c.typeToSearchDebugLog(localCtx, "openExplorerInstance %q", pending)
			plugin.GetPluginManager().GetUI().OpenWoxInstance(localCtx, common.OpenWoxInstanceRequest{
				Role:         common.WoxInstanceRoleSecondary,
				InstanceName: string(common.ShowSourceExplorer),
				Query: common.PlainQuery{
					QueryId:   uuid.NewString(),
					QueryType: plugin.QueryTypeInput,
					QueryText: pending,
					QueryScope: common.QueryScope{
						Plugins: []common.QueryScopePlugin{{PluginID: explorerPluginID}},
					},
				},
				ShowApp: *explorerShow,
			})
		}

		// Open/save dialogs default to the filename input. Keep that native focus
		// intact unless the user explicitly clicks the Wox hint.
		openDialogQuery := func(localCtx context.Context, pid int) {
			if pid <= 0 {
				return
			}
			if isDialog, err := window.IsOpenSaveDialogByPid(pid); err != nil || !isDialog {
				return
			}

			dialogWindowId := GetOpenSaveDialogWindowIdByPid(pid)
			x, y, w, h, ok := GetOpenSaveDialogRectByPid(pid)
			if !ok || w <= 0 || h <= 0 {
				return
			}

			overlay.Close(explorerDialogHintOverlayName)
			woxSetting := setting.GetSettingManager().GetWoxSetting(localCtx)
			initialWindowHeight := getExplorerInitialWindowHeight(localCtx)
			position := getExplorerWindowPosition(common.WindowRect{X: x, Y: y, Width: w, Height: h}, woxSetting.AppWidth.Get()/2, initialWindowHeight)
			showContext := common.ShowContext{
				HideToolbar:          true,
				QueryBoxAtBottom:     true,
				HideOnBlur:           true,
				ShowSource:           common.ShowSourceExplorer,
				WindowPosition:       &position,
				WindowPositionHeight: initialWindowHeight,
				WindowWidth:          woxSetting.AppWidth.Get() / 2,
			}
			// Seed the owner before the new session query builds its QueryEnv.
			// Re-probe folder-only so Explorer can filter before async snapshot details finish.
			selectFolder := false
			if isSelectFolder, err := window.IsOpenSaveDialogSelectFolderByPid(pid); err == nil {
				selectFolder = isSelectFolder
			}
			sourceWindow := common.ActiveWindowSnapshot{
				Name:                         window.GetWindowNameByPid(pid),
				Pid:                          pid,
				WindowId:                     dialogWindowId,
				IsOpenSaveDialog:             true,
				IsOpenSaveDialogSelectFolder: selectFolder,
			}
			ui.GetUIManager().SeedActiveWindowSnapshotForQuery(sourceWindow)
			showContext.RestoreWindow = &sourceWindow
			plugin.GetPluginManager().GetUI().OpenWoxInstance(localCtx, common.OpenWoxInstanceRequest{
				Role:         common.WoxInstanceRoleSecondary,
				InstanceName: string(common.ShowSourceExplorer),
				Query: common.PlainQuery{
					QueryId:   uuid.NewString(),
					QueryType: plugin.QueryTypeInput,
					QueryScope: common.QueryScope{
						Plugins: []common.QueryScopePlugin{{PluginID: explorerPluginID}},
					},
				},
				ShowApp: showContext,
			})
		}

		showDialogHint := func(localCtx context.Context, pid int) {
			if !runtime.typeToSearchEnabled.Load() {
				return
			}
			if pid <= 0 {
				return
			}
			messageKey := "plugin_explorer_hint_message_dialog"
			// The old Windows renderer accepted points; DirectWrite uses DIPs.
			fontSize := float32(10.0 * 96.0 / 72.0)
			if util.IsMacOS() {
				messageKey = "plugin_explorer_hint_message_dialog_macos"
				fontSize = 12
			}

			title := window.GetWindowNameByPid(pid)
			dialogWindowId := GetOpenSaveDialogWindowIdByPid(pid)
			// An empty id means the overlay cannot anchor to the dialog and will neither
			// inject the sticky subclass nor position itself against the right window.
			util.GetLogger().Info(localCtx, fmt.Sprintf("Explorer dialog hint: pid=%d title=%q dialogWindowId=%q", pid, title, dialogWindowId))
			c.prefetchOpenSaveDialogPath(localCtx, pid, title, dialogWindowId)
			textoverlay.Show(textoverlay.Options{
				Window: overlay.WindowOptions{
					ID:              explorerDialogHintOverlayName,
					StickyWindowPid: pid,
					Anchor:          overlay.AnchorBelowCenter,
					Topmost:         true,
					StickyWindowId:  dialogWindowId,
					MaxWidth:        500,
				},
				Message:  c.api.GetTranslation(localCtx, messageKey),
				FontSize: fontSize,
				OnClick: func() bool {
					clickCtx := context.WithValue(ctx, util.ContextKeyTraceId, uuid.NewString())
					clickCtx = util.WithCoreSessionContext(clickCtx)
					openDialogQuery(clickCtx, pid)
					return true
				},
			})
		}

		showOverlay := func(localCtx context.Context) bool {
			overlay.Close(explorerDialogHintOverlayName)
			if activePid <= 0 || window.GetActiveWindowPid() != activePid {
				c.typeToSearchDebugLog(localCtx, "showOverlay skipped (foreground pid no longer matches explorer pid=%d)", activePid)
				return false
			}
			sourceWindow := common.ActiveWindowSnapshot{
				Name:     window.GetActiveWindowName(),
				Pid:      activePid,
				WindowId: window.GetActiveWindowId(),
			}
			x, y, w, h, ok := GetActiveExplorerRect()
			if !ok {
				x, y, w, h, ok = GetActiveDialogRect()
				if !ok {
					c.typeToSearchDebugLog(localCtx, "showOverlay skipped (no active explorer/dialog rect)")
					return false
				}
			}
			if w <= 0 || h <= 0 {
				c.typeToSearchDebugLog(localCtx, "showOverlay skipped (invalid rect w=%d h=%d)", w, h)
				return false
			}
			c.typeToSearchDebugLog(localCtx, "showOverlay explorerRect=(%d,%d,%d,%d)", x, y, w, h)
			woxSetting := setting.GetSettingManager().GetWoxSetting(localCtx)
			initialWindowHeight := getExplorerInitialWindowHeight(localCtx)
			position := getExplorerWindowPosition(common.WindowRect{X: x, Y: y, Width: w, Height: h}, woxSetting.AppWidth.Get()/2, initialWindowHeight)
			showContext := common.ShowContext{
				HideToolbar:          true,
				QueryBoxAtBottom:     true,
				HideOnBlur:           true,
				ShowSource:           common.ShowSourceExplorer,
				WindowPosition:       &position,
				WindowPositionHeight: initialWindowHeight,
				WindowWidth:          woxSetting.AppWidth.Get() / 2,
				RestoreWindow:        &sourceWindow,
			}
			ui.GetUIManager().SeedActiveWindowSnapshotForQuery(sourceWindow)
			explorerShow = &showContext
			changeExplorerQuery(localCtx)
			return true
		}

		var handoffTimer *time.Timer
		var handoffC <-chan time.Time
		var dialogHintCloseTimer *time.Timer
		var dialogHintCloseC <-chan time.Time
		stopHandoffTimer := func() {
			if handoffTimer != nil {
				handoffTimer.Stop()
				handoffTimer = nil
			}
			handoffC = nil
		}
		stopDialogHintCloseTimer := func() {
			if dialogHintCloseTimer != nil {
				dialogHintCloseTimer.Stop()
				dialogHintCloseTimer = nil
			}
			dialogHintCloseC = nil
		}
		scheduleDialogHintClose := func() {
			stopDialogHintCloseTimer()
			dialogHintCloseTimer = time.NewTimer(explorerDialogHintCloseDelay)
			dialogHintCloseC = dialogHintCloseTimer.C
		}

		resetState := func() {
			handoffUntil = time.Time{}
			pending = ""
			pendingCtx = nil
			explorerShow = nil
			stopHandoffTimer()
		}

		beginHandoff := func() {
			// Brief raw-key capture window after opening the explorer secondary.
			// Focus handoff is not atomic, so fast typing can still arrive here
			// before the launcher text input is ready.
			handoffUntil = time.Now().Add(350 * time.Millisecond)
			stopHandoffTimer()
			handoffTimer = time.NewTimer(350 * time.Millisecond)
			handoffC = handoffTimer.C
		}

		for {
			select {
			case <-runtime.stopCh:
				stopHandoffTimer()
				stopDialogHintCloseTimer()
				return
			case ev := <-events:
				if c.integrationRuntime.Load() != runtime {
					stopHandoffTimer()
					stopDialogHintCloseTimer()
					return
				}
				switch ev.eventType {
				case overlayEventActivate:
					c.typeToSearchDebugLog(ctx, "event activate active=%v handoff=%v pending=%q", active, inHandoff(), pending)
					stopDialogHintCloseTimer()
					active = true
					activePid = ev.pid
					if ev.isDialog {
						c.requestQuickSwitch(ctx, OpenSaveDialogActivatedEvent{
							Pid:              ev.pid,
							WindowID:         ev.windowID,
							PreviousExplorer: ev.previousExplorer,
						})
						if runtime.typeToSearchEnabled.Load() {
							showDialogHint(ctx, ev.pid)
						} else {
							overlay.Close(explorerDialogHintOverlayName)
						}
					} else {
						overlay.Close(explorerDialogHintOverlayName)
					}
					// Keep pending keys during focus handoff; ShowApp can churn
					// activation before fast-typed keys are pushed to the UI.
					if !inHandoff() {
						resetState()
					}
				case overlayEventDeactivate:
					c.typeToSearchDebugLog(ctx, "event deactivate active=%v handoff=%v pending=%q", active, inHandoff(), pending)
					active = false
					activePid = 0
					c.quickSwitch.Invalidate()
					if util.IsWindows() {
						// Closing the secondary can briefly report another foreground window
						// before the dialog focus retry succeeds. Keep the existing hint alive
						// through that handoff so it does not close and recreate visibly.
						scheduleDialogHintClose()
					} else {
						overlay.Close(explorerDialogHintOverlayName)
					}
					if !inHandoff() {
						resetState()
					}
				case overlayEventOpenDialogSearch:
					localCtx := ev.ctx
					if localCtx == nil {
						localCtx = ctx
					}
					if active && activePid > 0 {
						openDialogQuery(localCtx, activePid)
					}
				case overlayEventKey:
					localCtx := ev.ctx
					if localCtx == nil {
						localCtx = ctx
					}
					handoff := inHandoff()
					c.typeToSearchDebugLog(localCtx, "event key=%q active=%v handoff=%v pending=%q", ev.key, active, handoff, pending)
					if (!active && !handoff) || ev.key == "" {
						c.typeToSearchDebugLog(localCtx, "ignore key=%q active=%v handoff=%v", ev.key, active, handoff)
						continue
					}
					if pendingCtx == nil {
						pendingCtx = localCtx
						c.typeToSearchDebugLog(pendingCtx, "begin key=%q", ev.key)
					}
					pending += strings.ToLower(ev.key)
					c.typeToSearchDebugLog(pendingCtx, "pending=%q", pending)
					if handoff {
						changeExplorerQuery(localCtx)
						beginHandoff()
						continue
					}
					if !showOverlay(pendingCtx) {
						c.typeToSearchDebugLog(pendingCtx, "showOverlay failed")
						resetState()
						continue
					}
					beginHandoff()
				}
			case <-handoffC:
				resetState()
			case <-dialogHintCloseC:
				dialogHintCloseTimer = nil
				dialogHintCloseC = nil
				overlay.Close(explorerDialogHintOverlayName)
			}
		}
	}()

	runtime.onExplorerActivated = onActivated
	runtime.onDialogActivated = onDialogActivated
	runtime.onDeactivated = onDeactivated
	runtime.onExplorerKey = onKey
	runtime.onDialogKey = onDialogKey
	c.registerIntegrationMonitors()
}

func getExplorerInitialWindowHeight(ctx context.Context) int {
	theme := ui.GetUIManager().GetCurrentTheme(ctx)
	// Explorer overlays position Wox before UI paints the query box. Using
	// the shared density helper keeps compact and comfortable launcher sizes from
	// appearing offset while preserving theme padding exactly as before.
	queryBoxHeight := ui.DensityQueryBoxBaseHeight(ctx) + theme.AppPaddingTop + theme.AppPaddingBottom
	if queryBoxHeight <= 0 {
		queryBoxHeight = 80
	}
	return queryBoxHeight
}

func getExplorerWindowPosition(anchorRect common.WindowRect, windowWidth int, windowHeight int) common.WindowPosition {
	const margin = 20

	x := anchorRect.X + anchorRect.Width - windowWidth - margin
	if x < anchorRect.X+10 {
		x = anchorRect.X + 10
	}

	y := anchorRect.Y + anchorRect.Height - windowHeight - margin
	if y < anchorRect.Y+10 {
		y = anchorRect.Y + 10
	}

	return common.WindowPosition{X: x, Y: y}
}
