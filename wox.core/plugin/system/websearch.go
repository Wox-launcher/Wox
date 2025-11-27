package system

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/selection"
	"wox/util/shell"
)

var webSearchesSettingKey = "webSearches"

var webSearchIcon = common.PluginWebsearchIcon

var defaultWebSearchAddedKey = "defaultWebSearchAdded"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WebSearchPlugin{})
}

type webSearch struct {
	Urls       []string
	Title      string
	Keyword    string
	IsFallback bool //if true, this search will be used when no other search is matched
	Icon       common.WoxImage
	Enabled    bool
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
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Provide the web search ability",
		Icon:          webSearchIcon.String(),
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
				Name: plugin.MetadataFeatureQuerySelection,
			},
		},
		SettingDefinitions: []definition.PluginSettingDefinitionItem{
			{
				Type:               definition.PluginSettingDefinitionTypeTable,
				IsPlatformSpecific: true,
				Value: &definition.PluginSettingValueTable{
					Key:           webSearchesSettingKey,
					Title:         "i18n:plugin_websearch_web_searches",
					SortColumnKey: "Keyword",
					SortOrder:     definition.PluginSettingValueTableSortOrderAsc,
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "Icon",
							Label: "i18n:plugin_websearch_icon",
							Type:  definition.PluginSettingValueTableColumnTypeWoxImage,
							Width: 40,
						},
						{
							Key:     "Keyword",
							Label:   "i18n:plugin_websearch_trigger_keyword",
							Tooltip: "i18n:plugin_websearch_trigger_keyword_tooltip",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
							Width: 60,
						},
						{
							Key:     "Title",
							Label:   "i18n:plugin_websearch_title",
							Tooltip: "i18n:plugin_websearch_title_tooltip",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:         "Urls",
							Label:       "i18n:plugin_websearch_urls",
							Tooltip:     "i18n:plugin_websearch_urls_tooltip",
							HideInTable: true,
							Type:        definition.PluginSettingValueTableColumnTypeTextList,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
							Width: 60,
						},
						{
							Key:   "Enabled",
							Label: "i18n:plugin_websearch_enabled",
							Type:  definition.PluginSettingValueTableColumnTypeCheckbox,
							Width: 60,
						},
						{
							Key:     "IsFallback",
							Label:   "i18n:plugin_websearch_is_fallback",
							Tooltip: "i18n:plugin_websearch_is_fallback_tooltip",
							Type:    definition.PluginSettingValueTableColumnTypeCheckbox,
							Width:   60,
						},
					},
				},
			},
		},
	}
}

func (r *WebSearchPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.webSearches = r.loadWebSearches(ctx)
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("loaded %d web searches", len(r.webSearches)))

	r.api.OnSettingChanged(ctx, func(key string, value string) {
		if key == webSearchesSettingKey {
			r.indexIcons(ctx)
			r.webSearches = r.loadWebSearches(ctx)
		}
	})

	util.Go(ctx, "parse websearch icons", func() {
		r.indexIcons(ctx)
	})
}

func (r *WebSearchPlugin) indexIcons(ctx context.Context) {
	hasAnyIconIndexed := false
	for i, search := range r.webSearches {
		if search.Icon.IsEmpty() {
			r.webSearches[i].Icon = r.indexWebSearchIcon(ctx, search)
			hasAnyIconIndexed = true
		}
	}

	if hasAnyIconIndexed {
		marshal, err := json.Marshal(r.webSearches)
		if err == nil {
			r.api.SaveSetting(ctx, webSearchesSettingKey, string(marshal), false)
		}
	}
}

func (r *WebSearchPlugin) indexWebSearchIcon(ctx context.Context, search webSearch) common.WoxImage {
	// if search url is google, return google icon
	if strings.Contains(search.Urls[0], "google.com") {
		return common.GoogleIcon
	}

	//sort urls, so that we can get the same icon between different runs
	slices.Sort(search.Urls)

	img, err := getWebsiteIconWithCache(ctx, search.Urls[0])
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get icon for %s: %s", search.Urls[0], err.Error()))
		return webSearchIcon
	}

	return img
}

