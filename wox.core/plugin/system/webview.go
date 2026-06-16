package system

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util/browser"
)

const (
	webviewSitesSettingKey          = "sites"
	webviewDefaultAddedKey          = "defaultSiteAdded"
	webviewDefaultInstagramAddedKey = "defaultInstagramAdded"
)

type webviewSite struct {
	Keyword       string
	Url           string
	InjectCss     string
	CacheDisabled bool
	Icon          common.WoxImage
	Disabled      bool
}

type WebViewPlugin struct {
	api   plugin.API
	sites []webviewSite
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &WebViewPlugin{})
}

func (p *WebViewPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "2ac1b5cf-bf55-41f0-8c34-421c323be780",
		Name:          "i18n:plugin_webview_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_webview_plugin_description",
		Icon:          common.PluginWebviewIcon.String(),
		TriggerKeywords: []string{
			"webview",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureResultPreviewWidthRatio,
				Params: map[string]any{
					"WidthRatio": 0.0,
				},
			},
		},
		SettingDefinitions: []definition.PluginSettingDefinitionItem{
			{
				Type:               definition.PluginSettingDefinitionTypeTable,
				IsPlatformSpecific: true,
				Value: &definition.PluginSettingValueTable{
					Key:           webviewSitesSettingKey,
					Title:         "i18n:plugin_webview_sites",
					SortColumnKey: "Keyword",
					SortOrder:     definition.PluginSettingValueTableSortOrderAsc,
					MaxHeight:     500,
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "Icon",
							Label: "i18n:plugin_webview_icon",
							Type:  definition.PluginSettingValueTableColumnTypeWoxImage,
							Width: 40,
						},
						{
							Key:     "Keyword",
							Label:   "i18n:plugin_webview_keyword",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   80,
							Tooltip: "i18n:plugin_webview_keyword_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:     "Url",
							Label:   "i18n:plugin_webview_url",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Tooltip: "i18n:plugin_webview_url_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:          "InjectCss",
							Label:        "i18n:plugin_webview_inject_css",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							TextMaxLines: 12,
							HideInTable:  true,
							Tooltip:      "i18n:plugin_webview_inject_css_tooltip",
						},
						{
							Key:     "CacheDisabled",
							Label:   "i18n:plugin_webview_cache_disabled",
							Type:    definition.PluginSettingValueTableColumnTypeCheckbox,
							Width:   60,
							Tooltip: "i18n:plugin_webview_cache_tooltip",
						},
						{
							Key:   "Disabled",
							Label: "i18n:plugin_webview_disabled",
							Type:  definition.PluginSettingValueTableColumnTypeCheckbox,
							Width: 70,
						},
					},
				},
			},
		},
	}
}

func (p *WebViewPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
	p.sites = p.loadSites(ctx)
	p.registerSiteCommands(ctx)
	p.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("loaded %d webview sites", len(p.sites)))

	p.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key != webviewSitesSettingKey {
			return
		}

		p.sites = p.loadSites(callbackCtx)
		p.registerSiteCommands(callbackCtx)
	})
}

func (p *WebViewPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.Type != plugin.QueryTypeInput {
		return plugin.QueryResponse{}
	}
	if strings.TrimSpace(query.Command) == "" {
		return plugin.QueryResponse{}
	}

	var results []plugin.QueryResult
	for _, site := range p.sites {
		if site.Disabled {
			continue
		}
		if !strings.EqualFold(site.Keyword, query.Command) {
			continue
		}

		currentSite := site
		previewPayload, marshalErr := json.Marshal(plugin.WoxPreviewWebviewData{
			Url:           site.Url,
			InjectCss:     currentSite.InjectCss,
			CacheDisabled: currentSite.CacheDisabled,
		})
		if marshalErr != nil {
			p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to marshal webview payload for %s: %s", site.Url, marshalErr.Error()))
			continue
		}

		results = append(results, plugin.QueryResult{
			Title: site.Url,
			Icon:  currentSite.Icon,
			Score: 100,
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeWebView,
				PreviewData: string(previewPayload),
			},
			Actions: []plugin.QueryResultAction{
				{
					Name:      "i18n:plugin_webview_open_in_browser",
					Icon:      common.SearchIcon,
					IsDefault: true,
					Action: func(actionCtx context.Context, actionContext plugin.ActionContext) {
						if openErr := browser.OpenURL(site.Url, ""); openErr != nil {
							p.api.Log(actionCtx, plugin.LogLevelError, fmt.Sprintf("failed to open url %s: %s", site.Url, openErr.Error()))
						}
					},
				},
			},
		})
	}

	return plugin.NewQueryResponse(results)
}

func (p *WebViewPlugin) registerSiteCommands(ctx context.Context) {
	var commands []plugin.MetadataCommand
	for _, site := range p.sites {
		if site.Disabled {
			continue
		}

		commands = append(commands, plugin.MetadataCommand{
			Command:     site.Keyword,
			Description: common.I18nString(site.Url),
		})
	}

	p.api.RegisterQueryCommands(ctx, commands)
}

func (p *WebViewPlugin) loadSites(ctx context.Context) []webviewSite {
	sitesJSON := p.api.GetSetting(ctx, webviewSitesSettingKey)
	if sitesJSON == "" {
		// Add default sites on first load
		defaultAdded := p.api.GetSetting(ctx, webviewDefaultAddedKey)
		if defaultAdded == "" {
			sites := []webviewSite{
				{
					Keyword: "x",
					Url:     "https://x.com",
					Icon:    common.NewWoxImageUrl("https://abs.twimg.com/favicons/twitter.2.ico"),
				},
				{
					Keyword:   "ig",
					Url:       "https://www.instagram.com",
					InjectCss: "\ndiv[data-pagelet=\"story_tray\"] {\n    display: none !important;\n}",
					Icon:      common.NewWoxImageUrl("https://static.cdninstagram.com/rsrc.php/v4/yI/r/VsNE-OHk_8a.png"),
				},
			}
			if encoded, err := json.Marshal(sites); err == nil {
				p.api.SaveSetting(ctx, webviewSitesSettingKey, string(encoded), false)
				p.api.SaveSetting(ctx, webviewDefaultAddedKey, "true", false)
			}
			return sites
		}

		return nil
	}

	var sites []webviewSite
	if err := json.Unmarshal([]byte(sitesJSON), &sites); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to unmarshal web preview sites: %s", err.Error()))
		return nil
	}
	return sites
}
