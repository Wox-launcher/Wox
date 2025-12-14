package system

import (
	"context"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/util"
)

var queryHistoryIcon = common.PluginQueryHistoryIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &QueryHistoryPlugin{})
}

type QueryHistoryPlugin struct {
	api plugin.API
}

func (i *QueryHistoryPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "fa51ecc4-e491-4e4b-b1f3-70df8a3966d8",
		Name:          "i18n:plugin_query_history_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_query_history_plugin_description",
		Icon:          queryHistoryIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"h",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (i *QueryHistoryPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API
}

func (i *QueryHistoryPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	queryHistories := setting.GetSettingManager().GetWoxSetting(ctx).QueryHistories.Get()

	maxResultCount := 0
	for k := len(queryHistories) - 1; k >= 0; k-- {
		var history = queryHistories[k]

		if strings.Contains(history.Query.String(), query.Search) {
			results = append(results, plugin.QueryResult{
				Title:    history.Query.String(),
				SubTitle: util.FormatTimestamp(history.Timestamp),
				Icon:     queryHistoryIcon,
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "i18n:plugin_query_history_use",
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							i.api.ChangeQuery(ctx, history.Query)
						},
					},
				},
			})

			maxResultCount++
			if maxResultCount >= 20 {
				break
			}
		}
	}

	return
}