func (r *WebSearchPlugin) loadWebSearches(ctx context.Context) (webSearches []webSearch) {
	webSearchesJson := r.api.GetSetting(ctx, webSearchesSettingKey)
	if webSearchesJson == "" {
		defaultAdded := r.api.GetSetting(ctx, defaultWebSearchAddedKey)
		if defaultAdded == "" {
			webSearches = []webSearch{
				{
					Urls:       []string{"https://www.google.com/search?q={query}"},
					Title:      "Search Google for {query}",
					Keyword:    "g",
					IsFallback: true,
					Enabled:    true,
					Icon:       common.GoogleIcon,
				},
			}
			if marshal, err := json.Marshal(webSearches); err == nil {
				r.api.SaveSetting(ctx, webSearchesSettingKey, string(marshal), false)
				r.api.SaveSetting(ctx, defaultWebSearchAddedKey, "true", false)
			}
		}
		return
	}

	unmarshalErr := json.Unmarshal([]byte(webSearchesJson), &webSearches)
	if unmarshalErr != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to unmarshal web searches: %s", unmarshalErr.Error()))
		return
	}

	return
}

func (r *WebSearchPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	if query.Type == plugin.QueryTypeSelection {
		return r.querySelection(ctx, query)
	}

	queries := strings.Split(query.RawQuery, " ")
	if len(queries) <= 1 {
		return
	}

	triggerKeyword := queries[0]
	otherQuery := strings.Join(queries[1:], " ")

	for _, search := range r.webSearches {
		if !search.Enabled {
			continue
		}
		if strings.ToLower(search.Keyword) == strings.ToLower(triggerKeyword) {
			results = append(results, plugin.QueryResult{Title: r.replaceVariables(ctx, search.Title, otherQuery),
				Score: 100,
				Icon:  search.Icon,
				Actions: []plugin.QueryResultAction{
					{
						Name: "Search",
						Icon: common.SearchIcon,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							util.Go(ctx, "open urls", func() {
								for _, url := range search.Urls {
									shell.Open(r.replaceVariables(ctx, url, otherQuery))
									time.Sleep(time.Millisecond * 100)
								}
							})
						},
					},
				},
			})
		}
	}

	return
}

func (r *WebSearchPlugin) QueryFallback(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	for _, search := range r.webSearches {
		if !search.Enabled {
			continue
		}
		if !search.IsFallback {
			continue
		}

		results = append(results, plugin.QueryResult{
			Title: r.replaceVariables(ctx, search.Title, query.RawQuery),
			Score: 100,
			Icon:  search.Icon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Search",
					Icon: common.SearchIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						for _, url := range search.Urls {
							shell.Open(r.replaceVariables(ctx, url, query.RawQuery))
							time.Sleep(time.Millisecond * 100)
						}
					},
				},
			},
		})
	}

	return results
}

func (r *WebSearchPlugin) querySelection(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	//only support text selection
	if query.Selection.Type == selection.SelectionTypeFile {
		return []plugin.QueryResult{}
	}

	for _, search := range r.webSearches {
		// only show fallback searches
		if !search.IsFallback || !search.Enabled {
			continue
		}

		results = append(results, plugin.QueryResult{Title: r.replaceVariables(ctx, search.Title, query.Selection.Text),
			Icon: search.Icon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "Search",
					Icon: common.SearchIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						for _, url := range search.Urls {
							shell.Open(r.replaceVariables(ctx, url, query.Selection.Text))
							time.Sleep(time.Millisecond * 100)
						}
					},
				},
			},
		})
	}

	return
}

func (r *WebSearchPlugin) replaceVariables(ctx context.Context, text string, query string) string {
	result := strings.ReplaceAll(text, "{query}", query)
	result = strings.ReplaceAll(result, "{lower_query}", strings.ToLower(query))
	result = strings.ReplaceAll(result, "{upper_query}", strings.ToUpper(query))
	return result
}
