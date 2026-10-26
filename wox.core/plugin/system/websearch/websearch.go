package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/plugin/system/migrateapply"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/browser"
	"wox/util/selection"
	"wox/util/websiteicon"
)

// fetchEmbeddedWebsiteIcon is the settings "fetch from URL" path. Tests replace it to stay offline.
var fetchEmbeddedWebsiteIcon = websiteicon.FetchEmbedded

const (
	webSearchDefaultBrowserSettingKey = "defaultBrowser"

	webSearchBrowserSystem     = "system"  // follow the system default browser
	webSearchBrowserUseDefault = "default" // follow the default browser setting of the plugin, which is webSearchBrowserSystem or a specific browser

	// webSearchOpenWebViewActionID is intercepted by visible launchers; its callback
	// opens a separate preview window when execution originates from a global hotkey.
	webSearchOpenWebViewActionID     = "__wox_internal_open_webview_preview__"
	webSearchOpenWebViewURLKey       = "url"
	webSearchOpenWebViewInjectCSSKey = "injectCss"
	webSearchOpenWebViewWidthKey     = "width"
	webSearchOpenWebViewHeightKey    = "height"
	webSearchOpenWebViewUserAgentKey = "userAgent"
	webSearchOpenWebViewKeepKey      = "keepInBackground"
	webSearchTableGroupAdvanced      = "advanced"
	webSearchTableGroupWebView       = "webview"

	webSearchMRUKeywordKey = "keyword"
	webSearchMRUValuesKey  = "values"
)

var webSearchesSettingKey = "webSearches"

var webSearchIcon = icons.Get(icons.PluginWebsearch)

var defaultWebSearchAddedKey = "defaultWebSearchAdded"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WebSearchPlugin{})
	migrateapply.ImportQuicklinks = importMigratedQuicklinks
}

type webSearch struct {
	Urls              []string
	Title             string
	Keyword           string
	Browser           string
	OpenInWebView     bool
	OpenInPrivate     bool // open the resolved browser's private/incognito window when supported
	IsFallback        bool //if true, this search will be used when no other search is matched
	Icon              common.WoxImage
	Disabled          bool // checked to hide the search; new rows stay on
	WebViewWidth      int
	WebViewHeight     int
	InjectCSS         string
	UserAgent         string
	KeepInBackground  bool
	triggerRegistered bool
}

