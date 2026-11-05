package plugin

import (
	"context"
	"wox/util"
)

func (m *Manager) GetGlanceItems(ctx context.Context, keys []GlanceKey, reason GlanceRefreshReason) []GlanceItemUI {
	return m.presentation.GetGlanceItems(ctx, keys, reason)
}

func (m *Manager) ExecuteGlanceAction(ctx context.Context, pluginId string, glanceId string, actionId string) error {
	return m.presentation.ExecuteGlanceAction(ctx, pluginId, glanceId, actionId)
}

func (m *Manager) ShowToolbarMsg(ctx context.Context, pluginInstance *Instance, msg ToolbarMsg) {
	m.presentation.ShowToolbarMsg(ctx, pluginInstance, msg)
}

func (m *Manager) ClearToolbarMsg(ctx context.Context, pluginInstance *Instance, toolbarMsgId string) {
	m.presentation.ClearToolbarMsg(ctx, pluginInstance, toolbarMsgId)
}

func (m *Manager) ExecuteToolbarMsgAction(ctx context.Context, sessionId string, toolbarMsgId string, actionId string) error {
	return m.presentation.ExecuteToolbarMsgAction(ctx, sessionId, toolbarMsgId, actionId)
}

func (m *Manager) HandleQueryLifecycle(ctx context.Context, query Query, pluginInstance *Instance) {
	m.presentation.HandleQueryLifecycle(ctx, query, pluginInstance)
}

// ShowSessionToolbarMsg displays an internal, session-bound message without requiring a scoped query.
func (m *Manager) ShowSessionToolbarMsg(ctx context.Context, pluginInstance *Instance, msg ToolbarMsg) {
	if pluginInstance == nil || util.GetContextSessionId(ctx) == "" {
		return
	}
	m.presentation.showSessionToolbarMsg(ctx, pluginInstance, msg, false, false)
}

// BeginSessionQuery marks older asynchronous query updates as stale immediately.
func (m *Manager) BeginSessionQuery(sessionID, queryID string) {
	if sessionID != "" && queryID != "" {
		m.presentation.latestSessionQueries.Store(sessionID, queryID)
	}
}

// SelectResultContext resolves ownership from the session's cached result rather than trusting UI data.
func (m *Manager) SelectResultContext(ctx context.Context, sessionID, queryID, resultID string) {
	pluginID := ""
	if resultID != "" {
		pluginID = m.PluginIDForCachedResult(sessionID, queryID, resultID)
	}
	m.presentation.selectResultContext(ctx, sessionID, queryID, resultID, pluginID)
}

// SelectedResultID resolves the selected result from the current plugin context.
func (m *Manager) SelectedResultID(ctx context.Context, pluginID string) string {
	sessionID, queryID := util.GetContextSessionId(ctx), util.GetContextQueryId(ctx)
	state, found := m.presentation.sessionPluginQueries.Load(sessionID)
	if !found || state.PluginId != pluginID || state.QueryId != queryID || !state.SelectedOwner {
		return ""
	}
	if latest, ok := m.presentation.latestSessionQueries.Load(sessionID); ok && latest != queryID {
		return ""
	}
	return state.ResultId
}
