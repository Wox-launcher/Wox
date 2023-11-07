package system

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"wox/i18n"
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
		Id:            "38564bf0-75ad-4b3e-8afe-a0e0a287c42e",
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
		return i.queryForTriggerKeyword(ctx, query)
	}

	if query.Command == "" {
		return i.queryForCommand(ctx, query)
	}

	return []plugin.QueryResult{}
}

func (i *IndicatorPlugin) queryForTriggerKeyword(ctx context.Context, query plugin.Query) []plugin.QueryResult {
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
				Icon:     plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48"><circle cx="22" cy="9" r="2" fill="#c767e5"></circle><circle cx="12" cy="9" r="2" fill="#c767e5"></circle><path fill="#c767e5" d="M34,9c0-1.1-0.9-2-2-2H22l-2,2c0,3,3,3,3,7c0,0.702-0.127,1.374-0.349,2H34V9z"></path><path fill="#c767e5" d="M11,16c0-4,3-4,3-7l-2-2H2C0.9,7,0,7.9,0,9v9h11.349C11.127,17.374,11,16.702,11,16z"></path><path fill="#a238c2" d="M34,39v-9H0v9c0,1.1,0.9,2,2,2h30C33.1,41,34,40.1,34,39z"></path><path fill="#ba54d9" d="M34,29.806c0-1.854,2.204-2.772,3.558-1.507C38.513,29.19,39.75,30,42,30	c3.675,0,6.578-3.303,5.902-7.102c-0.572-3.218-3.665-5.24-6.909-4.838c-1.663,0.206-2.671,0.92-3.479,1.684	C36.185,20.998,34,20.028,34,18.2V18H22.651c-0.825,2.329-3.04,4-5.651,4s-4.827-1.671-5.651-4H0v12h34V29.806z"></path></svg>`),
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "activate",
						PreventHideAfterAction: true,
						Action: func(actionContext plugin.ActionContext) {
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
					Icon:     plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48"><circle cx="22" cy="9" r="2" fill="#c767e5"></circle><circle cx="12" cy="9" r="2" fill="#c767e5"></circle><path fill="#c767e5" d="M34,9c0-1.1-0.9-2-2-2H22l-2,2c0,3,3,3,3,7c0,0.702-0.127,1.374-0.349,2H34V9z"></path><path fill="#c767e5" d="M11,16c0-4,3-4,3-7l-2-2H2C0.9,7,0,7.9,0,9v9h11.349C11.127,17.374,11,16.702,11,16z"></path><path fill="#a238c2" d="M34,39v-9H0v9c0,1.1,0.9,2,2,2h30C33.1,41,34,40.1,34,39z"></path><path fill="#ba54d9" d="M34,29.806c0-1.854,2.204-2.772,3.558-1.507C38.513,29.19,39.75,30,42,30	c3.675,0,6.578-3.303,5.902-7.102c-0.572-3.218-3.665-5.24-6.909-4.838c-1.663,0.206-2.671,0.92-3.479,1.684	C36.185,20.998,34,20.028,34,18.2V18H22.651c-0.825,2.329-3.04,4-5.651,4s-4.827-1.671-5.651-4H0v12h34V29.806z"></path></svg>`),
					Actions: []plugin.QueryResultAction{
						{
							Name:                   "activate",
							PreventHideAfterAction: true,
							Action: func(actionContext plugin.ActionContext) {
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

func (i *IndicatorPlugin) queryForCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	for _, pluginInstance := range plugin.GetPluginManager().GetPluginInstances() {
		_, found := lo.Find(pluginInstance.GetTriggerKeywords(), func(triggerKeyword string) bool {
			return IsStringMatchNoPinYin(ctx, triggerKeyword, query.TriggerKeyword)
		})
		if found {
			for _, metadataCommandShadow := range pluginInstance.Metadata.Commands {
				// action will be executed in another go routine, so we need to copy the variable
				metadataCommand := metadataCommandShadow
				if query.Search == "" || IsStringMatchNoPinYin(ctx, metadataCommand.Command, query.Search) {
					results = append(results, plugin.QueryResult{
						Id:       uuid.NewString(),
						Title:    fmt.Sprintf("%s %s ", query.TriggerKeyword, metadataCommand.Command),
						SubTitle: metadataCommand.Description,
						Score:    100,
						Icon:     plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" width="48" height="48" viewBox="0 0 48 48"><circle cx="22" cy="9" r="2" fill="#c767e5"></circle><circle cx="12" cy="9" r="2" fill="#c767e5"></circle><path fill="#c767e5" d="M34,9c0-1.1-0.9-2-2-2H22l-2,2c0,3,3,3,3,7c0,0.702-0.127,1.374-0.349,2H34V9z"></path><path fill="#c767e5" d="M11,16c0-4,3-4,3-7l-2-2H2C0.9,7,0,7.9,0,9v9h11.349C11.127,17.374,11,16.702,11,16z"></path><path fill="#a238c2" d="M34,39v-9H0v9c0,1.1,0.9,2,2,2h30C33.1,41,34,40.1,34,39z"></path><path fill="#ba54d9" d="M34,29.806c0-1.854,2.204-2.772,3.558-1.507C38.513,29.19,39.75,30,42,30	c3.675,0,6.578-3.303,5.902-7.102c-0.572-3.218-3.665-5.24-6.909-4.838c-1.663,0.206-2.671,0.92-3.479,1.684	C36.185,20.998,34,20.028,34,18.2V18H22.651c-0.825,2.329-3.04,4-5.651,4s-4.827-1.671-5.651-4H0v12h34V29.806z"></path></svg>`),
						Actions: []plugin.QueryResultAction{
							{
								Name:                   "activate",
								PreventHideAfterAction: true,
								Action: func(actionContext plugin.ActionContext) {
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
