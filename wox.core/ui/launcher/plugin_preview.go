package launcher

// clearPluginPreviewContext queues the WoxView hide before retiring its callback.
func (a *previewController) clearPluginPreviewContext() {
	previous, hadPrevious := a.pluginPreview.result()
	a.clearWoxViewPreview()
	if hadPrevious && previous.Preview.WoxViewID != "" {
		if service, ok := a.services.(interface {
			RetirePluginPreview(string, string, string, string) bool
		}); ok {
			service.RetirePluginPreview(a.sessionID, previous.QueryID, a.pluginPreview.pluginID, previous.Preview.WoxViewID)
		}
		a.enqueueWoxViewLifecycle(previous, previous.Preview.WoxViewID, "release-plugin-preview", nil)
	}
	a.pluginPreview.clear()
}

// pluginPreviewState owns the preview independently of selected rows and query generations.
type pluginPreviewState struct {
	pluginID string
	preview  *queryPreview
	queryID  string
}

// accept installs the first preview in a plugin context and ignores later query snapshots.
func (s *pluginPreviewState) accept(context queryContext, preview *queryPreview, queryID string) {
	if context.IsGlobalQuery || context.PluginID == "" {
		s.clear()
		return
	}
	if s.pluginID != context.PluginID {
		s.clear()
		s.pluginID = context.PluginID
	}
	if s.preview == nil && preview != nil {
		copy := *preview
		s.preview = &copy
		s.queryID = queryID
	}
}

func (s *pluginPreviewState) clear() {
	s.pluginID = ""
	s.preview = nil
	s.queryID = ""
}

// result provides a stable rendering identity independent of the selected result.
func (s *pluginPreviewState) result() (queryResult, bool) {
	if s.preview == nil {
		return queryResult{}, false
	}
	return queryResult{QueryID: s.queryID, ID: "plugin-preview:" + s.pluginID, Preview: *s.preview}, true
}

// update accepts only the owner and current document so delayed events cannot update a new context.
func (s *pluginPreviewState) updateWoxView(pluginID, documentID string, preview queryPreview) bool {
	if s.preview == nil || s.pluginID != pluginID || s.preview.PreviewType != "woxview" || preview.PreviewType != "woxview" || s.preview.WoxViewID != documentID || documentID == "" || preview.WoxViewID != documentID {
		return false
	}
	s.preview = &preview
	return true
}
