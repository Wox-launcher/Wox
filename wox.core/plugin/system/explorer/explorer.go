package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/setting/definition"
	"wox/ui"
	"wox/util"
	"wox/util/overlay"
	"wox/util/shell"
	"wox/util/window"

	"github.com/google/uuid"
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

	explorerCommandAdd = "add"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ExplorerPlugin{})
}

type overlayRuntime struct {
	stopCh chan struct{}
}

type ExplorerPlugin struct {
	api            plugin.API
	overlayRuntime atomic.Pointer[overlayRuntime]
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
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          enableTypeToSearchSettingKey,
					Label:        "i18n:plugin_explorer_setting_enable_type_to_search",
					Tooltip:      "i18n:plugin_explorer_setting_enable_type_to_search_tips",
					DefaultValue: "false",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     quickJumpPathsSettingKey,
					Title:   "i18n:plugin_explorer_setting_quick_jump_paths",
					Tooltip: "i18n:plugin_explorer_setting_quick_jump_paths_tips",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "Path",
							Label: "i18n:plugin_explorer_setting_quick_jump_path",
							Type:  definition.PluginSettingValueTableColumnTypeDirPath,
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

func (c *ExplorerPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if !c.shouldHandleQuery(ctx, query) {
		return []plugin.QueryResult{}
	}

	if strings.EqualFold(query.Command, explorerCommandAdd) {
		return c.queryAddQuickJumpPath(ctx, query)
	}

	return c.queryExplorerResults(ctx, query)
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

	c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer current directory query resolved: path=%q search=%q totalEntries=%d matched=%d", dirPath, search, len(entries), len(results)))
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

func (c *ExplorerPlugin) getCurrentFileExplorerPath(ctx context.Context, env plugin.QueryEnv) string {
	isFileExplorerPid := false
	if env.ActiveWindowPid > 0 {
		if ok, err := window.IsFileExplorer(env.ActiveWindowPid); err != nil {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer failed to detect file explorer pid=%d: %s", env.ActiveWindowPid, err.Error()))
		} else {
			isFileExplorerPid = ok
		}
	}

	shouldUseDialogPath := env.ActiveWindowIsOpenSaveDialog && !isFileExplorerPid
	if env.ActiveWindowIsOpenSaveDialog && isFileExplorerPid {
		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer ignored open/save flag for finder pid=%d", env.ActiveWindowPid))
	}

	if shouldUseDialogPath {
		if dialogPath := strings.TrimSpace(window.GetFileDialogPathByPid(env.ActiveWindowPid)); dialogPath != "" {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer path resolved from file dialog by pid: pid=%d path=%q", env.ActiveWindowPid, dialogPath))
			return dialogPath
		}
		if dialogPath := strings.TrimSpace(window.GetActiveFileDialogPath()); dialogPath != "" {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer path resolved from active file dialog: path=%q", dialogPath))
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
		if dialogPath := strings.TrimSpace(window.GetFileDialogPathByPid(env.ActiveWindowPid)); dialogPath != "" {
			c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Explorer path resolved from file dialog compatibility fallback: pid=%d path=%q", env.ActiveWindowPid, dialogPath))
			return dialogPath
		}
	}

	c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Explorer path resolve failed: pid=%d title=%q isOpenSaveDialog=%v", env.ActiveWindowPid, env.ActiveWindowTitle, env.ActiveWindowIsOpenSaveDialog))
	return ""
}

func (c *ExplorerPlugin) jumpToFolder(ctx context.Context, env plugin.QueryEnv, folderPath string) {
	util.Go(ctx, "navigate to folder", func() {
		if env.ActiveWindowIsOpenSaveDialog {
			c.activateOwnerWindow(ctx, env.ActiveWindowPid)
			if !window.NavigateActiveFileDialog(folderPath) {
				c.api.Log(ctx, plugin.LogLevelError, "Failed to navigate open/save dialog to path: "+folderPath)
			}
			return
		}

		if window.NavigateInFileExplorerByPid(env.ActiveWindowPid, folderPath) {
			window.ActivateWindowByPid(env.ActiveWindowPid)
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

	c.api.SaveSetting(ctx, quickJumpPathsSettingKey, string(payload), false)
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

func (c *ExplorerPlugin) stopOverlayListener() {
	c.api.Log(context.Background(), plugin.LogLevelInfo, "typeToSearch: stop monitor")
	StopExplorerMonitor()
	StopExplorerOpenSaveMonitor()
	setExplorerMonitorLogger(nil)

	if runtime := c.overlayRuntime.Swap(nil); runtime != nil {
		close(runtime.stopCh)
	}
}

func (c *ExplorerPlugin) startOverlayListener(ctx context.Context) {
	c.stopOverlayListener()

	setExplorerMonitorLogger(func(msg string) {
		c.api.Log(ctx, plugin.LogLevelDebug, "typeToSearch: "+msg)
	})
	c.api.Log(ctx, plugin.LogLevelInfo, "typeToSearch: start monitor")

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
	}

	events := make(chan overlayEvent, 64)
	pushEvent := func(ev overlayEvent) {
		select {
		case events <- ev:
		default:
		}
	}

	onActivated := func(pid int) {
		c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("typeToSearch: activated pid=%d", pid))
		pushEvent(overlayEvent{eventType: overlayEventActivate})
	}
	onDeactivated := func() {
		c.api.Log(ctx, plugin.LogLevelDebug, "typeToSearch: deactivated")
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
			pending        string
			pendingCtx     context.Context
		)

		resetState := func() {
			waitingVisible = false
			waitingSince = time.Time{}
			pending = ""
			pendingCtx = nil
		}

		showOverlay := func(localCtx context.Context) bool {
			x, y, w, h, ok := GetActiveExplorerRect()
			if !ok {
				x, y, w, h, ok = GetActiveDialogRect()
				if !ok {
					c.api.Log(localCtx, plugin.LogLevelInfo, "typeToSearch: showOverlay skipped (no active explorer/dialog rect)")
					return false
				}
			}
			if w <= 0 || h <= 0 {
				c.api.Log(localCtx, plugin.LogLevelInfo, fmt.Sprintf("typeToSearch: showOverlay skipped (invalid rect w=%d h=%d)", w, h))
				return false
			}

			overlayWidth := 400
			if woxSetting := setting.GetSettingManager().GetWoxSetting(localCtx); woxSetting != nil {
				if configuredWidth := woxSetting.AppWidth.Get() / 2; configuredWidth > 0 {
					overlayWidth = configuredWidth
				}
			}

			// Target X: Right edge of explorer - overlay width - padding
			targetX := x + w - overlayWidth - 20
			if targetX < x+10 {
				targetX = x + 10
			}

			// Keep the initial top position aligned with the actual query box height.
			// This avoids vertical drift before resize logic expands the result area.
			currentTheme := ui.GetUIManager().GetCurrentTheme(localCtx)
			queryBoxHeight := 55 + currentTheme.AppPaddingTop + currentTheme.AppPaddingBottom
			if queryBoxHeight <= 0 {
				queryBoxHeight = 80
			}
			// Target Y: Bottom edge of explorer - query box height - padding
			// We position it near the bottom so it can grow upwards
			targetY := y + h - queryBoxHeight - 20
			if targetY < y+10 {
				targetY = y + 10
			}

			c.api.Log(localCtx, plugin.LogLevelInfo, fmt.Sprintf("typeToSearch: showOverlay rect=(%d,%d,%d,%d) target=(%d,%d) width=%d", x, y, w, h, targetX, targetY, overlayWidth))
			plugin.GetPluginManager().GetUI().ShowApp(localCtx, common.ShowContext{
				SelectAll:      false,
				WindowPosition: &common.WindowPosition{X: targetX, Y: targetY},
				LayoutMode:     common.LayoutModeExplorer,
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
					active = true
					// Don't reset state when we're waiting for the overlay to become visible.
					// ShowApp steals focus from Explorer, which triggers deactivated → activated cycling.
					// Resetting here would clear pending keys before the ticker can send changeQuery.
					if !waitingVisible {
						resetState()
					}
				case overlayEventDeactivate:
					active = false
					if !waitingVisible {
						resetState()
					}
				case overlayEventKey:
					localCtx := ev.ctx
					if localCtx == nil {
						localCtx = ctx
					}
					if !active || ev.key == "" {
						c.api.Log(localCtx, plugin.LogLevelDebug, fmt.Sprintf("typeToSearch: ignore key=%q active=%v", ev.key, active))
						continue
					}
					if c.api.IsVisible(localCtx) {
						c.api.Log(localCtx, plugin.LogLevelDebug, fmt.Sprintf("typeToSearch: ignore key=%q (wox visible)", ev.key))
						continue
					}
					if pendingCtx == nil {
						pendingCtx = localCtx
						c.api.Log(pendingCtx, plugin.LogLevelInfo, fmt.Sprintf("typeToSearch: begin key=%q", ev.key))
					}
					pending += strings.ToLower(ev.key)
					c.api.Log(pendingCtx, plugin.LogLevelDebug, fmt.Sprintf("typeToSearch: pending=%q", pending))
					if !waitingVisible {
						if !showOverlay(pendingCtx) {
							c.api.Log(pendingCtx, plugin.LogLevelInfo, "typeToSearch: showOverlay failed")
							resetState()
							continue
						}
						waitingVisible = true
						waitingSince = time.Now()
					}
				}
			case <-ticker.C:
				if !waitingVisible {
					continue
				}
				tickCtx := pendingCtx
				if tickCtx == nil {
					tickCtx = ctx
				}
				if c.api.IsVisible(tickCtx) {
					if pending != "" {
						queryText := "explorer " + pending
						c.api.Log(tickCtx, plugin.LogLevelInfo, fmt.Sprintf("typeToSearch: changeQuery %q", queryText))
						c.api.ChangeQuery(tickCtx, common.PlainQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: queryText,
						})
					}
					resetState()
					continue
				}
				if !waitingSince.IsZero() && time.Since(waitingSince) > 2*time.Second {
					c.api.Log(tickCtx, plugin.LogLevelDebug, "typeToSearch: timeout waiting for wox visible")
					resetState()
				}
			}
		}
	}()

	// Start monitoring file explorer
	StartExplorerMonitor(onActivated, onDeactivated, onKey)

	// Start monitoring open/save dialogs
	StartExplorerOpenSaveMonitor(onActivated, onDeactivated, onKey)
}
