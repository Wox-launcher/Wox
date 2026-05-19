package system

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/shell"

	"github.com/samber/lo"
)

var urlIcon = common.PluginUrlIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &UrlPlugin{})
}

type UrlHistory struct {
	Url   string
	Icon  common.WoxImage
	Title string
}

type UrlPlugin struct {
	api        plugin.API
	recentUrls []UrlHistory
}

func (r *UrlPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "1af58721-6c97-4901-b291-620daf08d9c9",
		Name:          "i18n:plugin_url_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_url_plugin_description",
		Icon:          urlIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureMRU,
			},
		},
	}
}

func (r *UrlPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.recentUrls = r.loadRecentUrls(ctx)

	r.api.OnMRURestore(ctx, r.handleMRURestore)
}

func (r *UrlPlugin) loadRecentUrls(ctx context.Context) []UrlHistory {
	urlsJson := r.api.GetSetting(ctx, "recentUrls")
	if urlsJson == "" {
		return []UrlHistory{}
	}

	var urls []UrlHistory
	err := json.Unmarshal([]byte(urlsJson), &urls)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("load recent urls error: %s", err.Error()))
		return []UrlHistory{}
	}

	return urls
}

func (r *UrlPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	var results []plugin.QueryResult
	if len(query.Search) >= 2 {
		existingUrlHistory := lo.Filter(r.recentUrls, func(item UrlHistory, index int) bool {
			return strings.Contains(strings.ToLower(item.Url), strings.ToLower(query.Search))
		})

		for _, history := range existingUrlHistory {
			icon := r.getRecentUrlIcon(ctx, history)
			displayIcon := urlIcon
			if icon.IsValid() && icon != urlIcon {
				displayIcon = icon.Overlay(urlIcon, 0.4, 0.6, 0.6)
			}

			contextData := common.ContextData{
				"url":   history.Url,
				"title": history.Title,
				"type":  "history",
			}

			results = append(results, plugin.QueryResult{
				Title:    history.Url,
				SubTitle: history.Title,
				Score:    100,
				Icon:     displayIcon,
				Actions: []plugin.QueryResultAction{
					{
						Name:        "i18n:plugin_url_open",
						Icon:        common.OpenIcon,
						ContextData: contextData,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							openErr := shell.Open(history.Url)
							if openErr != nil {
								r.api.Log(ctx, "Error opening URL", openErr.Error())
							}
						},
					},
					{
						Name:        "i18n:plugin_url_remove",
						Icon:        common.TrashIcon,
						ContextData: contextData,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							r.removeRecentUrl(ctx, history.Url)
						},
					},
				},
			})
		}
	}

	if util.IsUrl(query.Search) {
		// Feature change: URL detection is shared with clipboard link records,
		// so both entry points accept and normalize the same direct URL shapes.
		normalizedURL := util.NormalizeUrl(query.Search)
		contextData := common.ContextData{
			"url":   normalizedURL,
			"title": "",
			"type":  "direct",
		}

		results = append(results, plugin.QueryResult{
			Title:    query.Search,
			SubTitle: "i18n:plugin_url_open_in_browser",
			Score:    100,
			Icon:     urlIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name:        "i18n:plugin_url_open",
					Icon:        urlIcon,
					ContextData: contextData,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						openErr := shell.Open(normalizedURL)
						if openErr != nil {
							r.api.Log(ctx, "Error opening URL", openErr.Error())
						} else {
							util.Go(ctx, "saveRecentUrl", func() {
								r.saveRecentUrl(ctx, normalizedURL)
							})
						}
					},
				},
			},
		})
	}
	return plugin.NewQueryResponse(results)
}

