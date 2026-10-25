package launcher

import "context"

// bindControllers is the composition root for per-launcher feature capabilities.
// Controllers share explicit collaborators; none receives the application itself.
func (app *App) bindControllers() {
	if app.lifecycleCtx == nil {
		app.lifecycleCtx = context.Background()
	}
	app.bindImageStore()
	app.bindTooltips()
	app.bindRemotePreview()
	app.bindFilePreview()
	app.bindNativeFilePreview()
	app.bindWebViewPreview()
	app.bindTerminal()
	app.bindDictationPreview()
	app.bindActions()
	app.bindQuery()
	app.bindChat()
	app.bindSettingsWindow()
	app.bindOnboarding()
	app.bindChatWindow()
	app.bindGlance()
	app.bindToolbar()
	app.bindForms()
	app.bindFormTables()
	app.bindGeneralSettings()
	app.bindHotkeyRecording()
	app.bindWindowGroupEditor()
	app.bindPluginSettings()
	app.bindCloudSettings()
	app.bindThemeSettings()
	app.bindSettingsSearch()
	app.bindDataSettings()
	app.bindRuntimeSettings()
	app.bindAISettings()
	app.bindHotkeySettings()
	app.bindNotes()
	app.bindPreviews()
	app.bindRequirements()
	app.bindConflicts()
	app.bindQuickSelect()
	app.bindAppearance()
	app.bindResultDrag()
	app.bindAttention()
}

// disposeControllers runs on the UI thread at final instance destruction, never
// on launcher hide: a dedicated chat window may still own the shared session.
func (a *App) disposeControllers() {
	a.querySession.resetQueryLoadingLocked()
	a.querySession.resetQueryTransitionLocked()
	a.querySession.releaseRefreshResultHoldLocked()
	a.quickSelect.stopQuickSelectLocked()
	a.glance.stopGlanceLocked(true)
	a.filePreview.cancelScheduledFilePreview()
	clear(a.filePreview.fileRequests)
	clear(a.remotePreview.previewRequests)
	a.imageStore.dispose()
	a.nativeFile.dispose()
	a.webView.dispose()
	a.settingsWindow.clearFlashLocked()
	a.notes.dispose()
	a.tooltips.disposeOnUI()
	a.dictation.dispose()
	a.chat.cancelChatAttachmentImports()
	a.toolbar.toolbarRevision++
	a.previews.releasePinnedPreviewImage()
}