// UnmarshalJSON keeps leftover Enabled flags readable without rewriting stored settings.
func (s *webSearch) UnmarshalJSON(data []byte) error {
	type rawSearch webSearch
	aux := struct {
		Enabled *bool `json:"Enabled"`
		*rawSearch
	}{rawSearch: (*rawSearch)(s)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Enabled == nil {
		return nil
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	if _, exists := keys["Disabled"]; exists {
		return nil
	}
	s.Disabled = !*aux.Enabled
	return nil
}

type WebSearchPlugin struct {
	api                plugin.API
	mu                 sync.Mutex
	webSearches        []webSearch
	registeredKeywords []string
}

func (r *WebSearchPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            common.WebSearchPluginID,
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
			{
				Name: plugin.MetadataFeatureMRU,
				Params: map[string]any{
					"HashBy": "rawQuery",
				},
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
					Groups: []definition.PluginSettingValueTableGroup{
						{Key: webSearchTableGroupAdvanced, Title: "i18n:plugin_websearch_group_advanced", CollapsedByDefault: true},
						{Key: webSearchTableGroupWebView, Title: "i18n:plugin_websearch_group_webview", CollapsedByDefault: true},
					},
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
							Width: 70,
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
							HideInTable:   true,
							Type:          definition.PluginSettingValueTableColumnTypeSelect,
							Width:         100,
							SelectOptions: r.getWebSearchItemBrowserOptions(),
						},
						{
							Key:     "Disabled",
							Label:   "i18n:ui_disabled",
							Tooltip: "i18n:ui_disabled_tooltip",
							Type:    definition.PluginSettingValueTableColumnTypeCheckbox,
							Width:   60,
						},
						{
							Key:         "OpenInPrivate",
							Label:       "i18n:plugin_websearch_open_in_private",
							Tooltip:     "i18n:plugin_websearch_open_in_private_tooltip",
							HideInTable: true,
							Type:        definition.PluginSettingValueTableColumnTypeCheckbox,
							Width:       80,
							Group:       webSearchTableGroupAdvanced,
						},
						{
							Key:         "IsFallback",
							Label:       "i18n:plugin_websearch_is_fallback",
							Tooltip:     "i18n:plugin_websearch_is_fallback_tooltip",
							HideInTable: true,
							Type:        definition.PluginSettingValueTableColumnTypeCheckbox,
							Width:       60,
							Group:       webSearchTableGroupAdvanced,
						},
						{
							Key:         "OpenInWebView",
							Label:       "i18n:plugin_websearch_default_webview",
							Tooltip:     "i18n:plugin_websearch_default_webview_tooltip",
							HideInTable: true,
							Type:        definition.PluginSettingValueTableColumnTypeCheckbox,
							Group:       webSearchTableGroupWebView,
						},
						{
							Key:         "WebViewWidth",
							Label:       "i18n:plugin_websearch_webview_width",
							Tooltip:     "i18n:plugin_websearch_webview_width_tooltip",
							Type:        definition.PluginSettingValueTableColumnTypeText,
							HideInTable: true,
							EmptyAsZero: true,
							Group:       webSearchTableGroupWebView,
						},
						{
							Key:         "WebViewHeight",
							Label:       "i18n:plugin_websearch_webview_height",
							Tooltip:     "i18n:plugin_websearch_webview_height_tooltip",
							Type:        definition.PluginSettingValueTableColumnTypeText,
							HideInTable: true,
							EmptyAsZero: true,
							Group:       webSearchTableGroupWebView,
						},
						{
							Key:          "InjectCSS",
							Label:        "i18n:plugin_websearch_inject_css",
							Tooltip:      "i18n:plugin_websearch_inject_css_tooltip",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							TextMaxLines: 8,
							HideInTable:  true,
							Group:        webSearchTableGroupWebView,
						},
						{
							Key:           "UserAgent",
							Label:         "i18n:plugin_websearch_user_agent",
							Tooltip:       "i18n:plugin_websearch_user_agent_tooltip",
							Type:          definition.PluginSettingValueTableColumnTypeSelect,
							SelectOptions: getWebviewUserAgentOptions(),
							HideInTable:   true,
							Group:         webSearchTableGroupWebView,
						},
						{
							Key:         "KeepInBackground",
							Label:       "i18n:plugin_websearch_keep_in_background",
							Tooltip:     "i18n:plugin_websearch_keep_in_background_tooltip",
							Type:        definition.PluginSettingValueTableColumnTypeCheckbox,
							HideInTable: true,
							Group:       webSearchTableGroupWebView,
						},
					},
				},
			},
		},
	}
}

func (r *WebSearchPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.reloadWebSearches(ctx)
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("loaded %d web searches", len(r.webSearches)))

	r.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key == webSearchesSettingKey {
			r.reloadWebSearches(callbackCtx)
			r.indexIcons(callbackCtx)
		}
	})

	util.Go(ctx, "parse websearch icons", func() {
		r.indexIcons(ctx)
	})
	r.api.OnMRURestore(ctx, r.handleMRURestore)
}

// reloadWebSearches loads stored rows and registers their keywords before a query can see them.
// Unmarshaling clears triggerRegistered, so a keyword that core still routes would otherwise return nothing.
func (r *WebSearchPlugin) reloadWebSearches(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webSearches = r.loadWebSearches(ctx)
	r.registerTriggerKeywords(ctx)
}

// ensureTriggerRegisteredLocked repairs a keyword that core still routes after a reload cleared the flag.
// Callers hold r.mu. A keyword that failed registration stays unregistered and still produces no result.
func (r *WebSearchPlugin) ensureTriggerRegisteredLocked(ctx context.Context, keyword string) {
	if keyword == "" {
		return
	}
	matched := false
	for i := range r.webSearches {
		if r.webSearches[i].Disabled || r.webSearches[i].Keyword != keyword {
			continue
		}
		matched = true
		if r.webSearches[i].triggerRegistered {
			return
		}
	}
	if matched {
		r.registerTriggerKeywords(ctx)
	}
}

func (r *WebSearchPlugin) cloneSearches() []webSearch {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.webSearches)
}

// searchesForKeyword returns a stable copy, restoring registration when a reload cleared it.
func (r *WebSearchPlugin) searchesForKeyword(ctx context.Context, keyword string) []webSearch {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureTriggerRegisteredLocked(ctx, keyword)
	return slices.Clone(r.webSearches)
}

