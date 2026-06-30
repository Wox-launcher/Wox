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
	shellplugin "wox/plugin/system/shell"
	"wox/setting"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/ui"
	"wox/util"
	"wox/util/overlay"
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

	explorerCommandAdd              = "add"
	explorerDialogHintOverlayName   = "explorer_dialog_hint"
	explorerDialogHintQueryText     = "explorer "
	explorerDialogHintMaxWidth      = 420
	explorerDialogHintVerticalInset = 40
	explorerDialogPathCacheDuration = 30 * time.Second
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ExplorerPlugin{})
}

type overlayRuntime struct {
	stopCh chan struct{}
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
	overlayRuntime         atomic.Pointer[overlayRuntime]
	dialogPathCacheMu      sync.Mutex
	dialogPathCache        openSaveDialogPathCache
	dialogPathResolveGroup singleflight.Group
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
					"requireActiveWindowName":             true,
					"requireActiveWindowPid":              true,
					"requireActiveWindowId":               true,
					"requireActiveWindowIsOpenSaveDialog": true,
				},
			},
		},
	}
}

func (c *ExplorerPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API

	// Start overlay hint listener if enabled
	enableTypeToSearch := c.api.GetSetting(ctx, enableTypeToSearchSettingKey)
	if enableTypeToSearch == "true" {
		c.startOverlayListener(ctx)
	}

	// Listen for setting changes
	c.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key == enableTypeToSearchSettingKey {
			if value == "true" {
				c.startOverlayListener(callbackCtx)
			} else {
				c.stopOverlayListener()
				overlay.Close("explorer_hint")
			}
		}
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
	directoryResults := c.queryCurrentDirectoryEntries(ctx, query)
	jumpResults := c.queryJumpFolders(ctx, query)

	results := make([]plugin.QueryResult, 0, len(directoryResults)+len(jumpResults))
	results = append(results, directoryResults...)
	results = append(results, jumpResults...)
	return results
}

func (c *ExplorerPlugin) queryAddQuickJumpPath(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	path := c.resolveAddPath(ctx, query)
	if path == "" {
		c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Explorer add skipped: no resolvable path (pid=%d, title=%q, isOpenSaveDialog=%v)", query.Env.ActiveWindowPid, query.Env.ActiveWindowTitle, query.Env.ActiveWindowIsOpenSaveDialog))
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
		c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Explorer current directory query skipped: path not found (search=%q, pid=%d, title=%q, isOpenSaveDialog=%v)", query.Search, query.Env.ActiveWindowPid, query.Env.ActiveWindowTitle, query.Env.ActiveWindowIsOpenSaveDialog))
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

	results := make([]plugin.QueryResult, 0, len(entries))
	for _, entry := range entries {
		isMatch := true
		var matchScore int64

		if search != "" {
			isMatch, matchScore = plugin.IsStringMatchScore(ctx, entry.Name(), search)
		}

		if !isMatch {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		isDir := entry.IsDir()
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

		results = append(results, c.buildDirectoryEntryResult(query, entry.Name(), fullPath, isDir, icon, matchScore))
	}

	return results
}

func (c *ExplorerPlugin) buildDirectoryEntryResult(query plugin.Query, title string, fullPath string, isDir bool, icon common.WoxImage, score int64) plugin.QueryResult {
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
			{
				Name:      "i18n:plugin_explorer_reveal_in_explorer",
				IsDefault: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					c.revealEntry(ctx, query.Env, fullPath, isDir)
				},
			},
		},
	}
}

// buildExecuteCommandAtLocationAction opens Shell with the selected location as its working directory.
func (c *ExplorerPlugin) buildExecuteCommandAtLocationAction(path string, isDir bool) plugin.QueryResultAction {
	workingDirectory := path
	if !isDir {
		workingDirectory = filepath.Dir(path)
	}

	return plugin.QueryResultAction{
		Name:                   "i18n:plugin_file_execute_command_here",
		Icon:                   common.PluginShellIcon,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			result, err := c.api.InvokePluginCommand(ctx, plugin.PluginCommandRequest{
				PluginId: shellplugin.PluginID,
				Command:  shellplugin.PluginCommandPrepareCommandAtDirectory,
				Data: common.ContextData{
					shellplugin.PluginCommandDataWorkingDirectory: workingDirectory,
				},
			})
			if err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to invoke shell plugin command: %s", err.Error()))
				c.api.Notify(ctx, err.Error())
				return
			}
			if !result.Handled {
				message := result.Message
				if message == "" {
					message = "shell plugin command was not handled"
				}
				c.api.Log(ctx, plugin.LogLevelWarning, message)
				c.api.Notify(ctx, message)
				return
			}
			if result.Message != "" {
				c.api.Notify(ctx, result.Message)
			}
		},
	}
}

