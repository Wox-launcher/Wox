package launcher

import (
	"sync/atomic"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

// previewController coordinates preview resources for the currently published selection.
// It preserves an active WebView during typed-query grace without starting stale work.
type previewController struct {
	resultList                       *resultController
	querySession                     *queryController
	imageStore                       *imageStore
	remotePreview                    *remotePreviewController
	filePreview                      *filePreviewController
	nativeFile                       *nativeFilePreviewController
	webView                          *webViewPreviewController
	terminal                         *terminalController
	dictation                        *dictationPreviewController
	chat                             *chatController
	window                           func() *woxui.Window
	runOnUI                          func(string, func()) error
	visible                          func() bool
	isDestroyed                      func() bool
	isPreviewFullscreen              func() bool
	fontFamily                       func() string
	activateRequirementPreview       func(queryResult, queryPreview) error
	deactivateRequirementForm        func()
	activateTriggerConflictPreview   func(queryResult, queryPreview) error
	deactivateTriggerConflictPreview func()
	webViewDocumentKey               func() string
	previewTooltipRevision           atomic.Uint64
	// pinnedPreview is the full-bleed preview image currently on screen.
	pinnedPreview     viewportPreviewPin
	previewVisibility queryPreviewVisibility
	mdDocs            map[string]woxcomponent.MarkdownDocument
	previewLayouts    map[string]*textLayoutCache
}

// bindPreviews supplies selected-result state and each preview owner's lifecycle API.
func (a *App) bindPreviews() {
	c := &a.previews
	c.resultList = &a.resultList
	c.querySession = &a.querySession
	c.imageStore = &a.imageStore
	c.remotePreview = &a.remotePreview
	c.filePreview = &a.filePreview
	c.nativeFile = &a.nativeFile
	c.webView = &a.webView
	c.terminal = &a.terminal
	c.dictation = &a.dictation
	c.chat = &a.chat
	c.window = func() *woxui.Window { return a.window }
	c.runOnUI = a.runOnUI
	c.visible = func() bool { return a.visible }
	c.isDestroyed = a.isDestroyed
	c.isPreviewFullscreen = a.isPreviewFullscreen
	c.fontFamily = func() string {
		if a.generalSettings != nil {
			return a.generalSettings.Data().AppFontFamily
		}
		return ""
	}
	c.activateRequirementPreview = a.requirements.activateRequirementPreview
	c.deactivateRequirementForm = a.requirements.deactivateRequirementForm
	c.activateTriggerConflictPreview = a.conflicts.activateTriggerConflictPreview
	c.deactivateTriggerConflictPreview = a.conflicts.deactivateTriggerConflictPreview
	c.webViewDocumentKey = a.webViewDocumentKey
}
