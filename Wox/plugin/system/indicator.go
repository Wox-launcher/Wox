package system

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
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
	if query.TriggerKeyword == "" {
		return i.queryForNonTriggerKeyword(ctx, query)
	}

	if query.Command == "" {
		return i.queryForNonCommand(ctx, query)
	}

	return []plugin.QueryResult{}
}

func (i *IndicatorPlugin) queryForNonTriggerKeyword(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	for _, pluginInstance := range plugin.GetPluginManager().GetPluginInstances() {
		triggerKeyword, found := lo.Find(pluginInstance.GetTriggerKeywords(), func(triggerKeyword string) bool {
			return triggerKeyword != "*" && plugin.GetPluginManager().StringMatchNoPinYin(ctx, triggerKeyword, query.Search)
		})
		if found {
			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    triggerKeyword,
				SubTitle: fmt.Sprintf("Activate %s plugin", pluginInstance.Metadata.Name),
				Icon:     plugin.WoxImage{},
				Actions: []plugin.QueryResultAction{
					{
						Name: "activate",
						Action: func() {
							i.api.ChangeQuery(ctx, fmt.Sprintf("%s ", triggerKeyword))
						},
					},
				},
			})
			for _, metadataCommandShadow := range pluginInstance.Metadata.Commands {
				// action will be executed in another go routine, so we need to copy the variable
				metadataCommand := metadataCommandShadow
				results = append(results, plugin.QueryResult{
					Id:       uuid.NewString(),
					Title:    fmt.Sprintf("%s %s ", triggerKeyword, metadataCommand.Command),
					SubTitle: pluginInstance.Metadata.Description,
					Icon:     plugin.WoxImage{},
					Actions: []plugin.QueryResultAction{
						{
							Name: "activate",
							Action: func() {
								i.api.ChangeQuery(ctx, fmt.Sprintf("%s %s ", triggerKeyword, metadataCommand.Command))
							},
						},
					},
				})
			}
		}
	}
	return results
}

// query for trigger keyword exist but no command exist
func (i *IndicatorPlugin) queryForNonCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	for _, pluginInstance := range plugin.GetPluginManager().GetPluginInstances() {
		_, found := lo.Find(pluginInstance.GetTriggerKeywords(), func(triggerKeyword string) bool {
			return plugin.GetPluginManager().StringMatchNoPinYin(ctx, triggerKeyword, query.TriggerKeyword)
		})
		if found {
			for _, metadataCommandShadow := range pluginInstance.Metadata.Commands {
				// action will be executed in another go routine, so we need to copy the variable
				metadataCommand := metadataCommandShadow
				if plugin.GetPluginManager().StringMatchNoPinYin(ctx, metadataCommand.Command, query.Search) {
					results = append(results, plugin.QueryResult{
						Id:       uuid.NewString(),
						Title:    fmt.Sprintf("%s %s ", query.TriggerKeyword, metadataCommand.Command),
						SubTitle: pluginInstance.Metadata.Description,
						Icon:     plugin.WoxImage{},
						Actions: []plugin.QueryResultAction{
							{
								Name: "activate",
								Action: func() {
									i.api.ChangeQuery(ctx, fmt.Sprintf("%s %s ", query.TriggerKeyword, metadataCommand.Command))
								},
							},
						},
					})
				}
			}
		}
	}
	return results
}

func (i *IndicatorPlugin) getIcon() plugin.WoxImage {
	return plugin.WoxImage{
		ImageType: plugin.WoxImageTypeSvg,
		ImageData: `<svg t="1697178225584" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="16738" width="200" height="200"><path d="M842.99 884.364H181.01c-22.85 0-41.374-18.756-41.374-41.892V181.528c0-23.136 18.524-41.892 41.374-41.892h661.98c22.85 0 41.374 18.756 41.374 41.892v660.944c0 23.136-18.524 41.892-41.374 41.892z" fill="#9C34FE" p-id="16739" data-spm-anchor-id="a313x.search_index.0.i6.1f873a81xqBP8f"></path><path d="M387.88 307.2h-82.748v83.78c0 115.68 92.618 209.456 206.868 209.456s206.868-93.776 206.868-209.454V307.2h-82.746v83.78c0 69.408-55.572 125.674-124.122 125.674s-124.12-56.266-124.12-125.672V307.2z" fill="#FFFFFF" p-id="16740"></path></svg>`,
	}
}
