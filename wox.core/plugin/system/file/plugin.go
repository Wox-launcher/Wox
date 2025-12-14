package file

import (
	"context"
	"errors"
	"os"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util/nativecontextmenu"
	"wox/util/shell"

	"github.com/samber/lo"
)

var fileIcon = common.PluginFileIcon
var EverythingNotRunningError = errors.New("Everything is not running")

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &Plugin{})
}

type Plugin struct {
	api plugin.API
}

func (c *Plugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "979d6363-025a-4f51-88d3-0b04e9dc56bf",
		Name:          "i18n:plugin_file_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_file_plugin_description",
		Icon:          fileIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"f",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		SettingDefinitions: definition.PluginSettingDefinitions{},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureDebounce,
				Params: map[string]any{
					"IntervalMs": 500,
				},
			},
		},
	}
}

func (c *Plugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API

	initErr := searcher.Init(ctx)
	if initErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, initErr.Error())
	}
}

func (c *Plugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	// if query is empty, return empty result
	if query.Search == "" {
		return []plugin.QueryResult{}
	}

	// search for the query
	results, err := searcher.Search(SearchPattern{Name: query.Search})
	if err != nil {
		if err == EverythingNotRunningError {
			return []plugin.QueryResult{
				{
					Title:    "i18n:plugin_file_everything_not_running",
					SubTitle: "i18n:plugin_file_everything_please_run_everything",
					Icon:     fileIcon,
					Actions: []plugin.QueryResultAction{
						{
							Name: "i18n:plugin_file_everything_goto_website",
							Action: func(ctx context.Context, actionContext plugin.ActionContext) {
								shell.Open("https://www.voidtools.com/")
							},
						},
					},
				},
			}
		}

		c.api.Log(ctx, plugin.LogLevelError, err.Error())
		c.api.Notify(ctx, err.Error())
		return []plugin.QueryResult{}
	}

	return lo.Map(results, func(item SearchResult, _ int) plugin.QueryResult {
		icon := fileIcon
		if info, err := os.Stat(item.Path); err == nil {
			if info.IsDir() {
				icon = common.FolderIcon
			} else {
				icon = common.NewWoxImageFileIcon(item.Path)
			}
		}

		return plugin.QueryResult{
			Title:    item.Name,
			SubTitle: item.Path,
			Icon:     icon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_file_open",
					Icon: common.PreviewIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						shell.Open(item.Path)
					},
				},
				{
					Name: "i18n:plugin_file_open_containing_folder",
					Icon: common.OpenContainingFolderIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						shell.OpenFileInFolder(item.Path)
					},
					Hotkey: "ctrl+enter",
				},
				{
					Name: "i18n:plugin_file_show_context_menu",
					Icon: common.PluginMenusIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						c.api.Log(ctx, plugin.LogLevelInfo, "Showing context menu for: "+item.Path)
						err := nativecontextmenu.ShowContextMenu(item.Path)
						if err != nil {
							c.api.Log(ctx, plugin.LogLevelError, err.Error())
							c.api.Notify(ctx, err.Error())
						}
					},
					Hotkey:                 "ctrl+m",
					PreventHideAfterAction: true,
				},
			},
		}
	})
}
