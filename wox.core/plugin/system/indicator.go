package system

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/util"
	"wox/util/fuzzymatch"

	"github.com/google/uuid"
)

var indicatorIcon = common.PluginIndicatorIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &IndicatorPlugin{})
}

type IndicatorPlugin struct {
	api plugin.API
	// Search metadata changes rarely, so keep translated and normalized candidates out of the query hot path.
	searchIndexMu  sync.RWMutex
	searchIndex    []indicatorSearchEntry
	searchIndexKey string
}

type indicatorSearchCandidate struct {
	text     string
	prepared *fuzzymatch.PreparedText
}

type indicatorCommandSearchEntry struct {
	command             plugin.MetadataCommand
	preparedCommand     *fuzzymatch.PreparedText
	preparedDescription *fuzzymatch.PreparedText
}

type indicatorSearchEntry struct {
	pluginInstance        *plugin.Instance
	primaryTriggerKeyword string
	triggerKeywords       []indicatorSearchCandidate
	pluginName            string
	preparedPluginName    *fuzzymatch.PreparedText
	preparedDescription   *fuzzymatch.PreparedText
	commands              []indicatorCommandSearchEntry
}

type indicatorMatchedCommand struct {
	command plugin.MetadataCommand
	score   int64
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
	searchIndex := i.getSearchIndex(ctx, pluginInstances)
	preparedPattern := fuzzymatch.PreparePattern(search)
	usePinyin := setting.GetSettingManager().GetWoxSetting(ctx).UsePinYin.Get()

	var results []plugin.QueryResult
	for _, entry := range searchIndex {
		var matchedTriggerKeyword string
		var triggerKeywordScore int64
		for _, triggerKeyword := range entry.triggerKeywords {
			matchResult := fuzzymatch.FuzzyMatchPrepared(triggerKeyword.prepared, preparedPattern, false)
			if matchResult.IsMatch && matchResult.Score > triggerKeywordScore {
				matchedTriggerKeyword = triggerKeyword.text
				triggerKeywordScore = matchResult.Score
			}
		}

		pluginNameMatch := fuzzymatch.FuzzyMatchPrepared(entry.preparedPluginName, preparedPattern, usePinyin)
		pluginDescriptionMatch := fuzzymatch.FuzzyMatchPrepared(entry.preparedDescription, preparedPattern, usePinyin)
		pluginTextMatch := pluginNameMatch.IsMatch || pluginDescriptionMatch.IsMatch
		pluginTextScore := max(pluginNameMatch.Score, pluginDescriptionMatch.Score)

		var matchedCommands []indicatorMatchedCommand
		var matchedCommandsBestScore int64
		for _, commandEntry := range entry.commands {
			commandMatch := fuzzymatch.FuzzyMatchPrepared(commandEntry.preparedCommand, preparedPattern, false)
			descriptionMatch := fuzzymatch.FuzzyMatchPrepared(commandEntry.preparedDescription, preparedPattern, usePinyin)
			if !commandMatch.IsMatch && !descriptionMatch.IsMatch {
				continue
			}
			commandBestScore := max(commandMatch.Score, descriptionMatch.Score)
			matchedCommands = append(matchedCommands, indicatorMatchedCommand{
				command: commandEntry.command,
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
			triggerKeywordToUse = entry.primaryTriggerKeyword
		}
		if triggerKeywordToUse == "" {
			continue
		}

		resultBaseScore := max(triggerKeywordScore, pluginTextScore, matchedCommandsBestScore)
		if resultBaseScore <= 0 {
			resultBaseScore = 10
		}

		pluginInstance := entry.pluginInstance
		storePlugin, storeErr := plugin.GetStoreManager().GetStorePluginManifestById(ctx, pluginInstance.Metadata.Id)
		hasStorePlugin := storeErr == nil
		isUpgradable := hasStorePlugin && plugin.IsVersionUpgradable(pluginInstance.Metadata.Version, storePlugin.Version)
		upgradeTails := i.buildIndicatorUpgradeTails(pluginInstance.Metadata.Version, storePlugin.Version, hasStorePlugin)
		openSettingsAction := i.createOpenPluginSettingsAction(ctx, pluginInstance, entry.pluginName)
		resultIcon := pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, indicatorIcon)

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
		actions = append(actions, openSettingsAction)
		if isUpgradable {
			actions = append(actions, i.createIndicatorUpgradeAction(storePlugin))
		}

		results = append(results, plugin.QueryResult{
			Id:       uuid.NewString(),
			Title:    triggerKeywordToUse,
			SubTitle: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_indicator_activate_plugin"), entry.pluginName),
			Score:    resultBaseScore,
			Icon:     resultIcon,
			Tails:    upgradeTails,
			Actions:  actions,
		})

		var commandsToShow []indicatorMatchedCommand
		if len(matchedCommands) > 0 {
			commandsToShow = matchedCommands
		} else {
			commandsToShow = make([]indicatorMatchedCommand, 0, len(entry.commands))
			for _, commandEntry := range entry.commands {
				commandsToShow = append(commandsToShow, indicatorMatchedCommand{command: commandEntry.command, score: resultBaseScore - 1})
			}
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
			commandActions = append(commandActions, openSettingsAction)
			if isUpgradable {
				commandActions = append(commandActions, i.createIndicatorUpgradeAction(storePlugin))
			}

			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    fmt.Sprintf("%s %s ", triggerKeywordToUse, metadataCommand.Command),
				SubTitle: string(metadataCommand.Description),
				Score:    commandScore,
				Icon:     resultIcon,
				Tails:    upgradeTails,
				Actions:  commandActions,
			})
		}
	}
	return plugin.NewQueryResponse(results)
}

