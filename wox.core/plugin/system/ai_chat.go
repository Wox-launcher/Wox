package system

import (
	"context"
	"wox/plugin"
)

var aiChatIcon = plugin.PluginAIChatIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &AIChatPlugin{})
}

type AIChatPlugin struct {
	api plugin.API
}

func (r *AIChatPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              "a9cfd85a-6e53-415c-9d44-68777aa6323d",
		Name:            "Wox AI Chat",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "Chat with AI",
		Icon:            aiChatIcon.String(),
		TriggerKeywords: []string{"chat"},
		SupportedOS:     []string{"Windows", "Macos", "Linux"},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
		},
	}
}

func (r *AIChatPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
}

func (r *AIChatPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	return []plugin.QueryResult{}
}
