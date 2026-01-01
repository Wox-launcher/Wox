package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/shell"
	"wox/util/window"
)

var folderIcon = common.FolderIcon
var fileIcon = common.PluginFileIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ExplorerPlugin{})
}

type ExplorerPlugin struct {
	api plugin.API
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
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
		},
		SettingDefinitions: definition.PluginSettingDefinitions{},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureQueryEnv,
				Params: map[string]any{
					"requireActiveWindowPid":              true,
					"requireActiveWindowIsOpenSaveDialog": true,
				},
			},
		},
	}
}

func (c *ExplorerPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
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

	currentPath := window.GetActiveFileExplorerPath()
	if currentPath == "" || query.Search == "" {
		return []plugin.QueryResult{}
	}

	var results []plugin.QueryResult
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, "Failed to read directory: "+err.Error())
		return []plugin.QueryResult{}
	}

	for _, entry := range entries {
		isMatch, matchScore := plugin.IsStringMatchScore(ctx, entry.Name(), query.Search)
		if !isMatch {
			continue
		}

		fullPath := filepath.Join(currentPath, entry.Name())
		icon := fileIcon
		isDir := entry.IsDir()
		if isDir {
			icon = folderIcon
		} else {
			// Can use system icon if available, but simple icon for now
			icon = common.NewWoxImageFileIcon(fullPath)
		}

		// Create actions based on whether it's a directory or file
		var actions []plugin.QueryResultAction
		if isDir {
			// For directories, default action navigates in the current window
			actions = []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_explorer_open",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						window.NavigateActiveFileExplorer(fullPath)
					},
				},
				{
					Name: "i18n:plugin_explorer_open_containing_folder",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						shell.OpenFileInFolder(fullPath)
					},
					Hotkey: "ctrl+enter",
				},
			}
		} else {
			// For files, default action opens the file
			actions = []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_explorer_open",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						shell.Open(fullPath)
					},
				},
				{
					Name: "i18n:plugin_explorer_open_containing_folder",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						shell.OpenFileInFolder(fullPath)
					},
					Hotkey: "ctrl+enter",
				},
			}
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

type commonFolder struct {
	titleKey string
	path     string
}

func (c *ExplorerPlugin) queryOpenSaveDialog(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	folders := getCommonFolders()
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
		}
		if !isMatch {
			continue
		}

		folderPath := folder.path
		activePid := query.Env.ActiveWindowPid
		results = append(results, plugin.QueryResult{
			Title:    folder.titleKey,
			SubTitle: folderPath,
			Icon:     folderIcon,
			Score:    matchScore,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_explorer_open",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.Go(ctx, "navigate to active file explorer", func() {
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

func getCommonFolders() []commonFolder {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	folders := []commonFolder{
		{titleKey: "i18n:plugin_explorer_common_folder_home", path: homeDir},
		{titleKey: "i18n:plugin_explorer_common_folder_desktop", path: filepath.Join(homeDir, "Desktop")},
		{titleKey: "i18n:plugin_explorer_common_folder_documents", path: filepath.Join(homeDir, "Documents")},
		{titleKey: "i18n:plugin_explorer_common_folder_downloads", path: filepath.Join(homeDir, "Downloads")},
		{titleKey: "i18n:plugin_explorer_common_folder_pictures", path: filepath.Join(homeDir, "Pictures")},
		{titleKey: "i18n:plugin_explorer_common_folder_music", path: filepath.Join(homeDir, "Music")},
		{titleKey: "i18n:plugin_explorer_common_folder_videos", path: filepath.Join(homeDir, "Videos")},
	}

	var existing []commonFolder
	for _, folder := range folders {
		if _, err := os.Stat(folder.path); err == nil {
			existing = append(existing, folder)
		}
	}

	return existing
}
