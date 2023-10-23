package system

import (
	"context"
	"github.com/google/uuid"
	"github.com/samber/lo"
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
		Id:            "e2c5f005-6c73-43c8-bc53-ab04def265b2",
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
		for _, pluginManifestShadow := range pluginManifests {
			// action will be executed in another go routine, so we need to copy the variable
			pluginManifest := pluginManifestShadow
			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    pluginManifest.Name,
				SubTitle: pluginManifest.Description,
				Icon:     i.GetIcon(),
				Actions: []plugin.QueryResultAction{
					{
						Name: "install",
						Action: func() {
							plugin.GetStoreManager().Install(ctx, pluginManifest)
						},
					},
				}})
		}
	}

	if query.Command == "uninstall" {
		plugins := plugin.GetPluginManager().GetPluginInstances()
		if query.Search != "" {
			plugins = lo.Filter(plugins, func(pluginInstance *plugin.Instance, _ int) bool {
				return IsStringMatchNoPinYin(ctx, pluginInstance.Metadata.Name, query.Search)
			})
		}

		results = lo.Map(plugins, func(pluginInstanceShadow *plugin.Instance, _ int) plugin.QueryResult {
			// action will be executed in another go routine, so we need to copy the variable
			pluginInstance := pluginInstanceShadow
			return plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    pluginInstance.Metadata.Name,
				SubTitle: pluginInstance.Metadata.Description,
				Icon:     i.GetIcon(),
				Actions: []plugin.QueryResultAction{
					{
						Name: "uninstall",
						Action: func() {
							plugin.GetStoreManager().Uninstall(ctx, pluginInstance)
						},
					},
				},
			}
		})
	}

	return results
}

func (i *WPMPlugin) GetIcon() plugin.WoxImage {
	return plugin.NewWoxImageSvg(`<svg t="1697178225584" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="16738" width="200" height="200"><path d="M842.99 884.364H181.01c-22.85 0-41.374-18.756-41.374-41.892V181.528c0-23.136 18.524-41.892 41.374-41.892h661.98c22.85 0 41.374 18.756 41.374 41.892v660.944c0 23.136-18.524 41.892-41.374 41.892z" fill="#9C34FE" p-id="16739" data-spm-anchor-id="a313x.search_index.0.i6.1f873a81xqBP8f"></path><path d="M387.88 307.2h-82.748v83.78c0 115.68 92.618 209.456 206.868 209.456s206.868-93.776 206.868-209.454V307.2h-82.746v83.78c0 69.408-55.572 125.674-124.122 125.674s-124.12-56.266-124.12-125.672V307.2z" fill="#FFFFFF" p-id="16740"></path></svg>`)
}
