package system

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/plugin"
	"wox/util"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WebSearchPlugin{})
}

type webSearch struct {
	Url     string
	Title   string
	Keyword string
	IconUrl string
}

type WebSearchPlugin struct {
	api         plugin.API
	webSearches []webSearch
}

func (r *WebSearchPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "c1e350a7-c521-4dc3-b4ff-509f720fde86",
		Name:          "WebSearch",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Provide the web search ability",
		Icon:          "",
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
	}
}

func (r *WebSearchPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.webSearches = r.loadWebSearches(ctx)
	r.api.Log(ctx, fmt.Sprintf("loaded %d web searches", len(r.webSearches)))

	r.webSearches = append(r.webSearches, webSearch{
		Url:     "https://www.google.com/search?q={query}",
		Title:   "Google: {query}",
		Keyword: "g",
		IconUrl: "https://www.google.com/favicon.ico",
	})
}

func (r *WebSearchPlugin) loadWebSearches(ctx context.Context) (webSearches []webSearch) {
	webSearchesJson := r.api.GetSetting(ctx, "webSearches")
	if webSearchesJson == "" {
		return
	}

	unmarshalErr := json.Unmarshal([]byte(webSearchesJson), &webSearches)
	if unmarshalErr != nil {
		r.api.Log(ctx, fmt.Sprintf("failed to unmarshal web searches: %s", unmarshalErr.Error()))
		return
	}

	return
}

func (r *WebSearchPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	queries := strings.Split(query.RawQuery, " ")
	if len(queries) <= 1 {
		return
	}

	triggerKeyword := queries[0]
	otherQuery := strings.Join(queries[1:], " ")

	for _, search := range r.webSearches {
		if strings.ToLower(search.Keyword) == strings.ToLower(triggerKeyword) {
			results = append(results, plugin.QueryResult{
				Title: strings.ReplaceAll(search.Title, "{query}", otherQuery),
				Score: 100,
				Icon:  plugin.NewWoxImageBase64(search.IconUrl),
				Actions: []plugin.QueryResultAction{
					{
						Name: "Search",
						Action: func() {
							util.ShellOpen(strings.ReplaceAll(search.Url, "{query}", otherQuery))
						},
					},
				},
			})
		}
	}

	return
}
