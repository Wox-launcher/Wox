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
			if search.IconUrl == "" {
				search.IconUrl = `data:image/png;base64,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`
			}

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
