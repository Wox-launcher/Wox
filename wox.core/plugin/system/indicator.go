package system

import (
	"context"
	"fmt"
	"wox/entity"
	"wox/i18n"
	"wox/plugin"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

var indicatorIcon = plugin.PluginIndicatorIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &IndicatorPlugin{})
}

type IndicatorPlugin struct {
	api plugin.API
}

func (i *IndicatorPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "38564bf0-75ad-4b3e-8afe-a0e0a287c42e",
		Name:          "System Plugin Indicator",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Indicator for plugin trigger keywords",
		Icon:          indicatorIcon.String(),
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
			return triggerKeyword != "*" && IsStringMatchNoPinYin(ctx, triggerKeyword, query.Search)
		})
		if found {
			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    triggerKeyword,
				SubTitle: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_indicator_activate_plugin"), pluginInstance.Metadata.Name),
				Score:    10,
				Icon:     pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, indicatorIcon),
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "i18n:plugin_indicator_activate",
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							i.api.ChangeQuery(ctx, entity.PlainQuery{
								QueryType: plugin.QueryTypeInput,
								QueryText: fmt.Sprintf("%s ", triggerKeyword),
							})
						},
					},
				},
			})
			for _, metadataCommandShadow := range pluginInstance.GetQueryCommands() {
				// action will be executed in another go routine, so we need to copy the variable
				metadataCommand := metadataCommandShadow
				results = append(results, plugin.QueryResult{
					Id:       uuid.NewString(),
					Title:    fmt.Sprintf("%s %s ", triggerKeyword, metadataCommand.Command),
					SubTitle: metadataCommand.Description,
					Score:    10,
					Icon:     pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, indicatorIcon),
					Actions: []plugin.QueryResultAction{
						{
							Name:                   "i18n:plugin_indicator_activate",
							PreventHideAfterAction: true,
							Action: func(ctx context.Context, actionContext plugin.ActionContext) {
								i.api.ChangeQuery(ctx, entity.PlainQuery{
									QueryType: plugin.QueryTypeInput,
									QueryText: fmt.Sprintf("%s %s ", triggerKeyword, metadataCommand.Command),
								})
							},
						},
					},
				})
			}
		}
	}
	return results
}
