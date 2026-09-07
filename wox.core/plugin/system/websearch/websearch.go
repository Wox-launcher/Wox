package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/browser"
	"wox/util/selection"
)

const (
	webSearchDefaultBrowserSettingKey = "defaultBrowser"

	webSearchBrowserSystem     = "system"  // follow the system default browser
	webSearchBrowserUseDefault = "default" // follow the default browser setting of the plugin, which is webSearchBrowserSystem or a specific browser
)

var webSearchesSettingKey = "webSearches"

var webSearchIcon = common.PluginWebsearchIcon

var defaultWebSearchAddedKey = "defaultWebSearchAdded"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WebSearchPlugin{})
}

type webSearch struct {
	Urls              []string
	Title             string
	Keyword           string
	Browser           string
	IsFallback        bool //if true, this search will be used when no other search is matched
	Icon              common.WoxImage
	Enabled           bool
	triggerRegistered bool
}

type WebSearchPlugin struct {
	api                plugin.API
	webSearches        []webSearch
	registeredKeywords []string
}

func (r *WebSearchPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "c1e350a7-c521-4dc3-b4ff-509f720fde86",
		Name:          "i18n:plugin_websearch_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_websearch_plugin_description",
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
				Type: definition.PluginSettingDefinitionTypeSelect,
				Value: &definition.PluginSettingValueSelect{
					Key:          webSearchDefaultBrowserSettingKey,
					Label:        "i18n:plugin_websearch_default_browser",
					Tooltip:      "i18n:plugin_websearch_default_browser_tooltip",
					DefaultValue: webSearchBrowserSystem,
					Options:      r.getWebSearchDefaultBrowserOptions(),
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:           webSearchesSettingKey,
					Title:         "i18n:plugin_websearch_web_searches",
					SortColumnKey: "Keyword",
					SortOrder:     definition.PluginSettingValueTableSortOrderAsc,
					MaxHeight:     500,
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
							Key:               "Title",
							Label:             "i18n:plugin_websearch_title",
							Tooltip:           "i18n:plugin_websearch_title_tooltip",
							Type:              definition.PluginSettingValueTableColumnTypeQueryVariable,
							QueryVariableKind: definition.PluginSettingQueryVariableKindWebSearch,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:               "Urls",
							Label:             "i18n:plugin_websearch_urls",
							Tooltip:           "i18n:plugin_websearch_urls_tooltip",
							HideInTable:       true,
							Type:              definition.PluginSettingValueTableColumnTypeQueryVariableList,
							QueryVariableKind: definition.PluginSettingQueryVariableKindWebSearch,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
							Width: 60,
						},
						{
							Key:           "Browser",
							Label:         "i18n:plugin_websearch_browser",
							Tooltip:       "i18n:plugin_websearch_browser_tooltip",
							Type:          definition.PluginSettingValueTableColumnTypeSelect,
							Width:         100,
							SelectOptions: r.getWebSearchItemBrowserOptions(),
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
	r.registerTriggerKeywords(ctx)
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("loaded %d web searches", len(r.webSearches)))

	r.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key == webSearchesSettingKey {
			r.webSearches = r.loadWebSearches(callbackCtx)
			r.registerTriggerKeywords(callbackCtx)
			r.indexIcons(callbackCtx)
		}
	})

	util.Go(ctx, "parse websearch icons", func() {
		r.indexIcons(ctx)
	})
}

