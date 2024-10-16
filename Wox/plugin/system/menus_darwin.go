package system

import (
	"context"
	"wox/plugin"
	"wox/util/menus"

	"github.com/samber/lo"
)

var menusIcon = plugin.PluginMenusIcon

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
		return (match && score > 20) || (!query.IsGlobalQuery() && query.Search == "")
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
