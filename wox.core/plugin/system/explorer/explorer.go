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

const (
	openSaveHistorySettingKey    = "openSaveHistory"
	enableTypeToSearchSettingKey = "enableTypeToSearch"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ExplorerPlugin{})
}

type overlayRuntime struct {
	stopCh chan struct{}
}

type ExplorerPlugin struct {
	api                plugin.API
	openSaveHistoryMap *util.HashMap[string, []openSaveHistoryEntry] // app window title -> history entries

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
			"*",
			"explorer",
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
	c.openSaveHistoryMap = c.loadOpenSaveHistory(ctx)

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
	// If global trigger, check context
	if query.IsGlobalQuery() {
		isFileExplorer, err := window.IsFileExplorer(query.Env.ActiveWindowPid)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelError, "Failed to check if active app is file explorer: "+err.Error())
			return []plugin.QueryResult{}
		}

		if !isFileExplorer {
			if !query.Env.ActiveWindowIsOpenSaveDialog {
				return []plugin.QueryResult{}
			}
		}
	}

	if query.Env.ActiveWindowIsOpenSaveDialog {
		return c.queryOpenSaveDialog(ctx, query)
	}

	return c.queryFileExplorer(ctx, query)
}

func (c *ExplorerPlugin) queryFileExplorer(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	currentPath := ""
	if util.IsWindows() {
		// Prefer the actual foreground tab path on Windows 11 (tabs may share the same HWND).
		currentPath = window.GetActiveFileExplorerPath()
		if currentPath == "" {
			currentPath = window.GetFileExplorerPathByPidAndWindowTitle(query.Env.ActiveWindowPid, query.Env.ActiveWindowTitle)
		}
	} else {
		currentPath = window.GetFileExplorerPathByPid(query.Env.ActiveWindowPid)
	}
	if currentPath == "" {
		return []plugin.QueryResult{}
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, "Failed to read directory: "+err.Error())
		return []plugin.QueryResult{}
	}

	results := make([]plugin.QueryResult, 0, len(entries))
	for _, entry := range entries {
		isMatch := true
		var matchScore int64

		if query.Search != "" {
			isMatch, matchScore = plugin.IsStringMatchScore(ctx, entry.Name(), query.Search)
		}

		if !isMatch {
			continue
		}

		fullPath := filepath.Join(currentPath, entry.Name())
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

		var actions []plugin.QueryResultAction
		actions = []plugin.QueryResultAction{
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
					c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Navigate explorer by pid: pid=%d path=%s", query.Env.ActiveWindowPid, fullPath))
					if !window.SelectInFileExplorerByPid(query.Env.ActiveWindowPid, fullPath) {
						c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Select in explorer by pid failed: pid=%d path=%s", query.Env.ActiveWindowPid, fullPath))
					} else {
						window.ActivateWindowByPid(query.Env.ActiveWindowPid)
					}
				},
			},
		}

		results = append(results, plugin.QueryResult{
			Title:    entry.Name(),
			SubTitle: fullPath,
			Icon:     icon,
			Score:    matchScore,
			Actions:  actions,
		})
	}

	return results
}

func (c *ExplorerPlugin) queryOpenSaveDialog(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	folders := c.getOpenSaveFolders(ctx, query.Env)
	if len(folders) == 0 {
		return []plugin.QueryResult{}
	}

	search := strings.TrimSpace(query.Search)
	var results []plugin.QueryResult
	for _, folder := range folders {
		title := i18n.GetI18nManager().TranslateWox(ctx, folder.titleKey)
		// If folder has an explicit title (for common folders), translate it.
		// For dynamic Finder windows, titleKey is just the path or name, so we use it directly if translation fails or key is missing.
		if title == folder.titleKey && !strings.HasPrefix(title, "i18n:") {
			// It's likely a raw path or name
		}

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

		folderPath := folder.path
		activePid := query.Env.ActiveWindowPid
		score := matchScore + folder.scoreBoost
		results = append(results, plugin.QueryResult{
			Title:    title,
			SubTitle: folderPath,
			Icon:     common.FolderIcon,
			Score:    score,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_explorer_jump_to",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.Go(ctx, "navigate to active file explorer", func() {
							c.recordOpenSaveHistory(ctx, query.Env.ActiveWindowTitle, folderPath)
							if activePid > 0 {
								if !window.ActivateWindowByPid(activePid) {
									c.api.Log(ctx, plugin.LogLevelError, "Failed to activate dialog owner window")
								}
								time.Sleep(150 * time.Millisecond)
							}
							if !window.NavigateActiveFileDialog(folderPath) {
								c.api.Log(ctx, plugin.LogLevelError, "Failed to navigate open/save dialog to path: "+folderPath)
							}
						})
					},
				},
			},
		})
	}

	return results
}

