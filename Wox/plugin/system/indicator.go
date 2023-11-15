package system

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"wox/i18n"
	"wox/plugin"
	"wox/share"
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
				Icon:     plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYAAABw4pVUAAAACXBIWXMAAAsTAAALEwEAmpwYAAACVklEQVR4nO2cQUrdUBSG30aae1Md5RZUcFSecCaOWtfRtw+ltHYB2g2cXWgp4kTX4CKqzlOS+AqVQqG+l//05fvgzIQcv48kjwzubAYAAAAAAAAAAAAAAAAAsBZSbq5TXdoXTy5Xm7SLjJUIqIfZpF1kLP+Bt8ff/3nSioNE2EVGJAkp0C4yIklIgXaREUlCCrSLjEgSUqBdZESSkALtIiOShBRol7Xyqi5HVd18S3XzuMrf+um/muaxqpvLqi7vpTGqXD7qZZRQU+XmRHZn9Ets77S7iy/t/PymNb+b5MzPb9rdxWnvoo+iuFOGx1TpF1ELsSCzt/g83Cm5XIweJOXy0F384OutXIQFmc7F8Ngq9+MHeXpmqiVYsFl6CRVka/9w9Jfp1v7hX/d4/jcv2fNP1wsbZMqTCHInj0AQ14sniOtlE8T1ggnieqkECSDSCKKXZwTRCzOC6CUZQfRijCB6GRZg+HTi+ggEcb14grheNkFcL5ggrpdKkAAijSB6eUYQvTAjiF6SEUQvxgiil2EBhk8nro9AENeLJ4jrZRPE9YIJ4nqpBAkg0giil2cE0QszguglGUH0YowgehkWYPh04voIBHG9eIK4XjZBXC+YIK6XSpAAIm2TgygODkgBDioIG2TKk2RBng6fmfKxTPZs5me/Dp/5MXqQ7gS17uLdkURqERZk9j580h3P1B3S1V98e6ePMuU7ZX52O8RYHmD2+s270YP0UXJzon65pnDTHEti/Han5HKxfKdMcnJ56BzI7gwAAAAAAAAAAAAAAAAAAJitgZ+lmGHcVGx/0gAAAABJRU5ErkJggg==`),
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "activate",
						PreventHideAfterAction: true,
						Action: func(actionContext plugin.ActionContext) {
							i.api.ChangeQuery(ctx, share.ChangeQueryParam{
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
					SubTitle: pluginInstance.Metadata.Description,
					Icon:     plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYAAABw4pVUAAAACXBIWXMAAAsTAAALEwEAmpwYAAACVklEQVR4nO2cQUrdUBSG30aae1Md5RZUcFSecCaOWtfRtw+ltHYB2g2cXWgp4kTX4CKqzlOS+AqVQqG+l//05fvgzIQcv48kjwzubAYAAAAAAAAAAAAAAAAAsBZSbq5TXdoXTy5Xm7SLjJUIqIfZpF1kLP+Bt8ff/3nSioNE2EVGJAkp0C4yIklIgXaREUlCCrSLjEgSUqBdZESSkALtIiOShBRol7Xyqi5HVd18S3XzuMrf+um/muaxqpvLqi7vpTGqXD7qZZRQU+XmRHZn9Ets77S7iy/t/PymNb+b5MzPb9rdxWnvoo+iuFOGx1TpF1ELsSCzt/g83Cm5XIweJOXy0F384OutXIQFmc7F8Ngq9+MHeXpmqiVYsFl6CRVka/9w9Jfp1v7hX/d4/jcv2fNP1wsbZMqTCHInj0AQ14sniOtlE8T1ggnieqkECSDSCKKXZwTRCzOC6CUZQfRijCB6GRZg+HTi+ggEcb14grheNkFcL5ggrpdKkAAijSB6eUYQvTAjiF6SEUQvxgiil2EBhk8nro9AENeLJ4jrZRPE9YIJ4nqpBAkg0giil2cE0QszguglGUH0YowgehkWYPh04voIBHG9eIK4XjZBXC+YIK6XSpAAIm2TgygODkgBDioIG2TKk2RBng6fmfKxTPZs5me/Dp/5MXqQ7gS17uLdkURqERZk9j580h3P1B3S1V98e6ePMuU7ZX52O8RYHmD2+s270YP0UXJzon65pnDTHEti/Han5HKxfKdMcnJ56BzI7gwAAAAAAAAAAAAAAAAAAJitgZ+lmGHcVGx/0gAAAABJRU5ErkJggg==`),
					Actions: []plugin.QueryResultAction{
						{
							Name:                   "activate",
							PreventHideAfterAction: true,
							Action: func(actionContext plugin.ActionContext) {
								i.api.ChangeQuery(ctx, share.ChangeQueryParam{
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

func (i *IndicatorPlugin) queryForCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	for _, pluginInstance := range plugin.GetPluginManager().GetPluginInstances() {
		_, found := lo.Find(pluginInstance.GetTriggerKeywords(), func(triggerKeyword string) bool {
			return IsStringMatchNoPinYin(ctx, triggerKeyword, query.TriggerKeyword)
		})
		if found {
			for _, metadataCommandShadow := range pluginInstance.GetQueryCommands() {
				// action will be executed in another go routine, so we need to copy the variable
				metadataCommand := metadataCommandShadow
				if query.Search == "" || IsStringMatchNoPinYin(ctx, metadataCommand.Command, query.Search) {
					results = append(results, plugin.QueryResult{
						Id:       uuid.NewString(),
						Title:    fmt.Sprintf("%s %s ", query.TriggerKeyword, metadataCommand.Command),
						SubTitle: metadataCommand.Description,
						Score:    100,
						Icon:     plugin.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYAAABw4pVUAAAACXBIWXMAAAsTAAALEwEAmpwYAAACVklEQVR4nO2cQUrdUBSG30aae1Md5RZUcFSecCaOWtfRtw+ltHYB2g2cXWgp4kTX4CKqzlOS+AqVQqG+l//05fvgzIQcv48kjwzubAYAAAAAAAAAAAAAAAAAsBZSbq5TXdoXTy5Xm7SLjJUIqIfZpF1kLP+Bt8ff/3nSioNE2EVGJAkp0C4yIklIgXaREUlCCrSLjEgSUqBdZESSkALtIiOShBRol7Xyqi5HVd18S3XzuMrf+um/muaxqpvLqi7vpTGqXD7qZZRQU+XmRHZn9Ets77S7iy/t/PymNb+b5MzPb9rdxWnvoo+iuFOGx1TpF1ELsSCzt/g83Cm5XIweJOXy0F384OutXIQFmc7F8Ngq9+MHeXpmqiVYsFl6CRVka/9w9Jfp1v7hX/d4/jcv2fNP1wsbZMqTCHInj0AQ14sniOtlE8T1ggnieqkECSDSCKKXZwTRCzOC6CUZQfRijCB6GRZg+HTi+ggEcb14grheNkFcL5ggrpdKkAAijSB6eUYQvTAjiF6SEUQvxgiil2EBhk8nro9AENeLJ4jrZRPE9YIJ4nqpBAkg0giil2cE0QszguglGUH0YowgehkWYPh04voIBHG9eIK4XjZBXC+YIK6XSpAAIm2TgygODkgBDioIG2TKk2RBng6fmfKxTPZs5me/Dp/5MXqQ7gS17uLdkURqERZk9j580h3P1B3S1V98e6ePMuU7ZX52O8RYHmD2+s270YP0UXJzon65pnDTHEti/Han5HKxfKdMcnJ56BzI7gwAAAAAAAAAAAAAAAAAAJitgZ+lmGHcVGx/0gAAAABJRU5ErkJggg==`),
						Actions: []plugin.QueryResultAction{
							{
								Name:                   "activate",
								PreventHideAfterAction: true,
								Action: func(actionContext plugin.ActionContext) {
									i.api.ChangeQuery(ctx, share.ChangeQueryParam{
										QueryType: plugin.QueryTypeInput,
										QueryText: fmt.Sprintf("%s %s ", query.TriggerKeyword, metadataCommand.Command),
									})
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
