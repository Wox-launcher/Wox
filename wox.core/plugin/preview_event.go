package plugin

import (
	"context"
	"errors"

	"wox/util"
)

// DispatchWoxViewPreviewEvent resolves the callback within its originating result and document.
func (m *Manager) DispatchWoxViewPreviewEvent(ctx context.Context, sessionID, queryID, resultID, documentID string, event WoxViewPreviewEvent) error {
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