// registerTriggerKeywords makes enabled searches participate in core scoped routing.
func (r *WebSearchPlugin) registerTriggerKeywords(ctx context.Context) {
	var keywords []string
	parameterSets := make(map[string][]string)
	variableSets := make(map[string][]plugin.QueryVariable)
	for i := range r.webSearches {
		search := &r.webSearches[i]
		search.triggerRegistered = false
		if search.Enabled {
			names, err := search.parameters()
			if err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("invalid web search %q: %v", search.Keyword, err))
				continue
			}
			// A keyword has one hint. Entries sharing it must agree on input names and order.
			if previous, exists := parameterSets[search.Keyword]; exists && !slices.Equal(previous, names) {
				util.GetLogger().Warn(ctx, fmt.Sprintf("web searches sharing keyword %q have different parameters", search.Keyword))
				continue
			}
			parameterSets[search.Keyword] = names
			for _, variable := range search.queryVariables() {
				if !slices.Contains(variableSets[search.Keyword], variable) {
					variableSets[search.Keyword] = append(variableSets[search.Keyword], variable)
				}
			}
			search.triggerRegistered = r.api.RegisterTriggerKeyword(ctx, plugin.RegisterTriggerKeywordOption{
				Keyword: search.Keyword, QueryHint: search.queryHint(names), QueryVariables: variableSets[search.Keyword],
			}).Success
			if search.triggerRegistered {
				keywords = append(keywords, search.Keyword)
			} else {
				util.GetLogger().Warn(ctx, fmt.Sprintf("failed to register web search trigger keyword %q: invalid or already occupied", search.Keyword))
			}
		}
	}
	for _, keyword := range r.registeredKeywords {
		if !slices.Contains(keywords, keyword) {
			r.api.UnregisterTriggerKeyword(ctx, plugin.UnregisterTriggerKeywordOption{Keyword: keyword})
		}
	}
	r.registeredKeywords = keywords
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
	if len(search.Urls) == 0 {
		return webSearchIcon
	}
	// if search url is google, return google icon
	if strings.Contains(search.Urls[0], "google.com") {
		return common.GoogleIcon
	}

	// Preserve URL order because it defines parameter order.
	urls := slices.Clone(search.Urls)
	slices.Sort(urls)

	img, err := system.GetWebsiteIconWithCache(ctx, urls[0])
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
					Urls:       []string{"https://www.google.com/search?q={wox:parameter?name=query}"},
					Title:      "Search Google for {wox:parameter?name=query}",
					Keyword:    "g",
					Browser:    webSearchBrowserSystem,
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

// Query reads named slots without splitting their text values into words.
func (r *WebSearchPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.Type == plugin.QueryTypeSelection {
		return plugin.NewQueryResponse(r.querySelection(ctx, query))
	}
	var results []plugin.QueryResult
	for _, search := range r.webSearches {
		if !search.Enabled || !search.triggerRegistered || query.TriggerKeyword != search.Keyword {
			continue
		}
		names, err := search.parameters()
		if err != nil {
			continue
		}
		values, complete := search.parameterValues(query, names)
		if !complete {
			results = append(results, plugin.QueryResult{
				Title: "i18n:plugin_websearch_fill_parameters", SubTitle: strings.Join(names, " · "), Icon: search.Icon,
				Actions: []plugin.QueryResultAction{{Name: "i18n:plugin_websearch_fill_parameters", PreventHideAfterAction: true,
					Action: func(ctx context.Context, _ plugin.ActionContext) {
						hint := search.queryHint(names)
						for i := range hint.Elements {
							if hint.Elements[i].Kind == common.QueryElementArgument && query.QueryHint != nil {
								hint.Elements[i].Value = query.QueryHint.Argument(hint.Elements[i].Id)
							}
						}
						// A pasted plain query is kept whole in the first slot; its boundaries are unknown.
						if query.QueryHint == nil {
							_, hint.Elements[0].Value, _ = strings.Cut(query.RawQuery, " ")
						}
						hint.Elements = append([]common.QueryElement{{Id: "command", Kind: common.QueryElementText, Text: search.Keyword + " "}}, hint.Elements...)
						r.api.ChangeQuery(ctx, common.PlainQuery{QueryType: plugin.QueryTypeInput, QueryText: hint.PlainText(), QueryHint: hint})
					},
				}},
			})
			continue
		}
		results = append(results, r.searchResult(ctx, search, values, query.QueryVariables, nil))
	}
	return plugin.NewQueryResponse(results)
}

// QueryFallback only maps a complete raw query to an unambiguous single input.
func (r *WebSearchPlugin) QueryFallback(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	return r.singleParameterResults(ctx, query.RawQuery, query.QueryVariables, nil)
}

func (r *WebSearchPlugin) querySelection(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Selection.Type != selection.SelectionTypeText {
		return nil
	}
	return r.singleParameterResults(ctx, query.Selection.Text, query.QueryVariables, &query.Selection.Text)
}

// singleParameterResults applies the same eligibility rule to fallback and text selection.
func (r *WebSearchPlugin) singleParameterResults(ctx context.Context, text string, environment map[string]string, selectedText *string) []plugin.QueryResult {
	var results []plugin.QueryResult
	for _, search := range r.webSearches {
		if !search.Enabled || !search.IsFallback {
			continue
		}
		names, err := search.parameters()
		if err != nil || len(names) != 1 {
			continue
		}
		values := map[string]string{plugin.ParameterQueryVariable(names[0]): text}
		results = append(results, r.searchResult(ctx, search, values, environment, selectedText))
	}
	return results
}