func (r *UrlPlugin) saveRecentUrl(ctx context.Context, url string) {
	icon, err := getWebsiteIconWithCache(ctx, url)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("get url icon error: %s", err.Error()))
		icon = urlIcon
	}

	title := ""
	body, err := util.HttpGet(ctx, url)
	if err == nil {
		titleStart := strings.Index(string(body), "<title>")
		titleEnd := strings.Index(string(body), "</title>")
		if titleStart != -1 && titleEnd != -1 {
			title = string(body[titleStart+7 : titleEnd])
		}
	} else {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("get url title error: %s", err.Error()))
	}

	newHistory := UrlHistory{
		Url:   url,
		Icon:  icon,
		Title: title,
	}

	// remove duplicate urls
	r.recentUrls = lo.Filter(r.recentUrls, func(item UrlHistory, index int) bool {
		return item.Url != url
	})
	r.recentUrls = append([]UrlHistory{newHistory}, r.recentUrls...)

	urlsJson, err := json.Marshal(r.recentUrls)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("save url setting error: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, "recentUrls", string(urlsJson), false)
}

func (r *UrlPlugin) getRecentUrlIcon(ctx context.Context, history UrlHistory) common.WoxImage {
	if history.Icon.IsValid() {
		return history.Icon
	}

	icon, err := getWebsiteIconWithCache(ctx, history.Url)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("recover url icon error: %s", err.Error()))
		return urlIcon
	}

	r.updateRecentUrlIcon(ctx, history.Url, icon)
	return icon
}

func (r *UrlPlugin) updateRecentUrlIcon(ctx context.Context, url string, icon common.WoxImage) {
	updated := false
	for i := range r.recentUrls {
		if r.recentUrls[i].Url == url {
			r.recentUrls[i].Icon = icon
			updated = true
			break
		}
	}

	if !updated {
		return
	}

	urlsJson, err := json.Marshal(r.recentUrls)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("save url setting error: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, "recentUrls", string(urlsJson), false)
}

func (r *UrlPlugin) removeRecentUrl(ctx context.Context, url string) {
	r.recentUrls = lo.Filter(r.recentUrls, func(item UrlHistory, index int) bool {
		return item.Url != url
	})

	urlsJson, err := json.Marshal(r.recentUrls)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("save url setting error: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, "recentUrls", string(urlsJson), false)
}

func (r *UrlPlugin) handleMRURestore(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	url := mruData.ContextData["url"]
	title := mruData.ContextData["title"]
	typeStr := mruData.ContextData["type"]
	if url == "" {
		return nil, fmt.Errorf("empty url in context data")
	}
	url = util.NormalizeUrl(url)

	// user may have cleared icon cache, so we need to get icon again
	if !mruData.Icon.IsValid() {
		icon, err := getWebsiteIconWithCache(ctx, url)
		if err != nil {
			r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("get url icon error: %s", err.Error()))
			icon = urlIcon
		}
		mruData.Icon = icon
	}

	result := &plugin.QueryResult{
		Title:    url,
		SubTitle: title,
		Icon:     mruData.Icon,
	}

	if typeStr == "history" {
		found := false
		for _, history := range r.recentUrls {
			if history.Url == url {
				found = true
				result.SubTitle = history.Title
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("URL no longer in history: %s", url)
		}

		result.Actions = []plugin.QueryResultAction{
			{
				Name:        "i18n:plugin_url_open",
				Icon:        common.OpenIcon,
				ContextData: mruData.ContextData,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					openErr := shell.Open(url)
					if openErr != nil {
						r.api.Log(ctx, "Error opening URL", openErr.Error())
					}
				},
			},
			{
				Name:        "i18n:plugin_url_remove",
				Icon:        common.TrashIcon,
				ContextData: mruData.ContextData,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					r.removeRecentUrl(ctx, url)
				},
			},
		}
	} else {
		result.Icon = common.OpenIcon
		result.SubTitle = "i18n:plugin_url_open_in_browser"
		result.Actions = []plugin.QueryResultAction{
			{
				Name:        "i18n:plugin_url_open",
				Icon:        urlIcon,
				ContextData: mruData.ContextData,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					openErr := shell.Open(url)
					if openErr != nil {
						r.api.Log(ctx, "Error opening URL", openErr.Error())
					} else {
						util.Go(ctx, "saveRecentUrl", func() {
							r.saveRecentUrl(ctx, url)
						})
					}
				},
			},
		}
	}

	return result, nil
}
