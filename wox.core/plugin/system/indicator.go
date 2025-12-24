package system

import (
	"context"
	"fmt"
	"strings"
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
		Name:          "i18n:plugin_indicator_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_indicator_plugin_description",
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
	search := strings.TrimSpace(query.Search)
	if search == "" {
		return nil
	}

	var results []plugin.QueryResult
	for _, pluginInstance := range plugin.GetPluginManager().GetPluginInstances() {
		pluginName := pluginInstance.GetName(ctx)
		pluginDescription := pluginInstance.GetDescription(ctx)

		primaryTriggerKeyword := lo.FindOrElse(pluginInstance.GetTriggerKeywords(), "", func(triggerKeyword string) bool {
			return triggerKeyword != "*"
		})

		var matchedTriggerKeyword string
		var triggerKeywordScore int64
		for _, triggerKeyword := range pluginInstance.GetTriggerKeywords() {
			if triggerKeyword == "*" {
				continue
			}
			match, score := plugin.IsStringMatchScoreNoPinYin(ctx, triggerKeyword, search)
			if match && score > triggerKeywordScore {
				matchedTriggerKeyword = triggerKeyword
				triggerKeywordScore = score
			}
		}

		pluginNameMatch, pluginNameScore := plugin.IsStringMatchScore(ctx, pluginName, search)
		pluginDescMatch, pluginDescScore := plugin.IsStringMatchScore(ctx, pluginDescription, search)
		pluginTextMatch := pluginNameMatch || pluginDescMatch
		pluginTextScore := max(pluginNameScore, pluginDescScore)

		type matchedCommand struct {
			command plugin.MetadataCommand
			score   int64
		}

		var matchedCommands []matchedCommand
		var matchedCommandsBestScore int64
		translatedCommands := pluginInstance.GetQueryCommands()
		for _, metadataCommand := range translatedCommands {
			cmdMatch, cmdScore := plugin.IsStringMatchScoreNoPinYin(ctx, metadataCommand.Command, search)
			descMatch, descScore := plugin.IsStringMatchScore(ctx, string(metadataCommand.Description), search)
			if !cmdMatch && !descMatch {
				continue
			}
			commandBestScore := max(cmdScore, descScore)
			matchedCommands = append(matchedCommands, matchedCommand{
				command: metadataCommand,
				score:   commandBestScore,
			})
			if commandBestScore > matchedCommandsBestScore {
				matchedCommandsBestScore = commandBestScore
			}
		}

		found := matchedTriggerKeyword != "" || pluginTextMatch || len(matchedCommands) > 0
		if !found {
			continue
		}

		triggerKeywordToUse := matchedTriggerKeyword
		if triggerKeywordToUse == "" {
			triggerKeywordToUse = primaryTriggerKeyword
		}
		if triggerKeywordToUse == "" {
			continue
		}

		resultBaseScore := max(triggerKeywordScore, pluginTextScore, matchedCommandsBestScore)
		if resultBaseScore <= 0 {
			resultBaseScore = 10
		}

		results = append(results, plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    triggerKeywordToUse,
			SubTitle: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_indicator_activate_plugin"), pluginName),
			Score:    resultBaseScore,
			Icon:     pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, indicatorIcon),
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "i18n:plugin_indicator_activate",
					PreventHideAfterAction: true,
					ContextData: common.ContextData{
						"triggerKeyword": triggerKeywordToUse,
						"pluginId":       pluginInstance.Metadata.Id,
					},
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						i.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: fmt.Sprintf("%s ", triggerKeywordToUse),
						})
					},
				},
			},
		})

		var commandsToShow []matchedCommand
		if len(matchedCommands) > 0 {
			commandsToShow = matchedCommands
		} else {
			commandsToShow = lo.Map(translatedCommands, func(cmd plugin.MetadataCommand, index int) matchedCommand {
				return matchedCommand{command: cmd, score: resultBaseScore - 1}
			})
		}

		for _, matchedCommandShadow := range commandsToShow {
			// action will be executed in another go routine, so we need to copy the variable
			matchedCommandCopy := matchedCommandShadow
			metadataCommand := matchedCommandCopy.command
			commandScore := matchedCommandCopy.score
			if commandScore <= 0 {
				commandScore = resultBaseScore - 1
			}
			if commandScore <= 0 {
				commandScore = 9
			}
			if len(matchedCommands) > 0 {
				commandScore = commandScore + 1
			}
			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    fmt.Sprintf("%s %s ", triggerKeywordToUse, metadataCommand.Command),
				SubTitle: string(metadataCommand.Description),
				Score:    commandScore,
				Icon:     pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, indicatorIcon),
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "i18n:plugin_indicator_activate",
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							i.api.ChangeQuery(ctx, common.PlainQuery{
								QueryType: plugin.QueryTypeInput,
								QueryText: fmt.Sprintf("%s %s ", triggerKeywordToUse, metadataCommand.Command),
							})
						},
					},
				},
			})
		}
	}
	return results
}

func (i *IndicatorPlugin) handleMRURestore(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	triggerKeyword := mruData.ContextData["triggerKeyword"]
	pluginId := mruData.ContextData["pluginId"]

	// Find the plugin instance by ID
	var pluginInstance *plugin.Instance
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance.Metadata.Id == pluginId {
			pluginInstance = instance
			break
		}
	}

	if pluginInstance == nil {
		return nil, fmt.Errorf("plugin no longer exists: %s", pluginId)
	}

	// Check if trigger keyword still exists
	triggerKeywords := pluginInstance.GetTriggerKeywords()
	found := false
	for _, keyword := range triggerKeywords {
		if keyword == triggerKeyword {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("trigger keyword no longer exists: %s", triggerKeyword)
	}

	translatedName := pluginInstance.GetName(ctx)
	result := &plugin.QueryResult{
		Id:       uuid.NewString(),
		Title:    triggerKeyword,
		SubTitle: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_indicator_activate_plugin"), translatedName),
		Icon:     pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, indicatorIcon),
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_indicator_activate",
				PreventHideAfterAction: true,
				ContextData:            mruData.ContextData,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					i.api.ChangeQuery(ctx, common.PlainQuery{
						QueryType: plugin.QueryTypeInput,
						QueryText: fmt.Sprintf("%s ", triggerKeyword),
					})
				},
			},
		},
	}

	return result, nil
}
