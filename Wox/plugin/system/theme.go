package system

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"wox/ai"
	"wox/plugin"
	"wox/resource"
	"wox/share"
	"wox/util"
)

var themeIcon = plugin.NewWoxImageBase64(`data:image/png;base64,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`)

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

	ui := plugin.GetPluginManager().GetUI()
	return lo.FilterMap(ui.GetAllThemes(ctx), func(theme share.Theme, _ int) (plugin.QueryResult, bool) {
		match, _ := IsStringMatchScore(ctx, theme.ThemeName, query.Search)
		if match {
			return plugin.QueryResult{
				Title: theme.ThemeName,
				Icon:  themeIcon,
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "Change theme",
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							ui.ChangeTheme(ctx, theme)
						},
					},
				},
			}, true
		} else {
			return plugin.QueryResult{}, false
		}
	})
}

func (c *ThemePlugin) queryAI(ctx context.Context, query plugin.Query) []plugin.QueryResult {
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

	var conversations []ai.Conversation
	conversations = append(conversations, ai.Conversation{
		Role: ai.ConversationRoleUser,
		Text: `
我正在编写Wox的主题，该主题是由一段json组成，例如：` + exampleThemeJson + `

现在我想让你根据上面的格式生成一个新的主题，主题的要求是：` + query.Search + `。

有一些注意点需要你遵守：
1. 你的回答结果必须是JSON格式,以{开头,以}结尾. 忽略解释，注释等信息
2. 主题名称你自己决定,但是必须有意义
3. 背景跟字体需要有区分度，不要让背景跟字体颜色太接近(这里包括正常未选中的结果与被高亮选中的结果)
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
					c.api.Notify(ctx, "Failed to extract json", "")
					return
				}

				var jsonTheme = fmt.Sprintf("{%s}", group["json"])
				var theme share.Theme
				unmarshalErr := json.Unmarshal([]byte(jsonTheme), &theme)
				if unmarshalErr != nil {
					c.api.Notify(ctx, "Failed to unmarshal theme json", unmarshalErr.Error())
					return
				}

				theme.ThemeId = uuid.NewString()
				theme.ThemeAuthor = "Wox launcher AI"
				theme.ThemeUrl = "https://www.github.com/wox-launcher/wox"
				theme.Version = "1.0.0"
				theme.IsSystem = false
				theme.ScreenshotUrls = []string{}
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
			OnRefresh: createLLMOnRefreshHandler(ctx, c.api.AIChatStream, ai.Model{}, conversations, func() bool {
				return startGenerate
			}, onAnswering, onAnswerErr),
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
