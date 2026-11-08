package plugin

import (
	"context"
	"errors"
	"strings"

	"wox/util"
)

// DispatchWoxViewPreviewEvent resolves the callback within its originating result and document.
func (m *Manager) DispatchWoxViewPreviewEvent(ctx context.Context, sessionID, queryID, resultID, documentID string, event WoxViewPreviewEvent) error {
	if owner, ok := strings.CutPrefix(resultID, "plugin-preview:"); ok && m.pluginPreviews != nil {
		entry, found := m.pluginPreviews.find(sessionID, queryID, owner, documentID)
		if !found {
			return errors.New("plugin preview is no longer available")
		}
		ctx = util.WithQueryIdContext(util.WithSessionContext(ctx, sessionID), queryID)
		if callback := entry.preview.OnWoxViewEvent; callback != nil {
			callback(ctx, event)
			return nil
		}
		if proxy, ok := entry.owner.Plugin.(WoxViewPreviewEventProxy); ok {
			return proxy.DispatchWoxViewPreviewEvent(ctx, resultID, documentID, event)
		}
		return errors.New("plugin preview callback unavailable")
	}
	cache, ok := m.resultStore.findInSession(sessionID, queryID, resultID)
	if !ok || documentID == "" {
		return errors.New("WoxView preview is no longer available")
	}
	preview := cache.previewSnapshot.Load()
	if preview == nil || preview.PreviewType != WoxPreviewTypeWoxView || preview.WoxViewID != documentID {
		return errors.New("WoxView preview is no longer available")
	}
	ctx = util.WithQueryIdContext(util.WithSessionContext(ctx, sessionID), queryID)
	if callback := preview.OnWoxViewEvent; callback != nil {
		callback(ctx, event)
		return nil
	}
	if proxy, ok := cache.PluginInstance.Plugin.(WoxViewPreviewEventProxy); ok {
		return proxy.DispatchWoxViewPreviewEvent(ctx, resultID, documentID, event)
	}
	return errors.New("WoxView preview callback unavailable")
}

// InstallPluginPreview registers only a current single-plugin query's accepted document.
func (m *Manager) InstallPluginPreview(sessionID, queryID, ownerID string, preview WoxPreview) bool {
	if m.pluginPreviews == nil || m.presentation.latestSessionQueries == nil || m.presentation.sessionPluginQueries == nil {
		return false
	}
	latest, ok := m.presentation.latestSessionQueries.Load(sessionID)
	state, active := m.presentation.sessionPluginQueries.Load(sessionID)
	if !ok || latest != queryID || !active || state.QueryId != queryID || state.PluginId != ownerID || state.SelectedOwner {
		return false
	}
	return m.pluginPreviews.install(sessionID, queryID, m.GetPluginInstanceById(ownerID), preview)
}

// RetirePluginPreview makes room for a new document while preserving the queued hide.
func (m *Manager) RetirePluginPreview(sessionID, queryID, ownerID, documentID string) bool {
	return m.pluginPreviews != nil && m.pluginPreviews.retire(sessionID, queryID, ownerID, documentID)
}

// ReleasePluginPreview ignores stale releases after a new document is installed.
func (m *Manager) ReleasePluginPreview(sessionID, queryID, ownerID, documentID string) bool {
	if m.pluginPreviews == nil {
		return false
	}
	entry, found := m.pluginPreviews.find(sessionID, queryID, ownerID, documentID)
	if !found || !m.pluginPreviews.release(sessionID, queryID, ownerID, documentID) {
		return false
	}
	if proxy, ok := entry.owner.Plugin.(interface {
		ReleasePluginPreview(context.Context, string, string) error
	}); ok {
		ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), sessionID), queryID)
		_ = proxy.ReleasePluginPreview(ctx, "plugin-preview:"+ownerID, documentID)
	}
	return true
}
