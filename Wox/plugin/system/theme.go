package system

import (
	"context"
	"github.com/samber/lo"
	"wox/plugin"
	"wox/share"
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
		Runtime:       "Nodejs",
		Description:   "Theme manager",
		Icon:          themeIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"theme",
		},
		Commands: []plugin.MetadataCommand{},
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
						Action: func(actionContext plugin.ActionContext) {
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
