package system

import (
	"context"
	"github.com/samber/lo"
	"wox/plugin"
	"wox/util/menus"
)

var menusIcon = plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24"><path fill="#53b9f9" d="M12 22c-4.714 0-7.071 0-8.536-1.465C2 19.072 2 16.714 2 12s0-7.071 1.464-8.536C4.93 2 7.286 2 12 2c4.714 0 7.071 0 8.535 1.464C22 4.93 22 7.286 22 12c0 4.714 0 7.071-1.465 8.535C19.072 22 16.714 22 12 22" opacity="0.5"/><path fill="#53b9f9" d="M18.75 8a.75.75 0 0 1-.75.75H6a.75.75 0 0 1 0-1.5h12a.75.75 0 0 1 .75.75m0 4a.75.75 0 0 1-.75.75H6a.75.75 0 0 1 0-1.5h12a.75.75 0 0 1 .75.75m0 4a.75.75 0 0 1-.75.75H6a.75.75 0 0 1 0-1.5h12a.75.75 0 0 1 .75.75"/></svg>`)

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
			"*", "menus",
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
		i.api.Log(ctx, plugin.LogLevelError, "Active window pid is not available")
		return []plugin.QueryResult{}
	}

	menuNames, err := menus.GetAppMenuTitles(query.Env.ActiveWindowPid)
	if err != nil {
		i.api.Log(ctx, plugin.LogLevelError, err.Error())
		return []plugin.QueryResult{}
	}

	filteredMenus := lo.Filter(menuNames, func(menu string, _ int) bool {
		match, score := IsStringMatchScore(ctx, menu, query.Search)
		return (match && score > 10) || (!query.IsGlobalQuery() && query.Search == "")
	})

	return lo.Map(filteredMenus, func(menu string, _ int) plugin.QueryResult {
		return plugin.QueryResult{
			Title: menu,
			Icon:  icon,
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
