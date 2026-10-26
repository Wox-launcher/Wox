package launcher

import (
	"log"
)

// reconcileSelectedPreview keeps stateful preview resources aligned with the selected render target.
func (a *previewController) reconcileSelectedPreview() {
	if err := a.runOnUI("reconcile selected preview", a.reconcileSelectedPreviewOnUI); err != nil {
		log.Printf("dispatch selected preview reconciliation: %v", err)
	}
}

// reconcileSelectedPreviewOnUI serializes resource transitions after native thread ownership is established.
func (a *previewController) reconcileSelectedPreviewOnUI() {
	a.reconcilePreviewVisibility()
	result, preview, visible := a.selectedPreviewForLifecycle()
	if a.window() != nil {
		key, html := a.inlineWebViewScriptTarget(preview, visible)
		a.window().RetainWebViewScriptTarget(key, html)
	}

	if !visible {
		a.releasePinnedPreviewImage()
		a.filePreview.cancelScheduledFilePreview()
		hideWebView := a.deactivatePreviewTypes("")
		if hideWebView {
			a.webView.hideWebView()
		}
		return
	}
	if a.resultList.resultsQueryID != a.querySession.query.QueryID {
		// The old WebView is still being painted with the retained results.
		// Keep its surface and overlay without reactivating obsolete preview work.
		return
	}
	a.remotePreview.prepareRemotePreview(preview)
	preview = a.remotePreview.resolvePreview(preview)
	// Release the old image even when the next surface bypasses buildPreviewBody.
	var imageSource woxImage
	if preview.PreviewType == "image" {
		imageSource, _ = parsePreviewImage(preview.PreviewData)
	}
	fileWebViewData := ""
	nativeFilePath := ""
	nativeFileAutoLoad := false
	if preview.PreviewType == "file" {
		a.filePreview.prepareFilePreview(preview.PreviewData)
		filePreview := a.filePreview.filePreviewFor(preview.PreviewData)
		if filePreview.Kind == "image" {
			imageSource = filePreview.Image
		}
		if filePreview.Kind == "webview" {
			fileWebViewData = filePreview.WebViewData
		}
		if filePreview.Kind == "native_file" {
			nativeFilePath = filePreview.NativeFilePath
			nativeFileAutoLoad = filePreview.NativeFileAutoLoad
		}
	} else {
		a.filePreview.cancelScheduledFilePreview()
	}

	if imageSource != a.pinnedPreview.source {
		a.releasePinnedPreviewImage()
	}

	previewType := preview.PreviewType
	if fileWebViewData != "" {
		previewType = "webview"
	}
	if nativeFilePath != "" {
		previewType = "native_file"
	}
	if nativeFilePath != "" && a.nativeFile.nativeFilePreviewTargetPath() != "" && a.nativeFile.nativeFilePreviewTargetPath() != nativeFilePath {
		// The native preview is an external child window, so replacing its controller path is not enough to remove the old HWND.
		a.nativeFile.deactivateNativeFilePreview()
	}
	hideWebView := a.deactivatePreviewTypes(previewType)
	webViewURLChanged := false
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
		if a.chat.activateChatPreview(result, preview) != nil {
			a.chat.deactivateChatPreview()
		}
	case "terminal":
		a.terminal.activateTerminalPreview(preview)
	case "dictation_history":
		a.dictation.reconcileDictationAudioPreview(preview)
	case "webview":
		webViewURLChanged = a.webView.activateWebViewPreview(preview.PreviewData)
	}
	if fileWebViewData != "" {
		webViewURLChanged = a.webView.activateWebViewPreview(fileWebViewData)
	}
	if nativeFilePath != "" {
		if nativeFileAutoLoad || a.nativeFile.nativeFilePreviewManualPath == nativeFilePath {
			a.nativeFile.scheduleNativeFilePreview(nativeFilePath)
		}
	}
	if webViewURLChanged || hideWebView {
		// Detach before switching URLs: native Hide releases ephemeral pages but
		// preserves sessions explicitly configured to run in the background.
		a.webView.hideWebView()
	}
}

