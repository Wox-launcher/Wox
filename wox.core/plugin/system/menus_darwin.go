package system

import (
	"context"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/menus"

	"github.com/samber/lo"
)

var menusIcon = common.PluginMenusIcon
var menusCacheTTL = time.Minute
var menusCache = util.NewHashMap[int, menusCacheEntry]()

type menusCacheEntry struct {
	titles    []string
	expiresAt time.Time
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &MenusPlugin{})
}

type MenusPlugin struct {
	api plugin.API
}

func (i *MenusPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "7e17292d-9539-4ed6-b2da-44cb7c585be7",
		Name:          "i18n:plugin_menus_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_menus_plugin_description",
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
				Params: map[string]any{
					"requireActiveWindowPid":  "true",
					"requireActiveWindowIcon": "true",
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
	if !query.Env.ActiveWindowIcon.IsEmpty() {
		icon = query.Env.ActiveWindowIcon
	}

	if query.Env.ActiveWindowPid == 0 {
		i.api.Log(ctx, plugin.LogLevelError, "Active window pid is not available")
		return []plugin.QueryResult{}
	}

	menuNames, ok := getMenusFromCache(query.Env.ActiveWindowPid)
	if !ok {
		var err error
		menuNames, err = menus.GetAppMenuTitles(query.Env.ActiveWindowPid)
		if err != nil {
			i.api.Log(ctx, plugin.LogLevelError, err.Error())
			return []plugin.QueryResult{}
		}
		storeMenusCache(query.Env.ActiveWindowPid, menuNames)
	}

	filteredMenus := lo.Filter(menuNames, func(menu string, _ int) bool {
		match, score := plugin.IsStringMatchScore(ctx, menu, query.Search)
		return (match && score > 20) || (!query.IsGlobalQuery() && query.Search == "")
	})

	return lo.Map(filteredMenus, func(menu string, _ int) plugin.QueryResult {
		return plugin.QueryResult{
			Title: menu,
			Icon:  icon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_menus_execute",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						menus.ExecuteActiveAppMenu(query.Env.ActiveWindowPid, menu)
					},
				},
			},
		}
	})
}

func getMenusFromCache(pid int) ([]string, bool) {
	entry, ok := menusCache.Load(pid)
	if !ok || time.Now().After(entry.expiresAt) {
		menusCache.Delete(pid)
		return nil, false
	}
	return entry.titles, true
}

func storeMenusCache(pid int, titles []string) {
	menusCache.Store(pid, menusCacheEntry{
		titles:    titles,
		expiresAt: time.Now().Add(menusCacheTTL),
	})
}
