package launcher

// queryPreviewVisibility overrides result defaults only after manual input, until the next query.
type queryPreviewVisibility struct {
	queryID string
	visible *bool
}

// reconcilePreviewVisibility preserves manual input across selection changes and streamed results.
func (a *App) reconcilePreviewVisibility() {
	if a.previewVisibility.queryID != a.query.QueryID {
		a.previewVisibility = queryPreviewVisibility{queryID: a.query.QueryID}
	}
}

// selectedPreviewLayout applies transient visibility without changing the plugin's layout or preview payload.
func (a *App) selectedPreviewLayout() queryLayout {
	layout := a.layout
	if a.isPreviewFullscreen() {
		ratio := 0.0
		layout.ResultPreviewWidthRatio = &ratio
		return layout
	}
	// Preview-only windows have no result row from which the user could reopen the content.
	if a.selected < 0 || a.selected >= len(a.results) || launcherPreviewRatio(layout, false) == 0 {
		return layout
	}
	result := a.results[a.selected]
	visible := !result.Preview.DefaultHidden && launcherPreviewRatio(layout, false) < 1
	state := a.previewVisibility
	if state.queryID == a.resultsQueryID && state.visible != nil {
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
	if a.selected < 0 || a.selected >= len(a.results) || a.resultsQueryID != a.query.QueryID {
		return false
	}
	preview := a.results[a.selected].Preview
	// Grid and preview-only surfaces retain their existing navigation contracts.
	if preview.PreviewData == "" || a.layout.GridLayout != nil || launcherPreviewRatio(a.layout, false) == 0 || a.isPreviewFullscreen() {
		return false
	}
	a.reconcilePreviewVisibility()
	visible := !launcherPreviewVisible(a.selectedPreviewLayout(), preview)
	a.previewVisibility.visible = &visible
	a.dismissLauncherHoverTooltipsOnUI()
	a.reconcileSelectedPreview()
	a.restoreQueryFocusAfterShow()
	a.restoreQueryTextInput()
	if a.window != nil {
		_ = a.applyWindowBounds()
		_ = a.window.Invalidate()
	}
	return true
}
