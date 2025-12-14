package system

import (
	"context"
	"encoding/json"
	"fmt"
	"wox/common"
	"wox/i18n"
	"wox/plugin"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

var indicatorIcon = common.PluginIndicatorIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &IndicatorPlugin{})
}

type IndicatorPlugin struct {
	api plugin.API
}

type indicatorContextData struct {
	TriggerKeyword string `json:"triggerKeyword"`
	PluginID       string `json:"pluginId"`
	Command        string `json:"command,omitempty"`
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
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureMRU,
			},
		},
	}
}

func (i *IndicatorPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API
	i.api.OnMRURestore(ctx, i.handleMRURestore)
}

func (i *IndicatorPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	for _, pluginInstance := range plugin.GetPluginManager().GetPluginInstances() {
		pluginName := pluginInstance.GetName(ctx)
		pluginDescription := pluginInstance.GetDescription(ctx)

		triggerKeyword, found := lo.Find(pluginInstance.GetTriggerKeywords(), func(triggerKeyword string) bool {
			return triggerKeyword != "*" && IsStringMatchNoPinYin(ctx, triggerKeyword, query.Search)
		})

		if !found {
			// search the plugin name and description
			if IsStringMatch(ctx, pluginDescription, query.Search) || IsStringMatch(ctx, pluginName, query.Search) {
				triggerKeywords := pluginInstance.GetTriggerKeywords()
				if len(triggerKeywords) > 0 {
					// use the first trigger keyword if it's not global keyword
					if triggerKeywords[0] != "*" {
						found = true
						triggerKeyword = triggerKeywords[0]
					}
				}
			}
		}

		if found {
			contextData := indicatorContextData{
				TriggerKeyword: triggerKeyword,
				PluginID:       pluginInstance.Metadata.Id,
			}
			contextDataJson, _ := json.Marshal(contextData)

			results = append(results, plugin.QueryResult{
				Id:          uuid.NewString(),
				Title:       triggerKeyword,
				SubTitle:    fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_indicator_activate_plugin"), pluginName),
				Score:       10,
				Icon:        pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, indicatorIcon),
				ContextData: string(contextDataJson),
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "i18n:plugin_indicator_activate",
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							i.api.ChangeQuery(ctx, common.PlainQuery{
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
								i.api.ChangeQuery(ctx, common.PlainQuery{
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

func (i *IndicatorPlugin) handleMRURestore(mruData plugin.MRUData) (*plugin.QueryResult, error) {
	var contextData indicatorContextData
	if err := json.Unmarshal([]byte(mruData.ContextData), &contextData); err != nil {
		return nil, fmt.Errorf("failed to parse context data: %w", err)
	}

	// Find the plugin instance by ID
	var pluginInstance *plugin.Instance
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance.Metadata.Id == contextData.PluginID {
			pluginInstance = instance
			break
		}
	}

	if pluginInstance == nil {
		return nil, fmt.Errorf("plugin no longer exists: %s", contextData.PluginID)
	}

	// Check if trigger keyword still exists
	triggerKeywords := pluginInstance.GetTriggerKeywords()
	found := false
	for _, keyword := range triggerKeywords {
		if keyword == contextData.TriggerKeyword {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("trigger keyword no longer exists: %s", contextData.TriggerKeyword)
	}

	translatedName := pluginInstance.GetName(context.Background())
	result := &plugin.QueryResult{
		Id:          uuid.NewString(),
		Title:       contextData.TriggerKeyword,
		SubTitle:    fmt.Sprintf(i18n.GetI18nManager().TranslateWox(context.Background(), "plugin_indicator_activate_plugin"), translatedName),
		Icon:        pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, indicatorIcon),
		ContextData: mruData.ContextData,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_indicator_activate",
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					i.api.ChangeQuery(ctx, common.PlainQuery{
						QueryType: plugin.QueryTypeInput,
						QueryText: fmt.Sprintf("%s ", contextData.TriggerKeyword),
					})
				},
			},
		},
	}

	return result, nil
}