// selectedPreviewForLifecycle retains an already active WebView during the result grace period.
func (a *previewController) selectedPreviewForLifecycle() (queryResult, queryPreview, bool) {
	if a.isDestroyed() || !a.visible() || a.resultList.resultsQueryID == "" || a.resultList.selected < 0 || a.resultList.selected >= len(a.resultList.results) {
		return queryResult{}, queryPreview{}, false
	}
	result := a.resultList.results[a.resultList.selected]
	preview := result.Preview
	if !launcherPreviewVisible(a.selectedPreviewLayout(), preview) {
		return queryResult{}, queryPreview{}, false
	}
	if a.resultList.resultsQueryID != a.querySession.query.QueryID {
		if a.querySession.queryTransitionTimer == nil || a.webView.webViewPreviewData == "" || a.webView.webViewPreviewError != "" {
			return queryResult{}, queryPreview{}, false
		}
		retained := a.remotePreview.resolvePreview(preview)
		if retained.PreviewType == "file" {
			file := a.filePreview.filePreviewFor(retained.PreviewData)
			retained = queryPreview{PreviewType: file.Kind, PreviewData: file.WebViewData}
		}
		if retained.PreviewType != "webview" || retained.PreviewData != a.webView.webViewPreviewData {
			return queryResult{}, queryPreview{}, false
		}
	}
	return result, preview, true
}

// deactivatePreviewTypes releases every stateful preview except the selected type.
func (a *previewController) deactivatePreviewTypes(keep string) bool {
	if keep != "query_requirement_settings" {
		a.deactivateRequirementForm()
	}
	if keep != "trigger_keyword_conflict" {
		a.deactivateTriggerConflictPreview()
	}
	if !a.keepChatPreview(keep) {
		a.chat.deactivateChatPreview()
	}
	if keep != "terminal" {
		a.terminal.deactivateTerminalPreview()
	}
	if keep != "dictation_history" {
		a.dictation.deactivateDictationAudio()
	}
	if keep != "webview" {
		hideWebView := a.webView.deactivateWebViewPreview()
		if keep != "native_file" {
			a.nativeFile.deactivateNativeFilePreview()
		}
		return hideWebView
	}
	if keep != "native_file" {
		a.nativeFile.deactivateNativeFilePreview()
	}
	return false
}

// keepChatPreview retains the chat surface when the selected preview is still an
// unresolved remote wrapper. Chat JSON is often large enough to be deferred, and
// tearing the surface down would drop fullscreen before the payload loads.
func (a *previewController) keepChatPreview(keep string) bool {
	if keep == "chat" {
		return true
	}
	return keep == "remote" && (a.resultList.layout.ChatMode || a.chat.chatFullscreen)
}

// inlineWebViewScriptTarget resolves core's remote preview envelope before checking
// document identity. Treating that envelope as a different page reloads the initial
// HTML on every ordinary result update and discards accepted scripts.
func (a *previewController) inlineWebViewScriptTarget(preview queryPreview, visible bool) (string, string) {
	if !visible || a.resultList.resultsQueryID != a.querySession.query.QueryID {
		return "", ""
	}
	preview = a.remotePreview.resolvePreview(preview)
	if preview.PreviewType == "webview" {
		if data, err := decodeWebViewPreview(preview.PreviewData); err == nil && data.URL == "" && data.HTML != "" {
			return a.webView.webViewDocumentKey(), data.HTML
		}
	}
	return "", ""
}

// hasInlinePreviewToRestore asks plugins for a fresh snapshot after reopening.
// Script updates deliberately do not replace the cached HTML, and the task may
// have finished while hidden, so no later heartbeat is guaranteed to repair it.
func (a *previewController) hasInlinePreviewToRestore() bool {
	if a.resultList.selected < 0 || a.resultList.selected >= len(a.resultList.results) {
		return false
	}
	key, _ := a.inlineWebViewScriptTarget(a.resultList.results[a.resultList.selected].Preview, true)
	return key != ""
}