// registerTriggerKeywords makes enabled searches participate in core scoped routing.
// Callers that share the plugin with queries hold r.mu; the method itself does not lock.
func (r *WebSearchPlugin) registerTriggerKeywords(ctx context.Context) {
	var keywords []string
	parameterSets := make(map[string][]string)
	variableSets := make(map[string][]plugin.QueryVariable)
	debounceIntervals := make(map[string]int)
	for i := range r.webSearches {
		search := &r.webSearches[i]
		search.triggerRegistered = false
		if !search.Disabled {
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
			if search.OpenInWebView && len(names) > 0 {
				debounceIntervals[search.Keyword] = 500
			}
			debounceMs := debounceIntervals[search.Keyword]
			search.triggerRegistered = r.api.RegisterTriggerKeyword(ctx, plugin.RegisterTriggerKeywordOption{
				Keyword: search.Keyword, QueryHint: search.queryHint(names), QueryVariables: variableSets[search.Keyword],
				DebounceMs: &debounceMs,
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
	r.mu.Lock()
	pending := slices.Clone(r.webSearches)
	r.mu.Unlock()

	hasAnyIconIndexed := false
	for i, search := range pending {
		// Imported quicklinks keep the generic globe until this fetch stores the site icon.
		if search.Icon.IsEmpty() || isGenericWebSearchIcon(search.Icon) {
			pending[i].Icon = r.indexWebSearchIcon(ctx, search)
			hasAnyIconIndexed = true
		}
	}
	if !hasAnyIconIndexed {
		return
	}

	r.mu.Lock()
	wrote := false
	if len(r.webSearches) == len(pending) {
		for i := range r.webSearches {
			if !webSearchIconNeedsFetch(r.webSearches[i].Icon) || r.webSearches[i].Keyword != pending[i].Keyword || sameWebSearchIcon(r.webSearches[i].Icon, pending[i].Icon) {
				continue
			}
			r.webSearches[i].Icon = pending[i].Icon
			wrote = true
		}
	}
	var raw []byte
	var err error
	if wrote {
		raw, err = json.Marshal(r.webSearches)
	}
	r.mu.Unlock()
	if wrote && err == nil {
		r.api.SaveSetting(ctx, webSearchesSettingKey, string(raw), false)
	}
}

func (r *WebSearchPlugin) indexWebSearchIcon(ctx context.Context, search webSearch) common.WoxImage {
	if len(search.Urls) == 0 {
		return webSearchIcon
	}
	// if search url is google, return google icon
	if strings.Contains(search.Urls[0], "google.com") {
		return icons.Get(icons.BrandGoogle)
	}

	// Preserve URL order because it defines parameter order.
	urls := slices.Clone(search.Urls)
	slices.Sort(urls)

	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	img, err := fetchEmbeddedWebsiteIcon(fetchCtx, urls[0])
	if err != nil || img.IsEmpty() {
		if r.api != nil {
			r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get icon for %s: %v", search.Urls[0], err))
		}
		return webSearchIcon
	}
	return img
}

func webSearchIconNeedsFetch(icon common.WoxImage) bool {
	return icon.IsEmpty() || isGenericWebSearchIcon(icon)
}

func isGenericWebSearchIcon(icon common.WoxImage) bool {
	return sameWebSearchIcon(icon, webSearchIcon)
}

func sameWebSearchIcon(left, right common.WoxImage) bool {
	return left.ImageType == right.ImageType && left.ImageData == right.ImageData
}

func (r *WebSearchPlugin) loadWebSearches(ctx context.Context) (webSearches []webSearch) {
	webSearchesJson := r.api.GetSetting(ctx, webSearchesSettingKey)
	if webSearchesJson == "" {
		defaultAdded := r.api.GetSetting(ctx, defaultWebSearchAddedKey)
		if defaultAdded == "" {
			webSearches = []webSearch{
				{
					Urls:          []string{"https://www.google.com/search?q={wox:parameter?name=query}"},
					Title:         "Search Google for {wox:parameter?name=query}",
					Keyword:       "g",
					Browser:       webSearchBrowserSystem,
					IsFallback:    true,
					WebViewHeight: 720,
					Icon:          icons.Get(icons.BrandGoogle),
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
		return r.querySelection(ctx, query)
	}
	var results []plugin.QueryResult
	var resultSearch webSearch
	for _, search := range r.searchesForKeyword(ctx, query.TriggerKeyword) {
		if search.Disabled || !search.triggerRegistered || query.TriggerKeyword != search.Keyword {
			continue
		}
		if query.ContextData[webSearchPreviewContextKey] == "true" {
			return r.queryWebViewPreview(ctx, search, query.ContextData)
		}
		names, err := search.parameters()
		if err != nil {
			continue
		}
		values, complete := search.parameterValues(query, names)
		if !complete {
			results = append(results, plugin.QueryResult{
				Title: "i18n:plugin_websearch_fill_parameters", SubTitle: strings.Join(names, " · "), Icon: search.Icon,
				Actions: []plugin.QueryResultAction{{Name: "i18n:plugin_websearch_fill_parameters", Icon: icons.Get(icons.ActionEdit), PreventHideAfterAction: true,
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
		resultSearch = search
	}
	return webSearchResponse(results, resultSearch)
}

// QueryFallback only maps a complete raw query to an unambiguous single input.
func (r *WebSearchPlugin) QueryFallback(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	return r.singleParameterResponse(ctx, query.RawQuery, query.QueryVariables, nil).Results
}

func (r *WebSearchPlugin) querySelection(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.Selection.Type != selection.SelectionTypeText {
		return plugin.QueryResponse{}
	}
	return r.singleParameterResponse(ctx, query.Selection.Text, query.QueryVariables, &query.Selection.Text)
}

// singleParameterResponse applies the same eligibility rule to fallback and text selection.
func (r *WebSearchPlugin) singleParameterResponse(ctx context.Context, text string, environment map[string]string, selectedText *string) plugin.QueryResponse {
	var results []plugin.QueryResult
	var resultSearch webSearch
	for _, search := range r.cloneSearches() {
		if search.Disabled || !search.IsFallback {
			continue
		}
		names, err := search.parameters()
		if err != nil || len(names) != 1 {
			continue
		}
		values := map[string]string{plugin.ParameterQueryVariable(names[0]): text}
		results = append(results, r.searchResult(ctx, search, values, environment, selectedText))
		resultSearch = search
	}
	return webSearchResponse(results, resultSearch)
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
	mruContext := webSearchMRUContext(search, values)
	var preview plugin.WoxPreview
	actions := []plugin.QueryResultAction{{
		Name: "i18n:plugin_websearch_search", Icon: icons.Get(icons.ActionSearch), IsDefault: true,
		ContextData: mruContext,
		Action: func(ctx context.Context, _ plugin.ActionContext) {
			util.Go(ctx, "open web search urls", func() { r.openSearchUrls(ctx, search, values) })
		},
	}}
	if previewURL := firstWebViewSearchURL(search, values); previewURL != "" {
		previewContext := webViewPreviewContextData(search, previewURL)
		for key, value := range mruContext {
			previewContext[key] = value
		}
		actions = append(actions, plugin.QueryResultAction{
			Id: webSearchOpenWebViewActionID, Name: "i18n:plugin_websearch_open_in_webview",
			Icon: icons.Get(icons.ActionPreview), Hotkey: util.PrimaryHotkey("enter"),
			PreventHideAfterAction: true, ContextData: previewContext,
			Action: func(actionCtx context.Context, _ plugin.ActionContext) {
				// Visible launchers intercept this action; silent/result hotkeys need
				// their own window and must keep the already captured variable values.
				plugin.GetPluginManager().GetUI().OpenWoxInstance(actionCtx, webSearchPreviewRequest(search, values))
			},
		})
		if search.OpenInWebView {
			preview = webSearchAutomaticPreview(search, previewURL)
			actions[0].IsDefault = false
			actions[0].Name = "i18n:plugin_websearch_open_in_browser"
			actions[1].IsDefault = true
			actions[1].Name = "i18n:plugin_websearch_focus_webview"
			actions[1].Hotkey = "enter"
			actions[0], actions[1] = actions[1], actions[0]
		}
	}
	return plugin.QueryResult{
		Title: renderWebSearchTemplate(search.Title, values, false), IdentityKey: webSearchIdentityKey(search.Keyword, values), Score: 100, Icon: search.Icon, Actions: actions, Preview: preview,
	}
}

// webSearchIdentityKey identifies one engine plus its parameter values, independent of the translated title.
func webSearchIdentityKey(keyword string, values map[string]string) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var identity strings.Builder
	identity.WriteString(keyword)
	for _, key := range keys {
		identity.WriteString("\x1f")
		identity.WriteString(key)
		identity.WriteString("=")
		identity.WriteString(values[key])
	}
	return identity.String()
}

// webViewPreviewContextData carries the first URL plus optional per-search preview chrome.
func webViewPreviewContextData(search webSearch, previewURL string) map[string]string {
	data := map[string]string{webSearchOpenWebViewURLKey: previewURL}
	if userAgent := resolveWebviewUserAgent(search.UserAgent); userAgent != "" {
		data[webSearchOpenWebViewUserAgentKey] = userAgent
	}
	if search.KeepInBackground {
		data[webSearchOpenWebViewKeepKey] = "true"
	}
	if css := strings.TrimSpace(search.InjectCSS); css != "" {
		data[webSearchOpenWebViewInjectCSSKey] = css
	}
	if search.WebViewWidth > 0 {
		data[webSearchOpenWebViewWidthKey] = strconv.Itoa(search.WebViewWidth)
	}
	if search.WebViewHeight > 0 {
		data[webSearchOpenWebViewHeightKey] = strconv.Itoa(search.WebViewHeight)
	}
	return data
}

// firstWebViewSearchURL returns the first resolved http(s) URL, which is the
// only page the in-launcher preview can show when a search opens several tabs.
func firstWebViewSearchURL(search webSearch, values map[string]string) string {
	for _, template := range search.Urls {
		resolved := renderWebSearchTemplate(template, values, true)
		if isWebViewSearchURL(resolved) {
			return resolved
		}
	}
	return ""
}

func isWebViewSearchURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")
}

// openSearchUrls encodes substituted values for their URL component, never the template itself.
func (r *WebSearchPlugin) openSearchUrls(ctx context.Context, search webSearch, values map[string]string) {
	configuredDefaultBrowser := r.api.GetSetting(ctx, webSearchDefaultBrowserSettingKey)
	browser := r.resolveWebSearchBrowser(search.Browser, configuredDefaultBrowser)
	for _, template := range search.Urls {
		resolvedURL := renderWebSearchTemplate(template, values, true)
		if err := r.openURLInWebSearchBrowser(resolvedURL, browser, search.OpenInPrivate); err != nil {
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
		{Label: "i18n:plugin_websearch_browser_system_default", Value: webSearchBrowserSystem, Icon: icons.Get(icons.PluginBrowser)},
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
		{Label: "i18n:plugin_websearch_browser_use_default", Value: webSearchBrowserUseDefault, Icon: icons.Get(icons.PluginBrowser)},
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

func webSearchMRUContext(search webSearch, values map[string]string) common.ContextData {
	data := common.ContextData{webSearchMRUKeywordKey: search.Keyword}
	if encoded, err := json.Marshal(values); err == nil {
		data[webSearchMRUValuesKey] = string(encoded)
	}
	return data
}

// handleMRURestore rebuilds a search result from the engine keyword and stored parameter values.
func (r *WebSearchPlugin) handleMRURestore(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	keyword := strings.TrimSpace(mruData.ContextData[webSearchMRUKeywordKey])
	if keyword == "" {
		return nil, fmt.Errorf("empty web search keyword in context data")
	}
	var values map[string]string
	if raw := strings.TrimSpace(mruData.ContextData[webSearchMRUValuesKey]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return nil, fmt.Errorf("invalid web search values in context data: %w", err)
		}
	}

	var found webSearch
	matched := false
	for _, search := range r.cloneSearches() {
		if search.Keyword == keyword && !search.Disabled {
			found = search
			matched = true
			break
		}
	}
	if !matched {
		return nil, fmt.Errorf("web search no longer exists: %s", keyword)
	}
	names, err := found.parameters()
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if strings.TrimSpace(values[plugin.ParameterQueryVariable(name)]) == "" {
			return nil, fmt.Errorf("missing web search parameter: %s", name)
		}
	}
	result := r.searchResult(ctx, found, values, values, nil)
	if result.Title == "i18n:plugin_websearch_missing_context" {
		return nil, fmt.Errorf("web search is missing required context: %s", keyword)
	}
	return &result, nil
}

func (r *WebSearchPlugin) openURLInWebSearchBrowser(url string, browserId string, private bool) error {
	normalizedBrowser := browser.NormalizeBrowserID(browserId)
	if normalizedBrowser == "" || normalizedBrowser == webSearchBrowserSystem {
		normalizedBrowser = ""
	}
	if private {
		return browser.OpenURLInPrivate(url, normalizedBrowser)
	}
	return browser.OpenURL(url, normalizedBrowser)
}
