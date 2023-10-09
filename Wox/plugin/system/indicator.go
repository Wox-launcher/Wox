package system

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"strings"
	"wox/plugin"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &IndicatorPlugin{})
}

type IndicatorPlugin struct {
	api plugin.API
}

func (i *IndicatorPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "39a4a6155f094ef89778188ae4a3ca03",
		Name:          "System Plugin Indicator",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Indicator for plugin trigger keywords",
		Icon:          "",
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (i *IndicatorPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API
}

func (i *IndicatorPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult

	for _, pluginInstance := range plugin.GetPluginManager().GetPluginInstances() {
		triggerKeyword, found := lo.Find(pluginInstance.GetTriggerKeywords(), func(triggerKeyword string) bool {
			return triggerKeyword != "*" && strings.Contains(triggerKeyword, query.Search)
		})
		if found {
			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    triggerKeyword,
				SubTitle: fmt.Sprintf("Activate %s plugin", pluginInstance.Metadata.Name),
				Icon:     plugin.WoxImage{},
				Action: func() bool {
					i.api.ChangeQuery(ctx, fmt.Sprintf("%s ", triggerKeyword))
					return false
				},
			})
			for _, metadataCommand := range pluginInstance.Metadata.Commands {
				results = append(results, plugin.QueryResult{
					Id:       uuid.NewString(),
					Title:    fmt.Sprintf("%s %s", triggerKeyword, metadataCommand.Command),
					SubTitle: pluginInstance.Metadata.Description,
					Icon:     plugin.WoxImage{},
					Action: func() bool {
						i.api.ChangeQuery(ctx, fmt.Sprintf("%s %s ", triggerKeyword, metadataCommand.Command))
						return false
					},
				})
			}
		}
	}

	return results
}
