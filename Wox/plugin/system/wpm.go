package system

import (
	"context"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"wox/plugin"
	"wox/util"
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
				return util.StringContains(pluginInstance.Metadata.Name, query.Search)
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
	return plugin.WoxImage{
		ImageType: plugin.WoxImageTypeSvg,
		ImageData: `<svg t="1697177314625" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="11760" width="200" height="200"><path d="M0 0m170.666667 0l682.666666 0q170.666667 0 170.666667 170.666667l0 682.666666q0 170.666667-170.666667 170.666667l-682.666666 0q-170.666667 0-170.666667-170.666667l0-682.666666q0-170.666667 170.666667-170.666667Z" fill="#7E8BE4" p-id="11761"></path><path d="M524.288 466.517333l265.557333-148.778666c6.144-3.669333 7.381333-11.050667 3.669334-17.194667l-3.669334-3.712c-59.008-33.152-239.744-136.448-239.744-136.448a76.672 76.672 0 0 0-76.202666 0S293.12 262.442667 234.154667 296.832c-6.144 3.712-7.381333 11.093333-3.669334 17.237333l3.669334 3.669334 265.557333 148.778666a28.714667 28.714667 0 0 0 24.576 0z m24.576 83.626667v299.946667c0 7.338667 4.949333 12.288 12.288 12.288 2.474667 0 4.949333 0 6.186667-1.28 59.008-33.152 238.506667-135.210667 238.506666-135.210667 23.338667-13.525333 38.101333-38.101333 38.101334-65.152v-269.226667c0-7.381333-4.949333-12.288-12.330667-12.288-2.432 0-4.906667 0-6.101333 1.237334l-264.32 148.736a23.808 23.808 0 0 0-12.330667 20.906666zM198.528 380.416l264.32 148.736a23.808 23.808 0 0 1 12.288 20.906667v298.752c0 1.237333 0 3.669333-1.28 6.144-3.669333 6.144-11.008 8.618667-17.194667 4.906666-59.008-33.194667-238.506667-135.253333-238.506666-135.253333a75.434667 75.434667 0 0 1-38.101334-65.152v-267.946667c0-2.474667 0-4.949333 1.28-6.186666 3.669333-6.144 11.008-8.618667 17.194667-4.906667z" fill="#FFFFFF" p-id="11762" data-spm-anchor-id="a313x.search_index.0.i0.1f873a81xqBP8f"></path></svg>`,
	}
}
