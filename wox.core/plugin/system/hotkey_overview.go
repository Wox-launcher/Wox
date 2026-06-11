package system

import (
	"context"
	"encoding/json"
	"strings"
	"wox/common"
	"wox/plugin"
)

var hotkeyOverviewGlobalAliases = []string{"hotkeys", "shortcuts", "keyboard shortcuts", "keyboard shortcut", "快捷键"}
var hotkeyOverviewIcon = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect x="6" y="12" width="52" height="40" rx="8" fill="#3B82F6"/><rect x="12" y="18" width="40" height="28" rx="4" fill="#EFF6FF"/><rect x="16" y="22" width="6" height="5" rx="1.4" fill="#2563EB"/><rect x="25" y="22" width="6" height="5" rx="1.4" fill="#2563EB"/><rect x="34" y="22" width="6" height="5" rx="1.4" fill="#2563EB"/><rect x="43" y="22" width="5" height="5" rx="1.4" fill="#2563EB"/><rect x="16" y="31" width="7" height="5" rx="1.4" fill="#2563EB"/><rect x="26" y="31" width="7" height="5" rx="1.4" fill="#2563EB"/><rect x="36" y="31" width="12" height="5" rx="1.4" fill="#2563EB"/><rect x="20" y="40" width="24" height="4" rx="2" fill="#2563EB"/></svg>`)

type hotkeyOverviewPreviewData struct {
	Search string `json:"search"`
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &HotkeyOverviewPlugin{})
}

type HotkeyOverviewPlugin struct{}

// GetMetadata declares the dedicated shortcut overview plugin context.
func (p *HotkeyOverviewPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "0c8f1d9e-4f7a-4c0f-8b7f-2efb6d0b12a4",
		Name:          "i18n:plugin_hotkey_overview_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_hotkey_overview_plugin_description",
		Icon:          hotkeyOverviewIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"hotkeys",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (p *HotkeyOverviewPlugin) Init(ctx context.Context, initParams plugin.InitParams) {}

// Query returns one preview-first result and asks the launcher to give it the full preview area.
func (p *HotkeyOverviewPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.IsGlobalQuery() {
		if !p.isGlobalAlias(query.Search) {
			return plugin.QueryResponse{}
		}
		return plugin.NewQueryResponse([]plugin.QueryResult{p.buildGlobalEntryResult()})
	}

	widthRatio := 0.0
	return plugin.QueryResponse{
		Results: []plugin.QueryResult{
			{
				Title:    "i18n:plugin_hotkey_overview_title",
				SubTitle: "i18n:plugin_hotkey_overview_subtitle",
				Score:    1000,
				Icon:     hotkeyOverviewIcon,
				Preview: plugin.WoxPreview{
					PreviewType: plugin.WoxPreviewTypeHotkeyOverview,
					PreviewData: p.buildPreviewData(query.Search),
				},
				Actions: []plugin.QueryResultAction{
					{
						Name:                   "i18n:plugin_hotkey_overview_open_settings",
						Icon:                   common.WoxIcon,
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx, common.DefaultSettingWindowContext)
						},
					},
				},
			},
		},
		Layout: plugin.QueryLayout{ResultPreviewWidthRatio: &widthRatio},
	}
}

func (p *HotkeyOverviewPlugin) buildPreviewData(search string) string {
	data, err := json.Marshal(hotkeyOverviewPreviewData{Search: strings.TrimSpace(search)})
	if err != nil {
		return "{}"
	}
	return string(data)
}

// isGlobalAlias keeps the global `*` hook limited to explicit shortcut overview searches.
func (p *HotkeyOverviewPlugin) isGlobalAlias(search string) bool {
	search = strings.TrimSpace(search)
	for _, alias := range hotkeyOverviewGlobalAliases {
		if strings.EqualFold(search, alias) {
			return true
		}
	}
	return false
}

// buildGlobalEntryResult sends users into the plugin query context where full preview layout is available.
func (p *HotkeyOverviewPlugin) buildGlobalEntryResult() plugin.QueryResult {
	return plugin.QueryResult{
		Title:    "i18n:plugin_hotkey_overview_title",
		SubTitle: "i18n:plugin_hotkey_overview_subtitle",
		Score:    1000,
		Icon:     hotkeyOverviewIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_hotkey_overview_open_overview",
				Icon:                   common.ExecuteRunIcon,
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					plugin.GetPluginManager().GetUI().ChangeQuery(ctx, common.PlainQuery{
						QueryType: plugin.QueryTypeInput,
						QueryText: "hotkeys ",
					})
				},
			},
		},
	}
}