func (c *ExplorerPlugin) revealEntry(ctx context.Context, env plugin.QueryEnv, fullPath string, isDir bool) {
	if env.ActiveWindowIsOpenSaveDialog {
		entryPath := strings.TrimSpace(fullPath)
		if entryPath == "" {
			c.api.Log(ctx, plugin.LogLevelError, "Reveal entry in open/save failed: empty entry path")
			return
		}

		c.activateOwnerWindow(ctx, env.ActiveWindowPid)

		// For current directory search results in open/save, select the item without entering it.
		if window.SelectInActiveFileDialog(entryPath) {
			util.Go(ctx, "highlight open/save dialog entry", func() {
				window.HighlightInActiveFileDialog(entryPath)
			})
			return
		}

		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Select entry in open/save failed: pid=%d entry=%q isDir=%v", env.ActiveWindowPid, entryPath, isDir))
		return
	}

	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Select in explorer by pid: pid=%d path=%s", env.ActiveWindowPid, fullPath))
	if !window.SelectInFileExplorerByPid(env.ActiveWindowPid, fullPath) {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Select in explorer by pid failed: pid=%d path=%s", env.ActiveWindowPid, fullPath))
		return
	}

	window.ActivateWindowByPid(env.ActiveWindowPid)
}

func (c *ExplorerPlugin) activateOwnerWindow(ctx context.Context, pid int) {
	if pid <= 0 {
		return
	}
	if !window.ActivateWindowByPid(pid) {
		c.api.Log(ctx, plugin.LogLevelError, "Failed to activate dialog owner window")
	}
	time.Sleep(150 * time.Millisecond)
}

func (c *ExplorerPlugin) queryJumpFolders(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	folders := c.getJumpFolderCandidates(ctx, query.Env)
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
				Name: "i18n:plugin_explorer_jump_to",
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					c.jumpToFolder(ctx, query.Env, folderPath)
				},
			},
			c.buildExecuteCommandAtLocationAction(folderPath, true),
		},
	}
}

