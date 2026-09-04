package launcher

import (
	"log"
)

// reconcileSelectedPreview keeps stateful preview resources aligned with the selected render target.
func (a *App) reconcileSelectedPreview() {
	if err := a.runOnUI("reconcile selected preview", a.reconcileSelectedPreviewOnUI); err != nil {
		log.Printf("dispatch selected preview reconciliation: %v", err)
	}
}

// reconcileSelectedPreviewOnUI serializes resource transitions after native thread ownership is established.
func (a *App) reconcileSelectedPreviewOnUI() {
	result, preview, visible := a.selectedPreviewForLifecycle()
	if !visible {
		a.cancelScheduledFilePreview()
		hideWebView := a.deactivatePreviewTypes("")
		if hideWebView {
			a.hideWebView()
		}
		return
	}
	a.prepareRemotePreview(preview)
	preview = a.resolvePreview(preview)
	fileWebViewData := ""
	nativeFilePath := ""
	nativeFileAutoLoad := false
	if preview.PreviewType == "file" {
		a.prepareFilePreview(preview.PreviewData)
		filePreview := a.filePreviewFor(preview.PreviewData)
		if filePreview.Kind == "webview" {
			fileWebViewData = filePreview.WebViewData
		}
		if filePreview.Kind == "native_file" {
			nativeFilePath = filePreview.NativeFilePath
			nativeFileAutoLoad = filePreview.NativeFileAutoLoad
		}
	} else {
		a.cancelScheduledFilePreview()
	}

	previewType := preview.PreviewType
	if fileWebViewData != "" {
		previewType = "webview"
	}
	if nativeFilePath != "" {
		previewType = "native_file"
	}
	if nativeFilePath != "" && a.nativeFilePreviewTargetPath() != "" && a.nativeFilePreviewTargetPath() != nativeFilePath {
		// The native preview is an external child window, so replacing its controller path is not enough to remove the old HWND.
		a.deactivateNativeFilePreview()
	}
	hideWebView := a.deactivatePreviewTypes(previewType)
	resetWebView := false
	switch preview.PreviewType {
	case "query_requirement_settings":
		if a.activateRequirementPreview(result, preview) != nil {
			a.deactivateRequirementForm()
		}
	case "trigger_keyword_conflict":
		if a.activateTriggerConflictPreview(result, preview) != nil {
			a.deactivateTriggerConflictPreview()
		}
	case "chat":
		if a.activateChatPreview(result, preview) != nil {
			a.deactivateChatPreview()
		}
	case "terminal":
		a.activateTerminalPreview(preview)
	case "dictation_history":
		a.reconcileDictationAudioPreview(preview)
	case "webview":
		resetWebView = a.activateWebViewPreview(preview.PreviewData)
	}
	if fileWebViewData != "" {
		resetWebView = a.activateWebViewPreview(fileWebViewData)
	}
	if nativeFilePath != "" {
		if nativeFileAutoLoad || a.nativeFilePreviewManualPath == nativeFilePath {
			a.scheduleNativeFilePreview(nativeFilePath)
		}
	}
	if resetWebView {
		a.resetWebView()
	} else if hideWebView {
		a.hideWebView()
	}
}

// selectedPreviewForLifecycle excludes stale query results and layouts that do not render a preview.
func (a *App) selectedPreviewForLifecycle() (queryResult, queryPreview, bool) {
	if a.destroyed.Load() || !a.visible || a.resultsQueryID == "" || a.resultsQueryID != a.query.QueryID || a.selected < 0 || a.selected >= len(a.results) {
		return queryResult{}, queryPreview{}, false
	}
	result := a.results[a.selected]
	preview := result.Preview
	if !launcherPreviewVisible(a.layout, preview) {
		return queryResult{}, queryPreview{}, false
	}
	ratio := float32(0.4)
	if a.layout.ResultPreviewWidthRatio != nil && *a.layout.ResultPreviewWidthRatio >= 0 && *a.layout.ResultPreviewWidthRatio <= 1 {
		ratio = float32(*a.layout.ResultPreviewWidthRatio)
	}
	if a.chatFullscreen || a.terminalFullscreen {
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
	if keep != "chat" {
		a.deactivateChatPreview()
	}
	if keep != "terminal" {
		a.deactivateTerminalPreview()
	}
	if keep != "dictation_history" {
		a.deactivateDictationAudio()
	}
	if keep != "webview" {
		hideWebView := a.deactivateWebViewPreview()
		if keep != "native_file" {
			a.deactivateNativeFilePreview()
		}
		return hideWebView
	}
	if keep != "native_file" {
		a.deactivateNativeFilePreview()
	}
	return false
}
