package system

import (
	"context"
	"github.com/samber/lo"
	"wox/plugin"
	"wox/util/menus"
)

var menusIcon = plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24"><g fill="#53b9f9"><path d="M8 6.983a1 1 0 1 0 0 2h8a1 1 0 1 0 0-2zM7 12a1 1 0 0 1 1-1h8a1 1 0 1 1 0 2H8a1 1 0 0 1-1-1m1 3.017a1 1 0 1 0 0 2h8a1 1 0 1 0 0-2z"/><path fill-rule="evenodd" d="M22 12c0 5.523-4.477 10-10 10S2 17.523 2 12S6.477 2 12 2s10 4.477 10 10m-2 0a8 8 0 1 1-16 0a8 8 0 0 1 16 0" clip-rule="evenodd"/></g></svg>`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &MenusPlugin{})
}

type MenusPlugin struct {
	api plugin.API
}

func (i *MenusPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "7e17292d-9539-4ed6-b2da-44cb7c585be7",
		Name:          "Macos Menus",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Search menus for current active application",
		Icon:          menusIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		SupportedOS: []string{
			"Macos",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureQueryEnv,
				Params: map[string]string{
					"requireActiveWindowPid": "true",
				},
			},
		},
	}
}

func (i *MenusPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API
}

func (i *MenusPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	icon := menusIcon
	if iconImage, iconErr := getActiveWindowIcon(ctx); iconErr == nil {
		icon = iconImage
	}

	if query.Env.ActiveWindowPid == 0 {
		return []plugin.QueryResult{
			{
				Title:    "No active window",
				SubTitle: "No active window found",
				Icon:     icon,
			},
		}
	}

	menuNames, err := menus.GetAppMenuTitles(query.Env.ActiveWindowPid)
	if err != nil {
		i.api.Log(ctx, plugin.LogLevelError, err.Error())
		return []plugin.QueryResult{}
	}

	filteredMenus := lo.Filter(menuNames, func(menu string, _ int) bool {
		match, score := IsStringMatchScore(ctx, menu, query.Search)
		return match && score > 0
	})

	return lo.Map(filteredMenus, func(menu string, _ int) plugin.QueryResult {
		return plugin.QueryResult{
			Title:    menu,
			SubTitle: "Press Enter to execute",
			Icon:     icon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Execute",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						menus.ExecuteActiveAppMenu(query.Env.ActiveWindowPid, menu)
					},
				},
			},
		}
	})
}