func (c *ExplorerPlugin) getJumpFolderCandidates(ctx context.Context, env plugin.QueryEnv) []openSaveFolder {
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

	// 2) Open Finder window paths.
	openPaths := window.GetOpenFinderWindowPaths()
	for _, p := range openPaths {
		if p == "" {
			continue
		}
		addCandidate(openSaveFolder{
			titleKey: filepath.Base(p),
			path:     p,
		})
	}

	// 3) Common system folders.
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
	util.Go(ctx, "navigate to folder", func() {
		if env.ActiveWindowIsOpenSaveDialog {
			c.activateOwnerWindow(ctx, env.ActiveWindowPid)
			if !window.NavigateActiveFileDialog(folderPath) {
				c.api.Log(ctx, plugin.LogLevelError, "Failed to navigate open/save dialog to path: "+folderPath)
				c.clearOpenSaveDialogPathCache(env.ActiveWindowPid)
				return
			}
			c.setCachedOpenSaveDialogPath(env.ActiveWindowPid, env.ActiveWindowTitle, env.ActiveWindowId, folderPath)
			c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			return
		}

		if window.NavigateInFileExplorer(env.ActiveWindowPid, folderPath, env.ActiveWindowTitle) {
			return
		}

		if env.ActiveWindowPid > 0 && window.SelectInFileExplorerByPid(env.ActiveWindowPid, folderPath) {
			window.ActivateWindowByPid(env.ActiveWindowPid)
			return
		}

		shell.Open(folderPath)
	})
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

func (c *ExplorerPlugin) typeToSearchDebugLog(ctx context.Context, format string, args ...any) {
	// c.api.Log(ctx, plugin.LogLevelDebug, "typeToSearch: "+fmt.Sprintf(format, args...))
}

func (c *ExplorerPlugin) stopOverlayListener() {
	c.typeToSearchDebugLog(context.Background(), "stop monitor")
	StopExplorerMonitor()
	StopExplorerOpenSaveMonitor()
	setExplorerMonitorLogger(nil)
	overlay.Close(explorerDialogHintOverlayName)
	c.clearOpenSaveDialogPathCache(0)

	if runtime := c.overlayRuntime.Swap(nil); runtime != nil {
		close(runtime.stopCh)
	}
}

func (c *ExplorerPlugin) startOverlayListener(ctx context.Context) {
	c.stopOverlayListener()

	setExplorerMonitorLogger(func(msg string) {
		c.typeToSearchDebugLog(ctx, "%s", msg)
	})
	c.typeToSearchDebugLog(ctx, "start monitor")

	runtime := &overlayRuntime{stopCh: make(chan struct{})}
	c.overlayRuntime.Store(runtime)

	type overlayEventType int
	const (
		overlayEventActivate overlayEventType = iota
		overlayEventDeactivate
		overlayEventKey
	)

	type overlayEvent struct {
		eventType overlayEventType
		key       string
		ctx       context.Context
		pid       int
		isDialog  bool
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
	onDialogActivated := func(pid int) {
		pushEvent(overlayEvent{eventType: overlayEventActivate, pid: pid, isDialog: true})
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

	go func() {
		var (
			active         bool
			waitingVisible bool
			waitingSince   time.Time
			handoffUntil   time.Time
			pending        string
			pendingCtx     context.Context
		)

		resetState := func() {
			waitingVisible = false
			waitingSince = time.Time{}
			handoffUntil = time.Time{}
			pending = ""
			pendingCtx = nil
		}

		changeExplorerQuery := func(localCtx context.Context) {
			if pending == "" {
				return
			}
			queryText := "explorer " + pending
			c.typeToSearchDebugLog(localCtx, "changeQuery %q", queryText)
			c.api.ChangeQuery(localCtx, common.PlainQuery{
				QueryType: plugin.QueryTypeInput,
				QueryText: queryText,
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
			plugin.GetPluginManager().GetUI().ShowApp(localCtx, common.ShowContext{
				HideToolbar:      true,
				QueryBoxAtBottom: true,
				HideOnBlur:       true,
				ShowSource:       common.ShowSourceExplorer,
				WindowPosition:   &position,
				WindowWidth:      woxSetting.AppWidth.Get() / 2,
			})
			// ShowApp refreshes foreground state, so seed the dialog owner after
			// Wox is visible and before ChangeQuery builds the plugin QueryEnv.
			ui.GetUIManager().SeedActiveWindowSnapshotForQuery(common.ActiveWindowSnapshot{
				Name:             window.GetWindowNameByPid(pid),
				Pid:              pid,
				WindowId:         dialogWindowId,
				IsOpenSaveDialog: true,
			})
			c.api.ChangeQuery(localCtx, common.PlainQuery{
				QueryType: plugin.QueryTypeInput,
				QueryText: explorerDialogHintQueryText,
			})
		}

		// The dialog hint is passive: it advertises Wox search without turning
		// ordinary filename typing into an Explorer query.
		showDialogHint := func(localCtx context.Context, pid int) {
			if pid <= 0 || c.api.IsVisible(localCtx) {
				return
			}

			title := window.GetWindowNameByPid(pid)
			dialogWindowId := GetOpenSaveDialogWindowIdByPid(pid)
			c.prefetchOpenSaveDialogPath(localCtx, pid, title, dialogWindowId)
			overlay.Show(overlay.OverlayOptions{
				Name:             explorerDialogHintOverlayName,
				Message:          c.api.GetTranslation(localCtx, "plugin_explorer_hint_message_dialog"),
				StickyWindowPid:  pid,
				Anchor:           overlay.AnchorBottomCenter,
				OffsetY:          explorerDialogHintVerticalInset,
				Topmost:          true,
				FontSize:         12,
				MaxWidth:         explorerDialogHintMaxWidth,
				AutoCloseSeconds: 0,
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
			plugin.GetPluginManager().GetUI().ShowApp(localCtx, common.ShowContext{
				HideToolbar:      true,
				QueryBoxAtBottom: true,
				HideOnBlur:       true,
				ShowSource:       common.ShowSourceExplorer,
				WindowPosition:   &position,
				WindowWidth:      woxSetting.AppWidth.Get() / 2,
			})
			return true
		}

		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-runtime.stopCh:
				return
			case ev := <-events:
				switch ev.eventType {
				case overlayEventActivate:
					c.typeToSearchDebugLog(ctx, "event activate active=%v waitingVisible=%v pending=%q", active, waitingVisible, pending)
					active = true
					if ev.isDialog {
						showDialogHint(ctx, ev.pid)
					} else {
						overlay.Close(explorerDialogHintOverlayName)
					}
					// Bug fix: keep pending keys while waiting for visible and during the handoff
					// grace window. ShowApp can trigger activation churn before all fast-typed
					// keys have either been pushed through ChangeQuery or handed to Flutter's
					// EditableText, so the old eager reset still dropped early characters.
					if !waitingVisible && handoffUntil.IsZero() {
						resetState()
					}
				case overlayEventDeactivate:
					c.typeToSearchDebugLog(ctx, "event deactivate active=%v waitingVisible=%v pending=%q", active, waitingVisible, pending)
					active = false
					overlay.Close(explorerDialogHintOverlayName)
					if !waitingVisible && handoffUntil.IsZero() {
						resetState()
					}
				case overlayEventKey:
					localCtx := ev.ctx
					if localCtx == nil {
						localCtx = ctx
					}
					visible := c.api.IsVisible(localCtx)
					c.typeToSearchDebugLog(localCtx, "event key=%q active=%v visible=%v waitingVisible=%v pending=%q", ev.key, active, visible, waitingVisible, pending)
					inHandoff := !handoffUntil.IsZero() && time.Now().Before(handoffUntil)
					canCaptureHandoffKey := waitingVisible || inHandoff
					if (!active && !canCaptureHandoffKey) || ev.key == "" {
						c.typeToSearchDebugLog(localCtx, "ignore key=%q active=%v waitingVisible=%v handoff=%v", ev.key, active, waitingVisible, inHandoff)
						continue
					}
					if visible {
						if !canCaptureHandoffKey {
							c.typeToSearchDebugLog(localCtx, "ignore key=%q (wox visible)", ev.key)
							continue
						}
						// Bug fix: Finder-to-Wox focus handoff is not atomic on macOS. Wox can
						// become visible before the ticker starts the grace window and before
						// Flutter's EditableText is ready, so fast typing after the first key was
						// ignored here and also missed by Flutter. Treat waitingVisible as part of
						// the handoff and push the full query immediately.
						pending += strings.ToLower(ev.key)
						changeExplorerQuery(localCtx)
						waitingVisible = false
						waitingSince = time.Time{}
						handoffUntil = time.Now().Add(350 * time.Millisecond)
						continue
					}
					if pendingCtx == nil {
						pendingCtx = localCtx
						c.typeToSearchDebugLog(pendingCtx, "begin key=%q", ev.key)
					}
					pending += strings.ToLower(ev.key)
					c.typeToSearchDebugLog(pendingCtx, "pending=%q", pending)
					if !waitingVisible {
						if !showOverlay(pendingCtx) {
							c.typeToSearchDebugLog(pendingCtx, "showOverlay failed")
							resetState()
							continue
						}
						waitingVisible = true
						waitingSince = time.Now()
					}
				}
			case <-ticker.C:
				if !handoffUntil.IsZero() && time.Now().After(handoffUntil) {
					resetState()
					continue
				}
				if !waitingVisible {
					continue
				}
				tickCtx := pendingCtx
				if tickCtx == nil {
					tickCtx = ctx
				}
				visible := c.api.IsVisible(tickCtx)
				c.typeToSearchDebugLog(tickCtx, "ticker waitingVisible=%v visible=%v pending=%q active=%v", waitingVisible, visible, pending, active)
				if visible {
					changeExplorerQuery(tickCtx)
					// Keep a short raw-key capture window after the first ChangeQuery. The
					// previous immediate reset assumed Flutter had already taken keyboard focus,
					// but macOS can still deliver the next few Finder key events before the
					// launcher text input is ready, which dropped characters in fast typing.
					waitingVisible = false
					waitingSince = time.Time{}
					handoffUntil = time.Now().Add(350 * time.Millisecond)
					continue
				}
				if !waitingSince.IsZero() && time.Since(waitingSince) > 2*time.Second {
					c.typeToSearchDebugLog(tickCtx, "timeout waiting for wox visible")
					resetState()
				}
			}
		}
	}()

	// Start monitoring file explorer
	StartExplorerMonitor(onActivated, onDeactivated, onKey)

	// Start monitoring open/save dialogs
	StartExplorerOpenSaveMonitor(onDialogActivated, onDeactivated, onKey)
}

func getExplorerInitialWindowHeight(ctx context.Context) int {
	theme := ui.GetUIManager().GetCurrentTheme(ctx)
	// Explorer overlays position Wox before Flutter paints the query box. Using
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
