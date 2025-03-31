package system

import (
	"context"
	"encoding/json"
	"fmt"
	"wox/common"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/setting/definition"
	"wox/util"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

var themeIcon = plugin.PluginThemeIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ThemePlugin{})
}

type ThemePlugin struct {
	api plugin.API
}

func (c *ThemePlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "58a59382-8b3a-48c2-89ac-0a9a0e12e03f",
		Name:          "Theme manager",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Theme manager",
		Icon:          themeIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"theme",
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     "ai",
				Description: "Generate a new theme with AI",
			},
			{
				Command:     "restore",
				Description: "Remove all custom themes and restore to default",
			},
		},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeSelectAIModel,
				Value: &definition.PluginSettingValueSelectAiModel{
					Key:     "model",
					Label:   "AI model",
					Tooltip: `AI model to use for generating theme.`,
				},
			},
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureAI,
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (c *ThemePlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
}

func (c *ThemePlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Command == "ai" {
		return c.queryAI(ctx, query)
	}
	if query.Command == "restore" {
		return c.queryRestore(ctx, query)
	}

	ui := plugin.GetPluginManager().GetUI()
	return lo.FilterMap(ui.GetAllThemes(ctx), func(theme common.Theme, _ int) (plugin.QueryResult, bool) {
		match, _ := IsStringMatchScore(ctx, theme.ThemeName, query.Search)
		if match {
			result := plugin.QueryResult{
				Title: theme.ThemeName,
				Icon:  common.NewWoxImageTheme(theme),
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "Change theme",
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							ui.ChangeTheme(ctx, theme)
						},
					},
				},
			}
			if theme.IsSystem {
				result.Tails = append(result.Tails, plugin.QueryResultTail{
					Type: plugin.QueryResultTailTypeText,
					Text: "System",
				})
			} else {
				result.Actions = append(result.Actions, plugin.QueryResultAction{
					Name:                   "Uninstall theme",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						ui.UninstallTheme(ctx, theme)
						c.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: fmt.Sprintf("%s ", query.TriggerKeyword),
						})
					},
				})
			}
			currentThemeId := setting.GetSettingManager().GetWoxSetting(ctx).ThemeId
			if currentThemeId == theme.ThemeId {
				result.Group = "Current"
				result.GroupScore = 100
			} else {
				result.Group = "Available"
				result.GroupScore = 50
			}

			return result, true
		} else {
			return plugin.QueryResult{}, false
		}
	})
}

func (c *ThemePlugin) queryAI(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	modelStr := c.api.GetSetting(ctx, "model")
	if modelStr == "" {
		return []plugin.QueryResult{
			{
				Title: "Please select an AI model in theme settings",
				Icon:  themeIcon,
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "Open theme settings",
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx, common.SettingWindowContext{
								Path:  "/plugin/setting",
								Param: c.GetMetadata().Name,
							})
						},
					},
				},
			},
		}
	}
	var aiModel common.Model
	unmarshalErr := json.Unmarshal([]byte(modelStr), &aiModel)
	if unmarshalErr != nil {
		c.api.Notify(ctx, unmarshalErr.Error())
		return []plugin.QueryResult{
			{
				Title:    "Failed to unmarshal model",
				SubTitle: unmarshalErr.Error(),
				Icon:     themeIcon,
			},
		}
	}

	if query.Search == "" {
		return []plugin.QueryResult{
			{
				Title: "Please describe the theme you want to generate",
				Icon:  themeIcon,
			},
		}
	}

	embedThemes := resource.GetEmbedThemes(ctx)
	if len(embedThemes) == 0 {
		return []plugin.QueryResult{
			{
				Title: "No embed theme found",
				Icon:  themeIcon,
			},
		}
	}

	exampleThemeJson := embedThemes[0]

	var conversations []common.Conversation
	conversations = append(conversations, common.Conversation{
		Role: common.ConversationRoleUser,
		Text: `
					我正在编写Wox的主题, 该主题是由一段json组成, 例如：` + exampleThemeJson + `

					现在我想让你根据上面的格式生成一个新的主题，主题的要求是：` + query.Search + `。

					有一些注意点需要你遵守：
					1. 你的回答结果必须是JSON格式, 以{开头, 以}结尾. 忽略解释，注释等信息
					2. 主题名称你自己决定, 但是必须有意义
					3. 背景颜色跟字体颜色需要有区分度，不要让这两者的颜色太接近(这里包括正常未选中的结果与被高亮选中的结果)
					`,
	})

	onAnswering := func(current plugin.RefreshableResult, deltaAnswer string, isFinished bool) plugin.RefreshableResult {
		current.SubTitle = "Generating..."
		current.Preview.PreviewData += deltaAnswer
		current.Preview.ScrollPosition = plugin.WoxPreviewScrollPositionBottom

		if isFinished {
			current.RefreshInterval = 0 // stop refreshing
			current.SubTitle = "Theme generated"

			var themeJson = current.Preview.PreviewData
			util.Go(ctx, "theme generated", func() {
				// use regex to get json snippet from the whole text
				group := util.FindRegexGroup(`(?ms){(?P<json>.*?)}`, themeJson)
				if len(group) == 0 {
					c.api.Notify(ctx, "Failed to extract json")
					return
				}

				var jsonTheme = fmt.Sprintf("{%s}", group["json"])
				var theme common.Theme
				unmarshalErr := json.Unmarshal([]byte(jsonTheme), &theme)
				if unmarshalErr != nil {
					c.api.Notify(ctx, unmarshalErr.Error())
					return
				}

				theme.ThemeId = uuid.NewString()
				theme.ThemeAuthor = "Wox launcher AI"
				theme.ThemeUrl = "https://www.github.com/wox-launcher/wox"
				theme.Version = "1.0.0"
				theme.IsSystem = false
				plugin.GetPluginManager().GetUI().InstallTheme(ctx, theme)
			})
		}

		return current
	}
	onAnswerErr := func(current plugin.RefreshableResult, err error) plugin.RefreshableResult {
		current.Preview.PreviewData += fmt.Sprintf("\n\nError: %s", err.Error())
		current.RefreshInterval = 0 // stop refreshing
		return current
	}

	startGenerate := false
	return []plugin.QueryResult{
		{
			Title:           "Generate theme with ai",
			SubTitle:        "Enter to generate",
			Icon:            themeIcon,
			Preview:         plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: ""},
			RefreshInterval: 100,
			OnRefresh: createLLMOnRefreshHandler(ctx, c.api.AIChatStream, aiModel, conversations, common.EmptyChatOptions, func() bool {
				return startGenerate
			}, nil, onAnswering, onAnswerErr),
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "Apply",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						startGenerate = true
					},
				},
			},
		},
	}
}

func (c *ThemePlugin) queryRestore(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	return []plugin.QueryResult{
		{
			Title: "Remove all custom themes and restore to default",
			Icon:  themeIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "Restore",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						plugin.GetPluginManager().GetUI().RestoreTheme(ctx)
					},
				},
			},
		},
	}
}
