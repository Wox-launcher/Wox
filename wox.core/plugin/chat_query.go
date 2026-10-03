package plugin

import (
	"context"
	"wox/common"
)

// BuildAIChatQuery resolves the current trigger when an action runs, so renamed
// keywords also work for selection, attachments, history, fallback, and dictation.
func (m *Manager) BuildAIChatQuery(ctx context.Context, text string, contextData common.ContextData) common.PlainQuery {
	query := common.PlainQuery{
		QueryType:   QueryTypeInput,
		QueryText:   text,
		ContextData: contextData,
	}
	if instance := m.GetAIChatPluginInstance(ctx); instance != nil {
		if keyword := instance.PrimaryTriggerKeyword(); keyword != "" {
			query.QueryText = keyword + " " + text
			return query
		}
	}

	// Without a usable keyword, keep the request targeted at AI Chat instead of
	// letting its text become a global query or route to another plugin.
	query.QueryScope = common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: common.AIChatPluginID}}}
	return query
}