// getSearchIndex reuses translated and normalized plugin metadata until the plugin snapshot changes.
func (i *IndicatorPlugin) getSearchIndex(ctx context.Context, pluginInstances []*plugin.Instance) []indicatorSearchEntry {
	indexKey := buildIndicatorSearchIndexKey(pluginInstances)
	i.searchIndexMu.RLock()
	if i.searchIndexKey == indexKey {
		index := i.searchIndex
		i.searchIndexMu.RUnlock()
		return index
	}
	i.searchIndexMu.RUnlock()

	index := make([]indicatorSearchEntry, 0, len(pluginInstances))
	for _, pluginInstance := range pluginInstances {
		triggerKeywords := pluginInstance.GetTriggerKeywords()
		entry := indicatorSearchEntry{
			pluginInstance:      pluginInstance,
			pluginName:          pluginInstance.GetName(ctx),
			preparedDescription: fuzzymatch.PrepareText(pluginInstance.GetDescription(ctx)),
		}
		entry.preparedPluginName = fuzzymatch.PrepareText(entry.pluginName)
		for _, triggerKeyword := range triggerKeywords {
			if triggerKeyword == "*" {
				continue
			}
			if entry.primaryTriggerKeyword == "" {
				entry.primaryTriggerKeyword = triggerKeyword
			}
			entry.triggerKeywords = append(entry.triggerKeywords, indicatorSearchCandidate{
				text:     triggerKeyword,
				prepared: fuzzymatch.PrepareText(triggerKeyword),
			})
		}

		commands := pluginInstance.GetQueryCommands()
		entry.commands = make([]indicatorCommandSearchEntry, 0, len(commands))
		for _, command := range commands {
			entry.commands = append(entry.commands, indicatorCommandSearchEntry{
				command:             command,
				preparedCommand:     fuzzymatch.PrepareText(command.Command),
				preparedDescription: fuzzymatch.PrepareText(string(command.Description)),
			})
		}
		index = append(index, entry)
	}

	i.searchIndexMu.Lock()
	if i.searchIndexKey != indexKey {
		i.searchIndex = index
		i.searchIndexKey = indexKey
	} else {
		index = i.searchIndex
	}
	i.searchIndexMu.Unlock()
	return index
}

// buildIndicatorSearchIndexKey invalidates cached translations when language, plugins, keywords, or runtime commands change.
func buildIndicatorSearchIndexKey(pluginInstances []*plugin.Instance) string {
	var builder strings.Builder
	builder.WriteString(string(i18n.GetI18nManager().GetCurrentLangCode()))
	for _, pluginInstance := range pluginInstances {
		fmt.Fprintf(&builder, "|%p", pluginInstance)
		for _, triggerKeyword := range pluginInstance.GetTriggerKeywords() {
			fmt.Fprintf(&builder, "|k%d:%s", len(triggerKeyword), triggerKeyword)
		}
		for _, command := range pluginInstance.RuntimeQueryCommands {
			description := string(command.Description)
			fmt.Fprintf(&builder, "|c%d:%s%d:%s", len(command.Command), command.Command, len(description), description)
		}
	}
	return builder.String()
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

func (i *IndicatorPlugin) createOpenPluginSettingsAction(ctx context.Context, pluginInstance *plugin.Instance, pluginName string) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name:                   fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_indicator_open_plugin_settings"), pluginName),
		Icon:                   pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, common.SettingIcon),
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx, common.SettingWindowContext{
				Path:  "/plugin/setting",
				Param: pluginInstance.Metadata.Id,
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
	actions = append(actions, i.createOpenPluginSettingsAction(ctx, pluginInstance, translatedName))
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
