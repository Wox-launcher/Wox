package launcher

// queryPreviewVisibility overrides result defaults only after manual input, until the next query.
type queryPreviewVisibility struct {
	queryID string
	visible *bool
}

// reconcilePreviewVisibility preserves manual input across selection changes and streamed results.
func (a *previewController) reconcilePreviewVisibility() {
	if a.previewVisibility.queryID != a.querySession.query.QueryID {
		a.previewVisibility = queryPreviewVisibility{queryID: a.querySession.query.QueryID}
	}
}

// selectedPreviewLayout applies transient visibility without changing the plugin's layout or preview payload.
func (a *previewController) selectedPreviewLayout() queryLayout {
	layout := a.resultList.layout
	if a.isPreviewFullscreen() {
		ratio := 0.0
		layout.ResultPreviewWidthRatio = &ratio
		return layout
	}
	// Preview-only windows have no result row from which the user could reopen the content.
	if a.resultList.selected < 0 || a.resultList.selected >= len(a.resultList.results) || launcherPreviewRatio(layout, false) == 0 {
		return layout
	}
	result := a.resultList.results[a.resultList.selected]
	visible := !result.Preview.DefaultHidden && launcherPreviewRatio(layout, false) < 1
	state := a.previewVisibility
	if state.queryID == a.resultList.resultsQueryID && state.visible != nil {
		visible = *state.visible
	}
	if !visible {
		ratio := 1.0
		layout.ResultPreviewWidthRatio = &ratio
	} else if launcherPreviewRatio(layout, false) >= 1 {
		layout.ResultPreviewWidthRatio = nil
	}
	return layout
}

// toggleSelectedPreview shares one transition between the row affordance and the platform shortcut.
func (a *App) toggleSelectedPreview() bool {
	if a.resultList.selected < 0 || a.resultList.selected >= len(a.resultList.results) || a.resultList.resultsQueryID != a.querySession.query.QueryID {
		return false
	}
	preview := a.resultList.results[a.resultList.selected].Preview
	// Grid and preview-only surfaces retain their existing navigation contracts.
	if preview.PreviewData == "" || a.resultList.layout.GridLayout != nil || launcherPreviewRatio(a.resultList.layout, false) == 0 || a.isPreviewFullscreen() {
		return false
	}
	a.previews.reconcilePreviewVisibility()
	visible := !launcherPreviewVisible(a.previews.selectedPreviewLayout(), preview)
	a.previews.previewVisibility.visible = &visible
	a.dismissLauncherHoverTooltipsOnUI()
	a.previews.reconcileSelectedPreview()
	a.restoreQueryFocusAfterShow()
	a.restoreQueryTextInput()
	if a.window != nil {
		_ = a.applyWindowBounds()
		_ = a.window.Invalidate()
	}
	return true
}