// searchResult binds preview and execution to the same immutable environment values.
func (r *WebSearchPlugin) searchResult(ctx context.Context, search webSearch, values, environment map[string]string, selectedText *string) plugin.QueryResult {
	variables := search.queryVariables()
	if len(variables) > 0 {
		for _, variable := range variables {
			value, exists := environment[variable]
			if variable == plugin.QueryVariableSelectedText && selectedText != nil {
				value, exists = *selectedText, true
			}
			if !exists || value == "" {
				return plugin.QueryResult{Title: "i18n:plugin_websearch_missing_context", SubTitle: variable, Icon: search.Icon}
			}
			values[variable] = value
		}
	}
	return plugin.QueryResult{
		Title: renderWebSearchTemplate(search.Title, values, false), Score: 100, Icon: search.Icon,
		Actions: []plugin.QueryResultAction{{Name: "i18n:plugin_websearch_search", Icon: common.SearchIcon,
			Action: func(ctx context.Context, _ plugin.ActionContext) {
				util.Go(ctx, "open web search urls", func() { r.openSearchUrls(ctx, search, values) })
			},
		}},
	}
}

// openSearchUrls encodes substituted values for their URL component, never the template itself.
func (r *WebSearchPlugin) openSearchUrls(ctx context.Context, search webSearch, values map[string]string) {
	configuredDefaultBrowser := r.api.GetSetting(ctx, webSearchDefaultBrowserSettingKey)
	browser := r.resolveWebSearchBrowser(search.Browser, configuredDefaultBrowser)
	for _, template := range search.Urls {
		resolvedURL := renderWebSearchTemplate(template, values, true)
		if err := r.openURLInWebSearchBrowser(resolvedURL, browser); err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to open web search URL: %v", err))
		}
		time.Sleep(time.Millisecond * 100)
	}
}
func (r *WebSearchPlugin) resolveWebSearchBrowser(itemBrowser string, defaultBrowser string) string {
	normalizedItemBrowser := browser.NormalizeBrowserID(itemBrowser)
	normalizedDefaultBrowser := browser.NormalizeBrowserID(defaultBrowser)
	if normalizedDefaultBrowser == "" {
		normalizedDefaultBrowser = webSearchBrowserSystem
	}

	switch normalizedItemBrowser {
	case "", webSearchBrowserUseDefault:
		return normalizedDefaultBrowser
	default:
		return normalizedItemBrowser
	}
}

func (r *WebSearchPlugin) getWebSearchDefaultBrowserOptions() []definition.PluginSettingValueSelectOption {
	options := []definition.PluginSettingValueSelectOption{
		{Label: "i18n:plugin_websearch_browser_system_default", Value: webSearchBrowserSystem, Icon: common.PluginBrowserIcon},
	}

	for _, localBrowser := range browser.GetInstalledBrowsers() {
		options = append(options, definition.PluginSettingValueSelectOption{
			Label: localBrowser.Label,
			Value: localBrowser.ID,
			Icon:  localBrowser.Icon,
		})
	}

	return options
}

func (r *WebSearchPlugin) getWebSearchItemBrowserOptions() []definition.PluginSettingValueSelectOption {
	options := []definition.PluginSettingValueSelectOption{
		{Label: "i18n:plugin_websearch_browser_use_default", Value: webSearchBrowserUseDefault, Icon: common.PluginBrowserIcon},
	}

	for _, localBrowser := range browser.GetInstalledBrowsers() {
		options = append(options, definition.PluginSettingValueSelectOption{
			Label: localBrowser.Label,
			Value: localBrowser.ID,
			Icon:  localBrowser.Icon,
		})
	}

	return options
}

func (r *WebSearchPlugin) openURLInWebSearchBrowser(url string, browserId string) error {
	normalizedBrowser := browser.NormalizeBrowserID(browserId)
	if normalizedBrowser == "" || normalizedBrowser == webSearchBrowserSystem {
		return browser.OpenURL(url, "")
	}
	return browser.OpenURL(url, normalizedBrowser)
}