func (c *ExplorerPlugin) getOpenSaveFolders(ctx context.Context, env plugin.QueryEnv) []openSaveFolder {
	candidates := make([]openSaveFolder, 0)

	// First, load from history
	c.openSaveHistoryMap.Range(func(key string, entries []openSaveHistoryEntry) bool {
		if key != env.ActiveWindowTitle {
			return true
		}

		now := time.Now().Unix()
		for _, entry := range entries {
			info, err := os.Stat(entry.Path)
			if err != nil || !info.IsDir() {
				continue
			}

			// Calculate score boost based on recency and frequency
			timeDiff := now - entry.UsedAt
			timeScore := int64(0)
			if timeDiff < 3600 { // within 1 hour
				timeScore = 100
			} else if timeDiff < 86400 { // within 1 day
				timeScore = 50
			} else if timeDiff < 604800 { // within 1 week
				timeScore = 20
			}
			frequencyScore := int64(entry.Count * 5)
			totalScore := timeScore + frequencyScore

			candidate := openSaveFolder{
				titleKey:   filepath.Base(entry.Path),
				path:       entry.Path,
				scoreBoost: totalScore,
			}
			candidates = append(candidates, candidate)
		}
		return false
	})

	// 2. Get open Finder windows
	openPaths := window.GetOpenFinderWindowPaths()
	for _, p := range openPaths {
		if p == "" {
			continue
		}
		candidate := openSaveFolder{
			titleKey: filepath.Base(p),
			path:     p,
		}
		candidates = append(candidates, candidate)
	}

	// 2. Add common system folders
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
			if _, err := os.Stat(folder.path); err == nil {
				candidate := openSaveFolder{
					titleKey: folder.titleKey,
					path:     folder.path,
				}
				candidates = append(candidates, candidate)
			}
		}
	}

	return candidates
}

func (c *ExplorerPlugin) recordOpenSaveHistory(ctx context.Context, key string, path string) {
	if key == "" || path == "" {
		return
	}

	if v, ok := c.openSaveHistoryMap.Load(key); ok {
		newList := []openSaveHistoryEntry{}

		found := false
		for _, entry := range v {
			if entry.Path == path {
				entry.UsedAt = time.Now().Unix()
				entry.Count += 1
				found = true
			}
			newList = append(newList, entry)
		}

		if !found {
			newList = append([]openSaveHistoryEntry{{Path: path, UsedAt: time.Now().Unix(), Count: 1}}, newList...)
		}

		c.openSaveHistoryMap.Store(key, newList)
	}

	payload, err := json.Marshal(c.openSaveHistoryMap.ToMap())
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, "Failed to marshal open/save history: "+err.Error())
		return
	}
	c.api.SaveSetting(ctx, openSaveHistorySettingKey, string(payload), false)
}

func (c *ExplorerPlugin) loadOpenSaveHistory(ctx context.Context) *util.HashMap[string, []openSaveHistoryEntry] {
	var items = util.NewHashMap[string, []openSaveHistoryEntry]()
	raw := c.api.GetSetting(ctx, openSaveHistorySettingKey)
	if raw == "" {
		return items
	}

	unmarshalMap := make(map[string][]openSaveHistoryEntry)
	if err := json.Unmarshal([]byte(raw), &unmarshalMap); err != nil {
		c.api.Log(ctx, plugin.LogLevelError, "Failed to load open/save history: "+err.Error())
		return items
	}

	for k, v := range unmarshalMap {
		items.Store(k, v)
	}

	return items
}

func (c *ExplorerPlugin) stopOverlayListener() {
	StopExplorerMonitor()
	StopExplorerOpenSaveMonitor()

	if runtime := c.overlayRuntime.Swap(nil); runtime != nil {
		close(runtime.stopCh)
	}
}

func (c *ExplorerPlugin) startOverlayListener(ctx context.Context) {
	c.stopOverlayListener()

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
					resetState()
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
					if util.IsWindows() {
						if c.api.IsVisible(localCtx) {
							c.api.Log(localCtx, plugin.LogLevelDebug, fmt.Sprintf("typeToSearch: ignore key=%q (wox visible)", ev.key))
							continue
						}
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
