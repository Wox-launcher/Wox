package system

import (
	"context"
	"github.com/google/uuid"
	"wox/plugin"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WPMPlugin{})
}

type WPMPlugin struct {
	api plugin.API
}

func (i *WPMPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "f2a471feeff845079d902fa17a969ab1",
		Name:          "Wox Plugin Manager",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Plugin manager for Wox",
		Icon:          "",
		Entry:         "",
		TriggerKeywords: []string{
			"wpm",
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     "install",
				Description: "Install Wox plugins",
			},
			{
				Command:     "uninstall",
				Description: "Uninstall Wox plugins",
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (i *WPMPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API
}

func (i *WPMPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult

	if query.Command == "install" {
		if query.Search == "" {
			//TODO: return featured plugins
			return results
		}

		pluginManifests := plugin.GetStoreManager().Search(ctx, query.Search)
		for _, pluginManifest := range pluginManifests {
			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    pluginManifest.Name,
				SubTitle: pluginManifest.Description,
				Icon:     plugin.WoxImage{},
				Action: func() {
					plugin.GetStoreManager().Install(ctx, pluginManifest)
				},
			})
		}
	}

	return results
}
