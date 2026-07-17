package launcher

import (
	"strings"

	woxui "wox/ui/runtime"
)

// reconcileSelectedPreview keeps stateful preview resources aligned with the selected render target.
func (a *App) reconcileSelectedPreview() {
	if err := woxui.Call(a.reconcileSelectedPreviewOnUI); err == nil {
		return
	}
	// Tests and startup paths without an active native loop still need deterministic state reconciliation.
	a.reconcileSelectedPreviewOnUI()
}

// reconcileSelectedPreviewOnUI serializes resource transitions after native thread ownership is established.
func (a *App) reconcileSelectedPreviewOnUI() {
	a.previewLifecycleMu.Lock()

	result, preview, visible := a.selectedPreviewForLifecycle()
	if !visible {
		hideWebView := a.deactivatePreviewTypes("")
		a.previewLifecycleMu.Unlock()
		if hideWebView {
			a.hideWebView()
		}
		return
	}
	a.prepareRemotePreview(preview)
	preview = a.resolvePreview(preview)
	if preview.PreviewType == "file" {
		a.prepareFilePreview(preview.PreviewData)
	}

	hideWebView := a.deactivatePreviewTypes(preview.PreviewType)
	switch preview.PreviewType {
	case "query_requirement_settings":
		if a.activateRequirementPreview(result, preview) != nil {
			a.deactivateRequirementForm()
		}
	case "trigger_keyword_conflict":
		if a.activateTriggerConflictPreview(result, preview) != nil {
			a.deactivateTriggerConflictPreview()
		}
	case "theme_edit":
		if a.activateThemeEditorPreview(result, preview) != nil {
			a.deactivateThemeEditorPreview()
		}
	case "chat":
		if a.activateChatPreview(result, preview) != nil {
			a.deactivateChatPreview()
		}
	case "terminal":
		a.activateTerminalPreview(preview)
	case "webview":
		hideWebView = a.activateWebViewPreview(preview.PreviewData) || hideWebView
	}
	a.previewLifecycleMu.Unlock()
	if hideWebView {
		a.hideWebView()
	}
}

// selectedPreviewForLifecycle excludes stale query results and layouts that do not render a preview.
func (a *App) selectedPreviewForLifecycle() (queryResult, queryPreview, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.destroyed || !a.visible || a.resultsQueryID == "" || a.resultsQueryID != a.query.QueryID || a.selected < 0 || a.selected >= len(a.results) {
		return queryResult{}, queryPreview{}, false
	}
	result := a.results[a.selected]
	preview := result.Preview
	if preview.PreviewData == "" {
		return queryResult{}, queryPreview{}, false
	}
	ratio := float32(0.4)
	if a.layout.ResultPreviewWidthRatio != nil && *a.layout.ResultPreviewWidthRatio >= 0 && *a.layout.ResultPreviewWidthRatio <= 1 {
		ratio = float32(*a.layout.ResultPreviewWidthRatio)
	}
	if a.chatFullscreen {
		ratio = 0
	}
	if ratio >= 1 {
		return queryResult{}, queryPreview{}, false
	}
	return result, preview, true
}

// deactivatePreviewTypes releases every stateful preview except the selected type.
func (a *App) deactivatePreviewTypes(keep string) bool {
	if keep != "query_requirement_settings" {
		a.deactivateRequirementForm()
	}
	if keep != "trigger_keyword_conflict" {
		a.deactivateTriggerConflictPreview()
	}
	if keep != "theme_edit" {
		a.deactivateLauncherThemeEditorPreview()
	}
	if keep != "chat" {
		a.deactivateChatPreview()
	}
	if keep != "terminal" {
		a.deactivateTerminalPreview()
	}
	if keep != "webview" {
		return a.deactivateWebViewPreview()
	}
	return false
}

// deactivateLauncherThemeEditorPreview leaves the independent Settings editor untouched.
func (a *App) deactivateLauncherThemeEditorPreview() {
	a.mu.RLock()
	isSettingsEditor := a.themeEditor != nil && strings.HasPrefix(a.themeEditor.key, "settings-theme|")
	a.mu.RUnlock()
	if !isSettingsEditor {
		a.deactivateThemeEditorPreview()
	}
}
