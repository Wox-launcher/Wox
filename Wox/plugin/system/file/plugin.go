package file

import (
	"context"
	"github.com/samber/lo"
	"path"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
)

var fileIcon = plugin.NewWoxImageSvg(`<svg t="1715182653592" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="7787" width="200" height="200"><path d="M0 0m102.4 0l819.2 0q102.4 0 102.4 102.4l0 819.2q0 102.4-102.4 102.4l-819.2 0q-102.4 0-102.4-102.4l0-819.2q0-102.4 102.4-102.4Z" fill="#8A73F2" p-id="7788"></path><path d="M767.36 776.7552c0 16.9472-8.5248 29.6448-21.2992 29.6448H307.1744c-12.8 0-25.5744-12.6976-25.5744-29.6448V247.2448c0-16.9472 8.5248-29.6448 21.2992-29.6448h235.8784l227.072 215.2448 1.5104 343.9104z" fill="#EEF0FF" p-id="7789"></path><path d="M766.5664 432.896h-194.2784c-20.8128 0-34.688-20.8128-34.688-48.6144V217.6l228.9664 215.296z" fill="#CDC7FB" p-id="7790"></path></svg>`)

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
					Name: "Open",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.ShellOpen(item.Path)
					},
				},
				{
					Name: "Open containing folder",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.ShellOpen(path.Dir(item.Path))
					},
				},
			},
		}
	})
}
