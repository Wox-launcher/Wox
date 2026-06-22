package system

import (
	"context"
	"fmt"
	"strings"
	"wox/common"
	"wox/database"
	"wox/plugin"
	"wox/util"
)

var attentionIcon = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#4f7cff" d="M4 5.5A2.5 2.5 0 0 1 6.5 3h11A2.5 2.5 0 0 1 20 5.5v13A2.5 2.5 0 0 1 17.5 21h-11A2.5 2.5 0 0 1 4 18.5z"/><path fill="#fff" d="M6.4 13.2h3.1c.5 0 .9.3 1.1.7l.5 1c.2.4.6.7 1.1.7h1.6c.5 0 .9-.3 1.1-.7l.5-1c.2-.4.6-.7 1.1-.7h3.1v5.3c0 .7-.6 1.3-1.3 1.3H7.7c-.7 0-1.3-.6-1.3-1.3z" opacity=".95"/><path fill="#dbe6ff" d="M7.8 6.2h8.4a.8.8 0 0 1 0 1.6H7.8a.8.8 0 1 1 0-1.6m0 3.2h8.4a.8.8 0 0 1 0 1.6H7.8a.8.8 0 1 1 0-1.6"/></svg>`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &AttentionPlugin{})
}

type AttentionPlugin struct {
	api     plugin.API
	manager *plugin.AttentionManager
}

func (a *AttentionPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "3644c342-9033-44b7-8db6-246088681917",
		Name:          "i18n:plugin_attention_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_attention_plugin_description",
		Icon:          attentionIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"attention",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (a *AttentionPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	a.api = initParams.API
}

func (a *AttentionPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	items, err := a.getManager().List(ctx)
	if err != nil {
		return plugin.QueryResponse{Results: []plugin.QueryResult{
			{
				Title:    "i18n:plugin_attention_load_failed",
				SubTitle: err.Error(),
				Icon:     attentionIcon,
				Score:    100,
			},
		}}
	}

	search := strings.TrimSpace(query.Search)
	results := []plugin.QueryResult{}
	results = append(results, a.buildItemResults(ctx, items.Unread, search, "i18n:plugin_attention_group_unread", 1000)...)
	results = append(results, a.buildItemResults(ctx, items.Read, search, "i18n:plugin_attention_group_read", 500)...)

	if len(results) == 0 {
		results = append(results, plugin.QueryResult{
			Title:      "i18n:plugin_attention_no_items",
			Icon:       attentionIcon,
			Score:      1,
			Group:      "i18n:plugin_attention_plugin_name",
			GroupScore: 1,
		})
	}

	return plugin.QueryResponse{Results: results}
}

// buildItemResults maps stored attention items to query results while preserving group ordering.
func (a *AttentionPlugin) buildItemResults(ctx context.Context, items []database.AttentionItem, search string, group string, groupScore int64) []plugin.QueryResult {
	results := []plugin.QueryResult{}
	for index, item := range items {
		match, matchScore := matchAttentionItem(ctx, item, search)
		if !match {
			continue
		}

		itemCopy := item
		score := int64(len(items) - index)
		if matchScore > score {
			score = matchScore
		}

		results = append(results, plugin.QueryResult{
			Id:         item.IdentityKey,
			Title:      item.Title,
			SubTitle:   item.Description,
			Icon:       plugin.ParseAttentionIcon(item.Icon),
			Score:      score,
			Group:      group,
			GroupScore: groupScore,
			Tails:      a.buildItemTails(ctx, item),
			Actions:    a.buildItemActions(ctx, itemCopy),
		})
	}
	return results
}

func (a *AttentionPlugin) buildItemTails(ctx context.Context, item database.AttentionItem) []plugin.QueryResultTail {
	tails := []plugin.QueryResultTail{}
	if pluginName := getAttentionPluginName(ctx, item.PluginID); pluginName != "" {
		tails = append(tails, plugin.NewQueryResultTailText(pluginName))
	}
	if item.IsRead && item.ReadTimestamp > 0 {
		tails = append(tails, plugin.NewQueryResultTailText(util.FormatTimestamp(item.ReadTimestamp)))
	} else if item.UpdatedTimestamp > 0 {
		tails = append(tails, plugin.NewQueryResultTailText(util.FormatTimestamp(item.UpdatedTimestamp)))
	}
	return tails
}

func (a *AttentionPlugin) buildItemActions(ctx context.Context, item database.AttentionItem) []plugin.QueryResultAction {
	actions := []plugin.QueryResultAction{}
	storedAction, actionErr := plugin.ParseAttentionAction(item.Action)
	if actionErr != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to parse attention action: %v", actionErr))
	}
	if storedAction != nil && storedAction.Type == plugin.AttentionActionTypeChangeQuery {
		query := storedAction.Query
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_attention_action_open",
			Icon:                   common.ExecuteRunIcon,
			IsDefault:              true,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				a.markReadAndPublish(ctx, item.IdentityKey)
				a.api.ChangeQuery(ctx, common.PlainQuery{
					QueryType: plugin.QueryTypeInput,
					QueryText: query,
				})
			},
		})
	}

	if !item.IsRead {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_attention_action_mark_read",
			Icon:                   common.PluginInstalledIcon,
			IsDefault:              len(actions) == 0,
			PreventHideAfterAction: true,
			Hotkey:                 util.PrimaryHotkey("enter"),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				a.markReadAndPublish(ctx, item.IdentityKey)
				a.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			},
		})
	}

	return actions
}

func (a *AttentionPlugin) markReadAndPublish(ctx context.Context, identityKey string) {
	if err := a.getManager().MarkRead(ctx, identityKey); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to mark attention item read: %v", err))
		return
	}
	if a.manager != nil {
		return
	}
	plugin.PublishAttentionUnreadCount(ctx)
}

func (a *AttentionPlugin) getManager() *plugin.AttentionManager {
	if a.manager != nil {
		return a.manager
	}
	return plugin.GetAttentionManager()
}

func matchAttentionItem(ctx context.Context, item database.AttentionItem, search string) (bool, int64) {
	if search == "" {
		return true, 0
	}

	titleMatch, titleScore := plugin.IsStringMatchScore(ctx, item.Title, search)
	descriptionMatch, descriptionScore := plugin.IsStringMatchScore(ctx, item.Description, search)
	if titleMatch || descriptionMatch {
		return true, max(titleScore, descriptionScore)
	}

	pluginName := getAttentionPluginName(ctx, item.PluginID)
	pluginMatch, pluginScore := plugin.IsStringMatchScore(ctx, pluginName, search)
	return pluginMatch, pluginScore
}

func getAttentionPluginName(ctx context.Context, pluginID string) string {
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance.Metadata.Id == pluginID {
			return instance.GetName(ctx)
		}
	}
	return pluginID
}
