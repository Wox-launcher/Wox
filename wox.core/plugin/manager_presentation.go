package plugin

import "context"

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

func (m *Manager) HasVisibleToolbarMsg(ctx context.Context) bool {
	return m.presentation.HasVisibleToolbarMsg(ctx)
}

func (m *Manager) VisibleToolbarMsgOwners() []string {
	if m == nil {
		return nil
	}
	return m.presentation.VisibleToolbarMsgOwners()
}

func (m *Manager) HandleQueryLifecycle(ctx context.Context, query Query, pluginInstance *Instance) {
	m.presentation.HandleQueryLifecycle(ctx, query, pluginInstance)
}
