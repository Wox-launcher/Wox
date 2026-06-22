package system

import (
	"context"
	"fmt"
	"strings"

	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/util"

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

func (i *IndicatorPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	search := strings.TrimSpace(query.Search)
	if search == "" {
		return plugin.QueryResponse{}
	}

	pluginInstances := plugin.GetPluginManager().GetPluginInstances()

	var results []plugin.QueryResult
	for _, pluginInstance := range pluginInstances {
		storePlugin, storeErr := plugin.GetStoreManager().GetStorePluginManifestById(ctx, pluginInstance.Metadata.Id)
		hasStorePlugin := storeErr == nil
		upgradeTails := i.buildIndicatorUpgradeTails(pluginInstance.Metadata.Version, storePlugin.Version, hasStorePlugin)

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

		pluginName := pluginInstance.GetName(ctx)
		pluginDescription := pluginInstance.GetDescription(ctx)

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

		actions := []plugin.QueryResultAction{
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
		}
		if hasStorePlugin && plugin.IsVersionUpgradable(pluginInstance.Metadata.Version, storePlugin.Version) {
			actions = append(actions, i.createIndicatorUpgradeAction(storePlugin))
		}

		results = append(results, plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    triggerKeywordToUse,
			SubTitle: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_indicator_activate_plugin"), pluginName),
			Score:    resultBaseScore,
			Icon:     pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, indicatorIcon),
			Tails:    upgradeTails,
			Actions:  actions,
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
			commandActions := []plugin.QueryResultAction{
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
			}
			if hasStorePlugin && plugin.IsVersionUpgradable(pluginInstance.Metadata.Version, storePlugin.Version) {
				commandActions = append(commandActions, i.createIndicatorUpgradeAction(storePlugin))
			}

			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    fmt.Sprintf("%s %s ", triggerKeywordToUse, metadataCommand.Command),
				SubTitle: string(metadataCommand.Description),
				Score:    commandScore,
				Icon:     pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, indicatorIcon),
				Tails:    upgradeTails,
				Actions:  commandActions,
			})
		}
	}
	return plugin.NewQueryResponse(results)
}

func (i *IndicatorPlugin) buildIndicatorUpgradeTails(installedVersion string, storeVersion string, hasStoreVersion bool) []plugin.QueryResultTail {
	if !hasStoreVersion || !plugin.IsVersionUpgradable(installedVersion, storeVersion) {
		return nil
	}

	return []plugin.QueryResultTail{
		{
			Type:  plugin.QueryResultTailTypeImage,
			Image: common.UpgradeIcon,
		},
		{
			Type: plugin.QueryResultTailTypeText,
			Text: fmt.Sprintf("v%s -> v%s", installedVersion, storeVersion),
		},
	}
}

func (i *IndicatorPlugin) createIndicatorUpgradeAction(storePlugin plugin.StorePluginManifest) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name:                   "i18n:plugin_wpm_upgrade",
		Icon:                   common.UpdateIcon,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			pluginName := storePlugin.GetName(ctx)
			i.api.Notify(ctx, fmt.Sprintf(
				i.api.GetTranslation(ctx, "i18n:plugin_installer_action_start"),
				i.api.GetTranslation(ctx, "i18n:plugin_installer_upgrade"),
				pluginName,
			))

			util.Go(ctx, "upgrade plugin from indicator", func() {
				pluginName := storePlugin.GetName(ctx)
				installErr := plugin.GetStoreManager().InstallWithProgress(ctx, storePlugin, func(message string) {
					i.api.Notify(ctx, fmt.Sprintf("%s: %s", pluginName, message))
				})

				if installErr != nil {
					i.api.Notify(ctx, fmt.Sprintf(
						i.api.GetTranslation(ctx, "i18n:plugin_installer_action_failed"),
						i.api.GetTranslation(ctx, "i18n:plugin_installer_upgrade"),
						formatPluginInstallError(ctx, i.api, storePlugin.Runtime, pluginName, storePlugin.Version, installErr),
					))
					return
				}

				i.api.Notify(ctx, fmt.Sprintf(
					i.api.GetTranslation(ctx, "i18n:plugin_installer_action_success"),
					pluginName,
					i.api.GetTranslation(ctx, "i18n:plugin_installer_verb_upgrade_past"),
				))
				i.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			})
		},
	}
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
	upgradeTails := []plugin.QueryResultTail(nil)
	actions := []plugin.QueryResultAction{
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
	}
	storePlugin, storeErr := plugin.GetStoreManager().GetStorePluginManifestById(ctx, pluginInstance.Metadata.Id)
	if storeErr == nil {
		upgradeTails = i.buildIndicatorUpgradeTails(pluginInstance.Metadata.Version, storePlugin.Version, true)
		if plugin.IsVersionUpgradable(pluginInstance.Metadata.Version, storePlugin.Version) {
			actions = append(actions, i.createIndicatorUpgradeAction(storePlugin))
		}
	}

	result := &plugin.QueryResult{
		Id:       uuid.NewString(),
		Title:    triggerKeyword,
		SubTitle: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_indicator_activate_plugin"), translatedName),
		Icon:     pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, indicatorIcon),
		Tails:    upgradeTails,
		Actions:  actions,
	}

	return result, nil
}
