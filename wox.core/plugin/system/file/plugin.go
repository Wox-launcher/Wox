package file

import (
	"context"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"

	"github.com/samber/lo"
)

var fileIcon = plugin.PluginFileIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &Plugin{})
}

type Plugin struct {
	api plugin.API
}

func (c *Plugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "979d6363-025a-4f51-88d3-0b04e9dc56bf",
		Name:          "files",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Search files in your computer",
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
				Params: map[string]string{
					"intervalMs": "500",
				},
			},
		},
	}
}

func (c *Plugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
}

func (c *Plugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	results := searcher.Search(SearchPattern{Name: query.Search})
	return lo.Map(results, func(item SearchResult, _ int) plugin.QueryResult {
		return plugin.QueryResult{
			Title:    item.Name,
			SubTitle: item.Path,
			Icon:     fileIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_file_open",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.ShellOpen(item.Path)
					},
				},
				{
					Name: "i18n:plugin_file_open_containing_folder",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.ShellOpenFileInFolder(item.Path)
					},
				},
			},
		}
	})
}
