package launcher

import (
	"context"
	"sync/atomic"

	"wox/common"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

// previewImageServices opens a preview picture in core's full-size image overlay.
type previewImageServices interface {
	ShowPreviewImage(ctx context.Context, sessionID string, image common.WoxImage) error
}

// previewController coordinates preview resources for the currently published selection
// and renders the generic preview bodies (text, Markdown, images, lists, files and folders)
// and their tags. It preserves an active WebView during typed-query grace without starting stale work.
type previewController struct {
	services                         previewImageServices
	sessionID                        string
	lifecycleCtx                     context.Context
	language                         *languageCatalog
	appearance                       *appearanceController
	tooltips                         *tooltipController
	chatWindow                       *chatWindowController
	show                             func() showAppParams
	imageFor                         func(source woxImage) *woxui.Image
	imageForSize                     func(source woxImage, size int) *woxui.Image
	imageForViewport                 func(source woxImage, size int) *woxui.Image
	releaseViewportImage             func(source woxImage, size int)
	imageErrorFor                    func(source woxImage, size int) string
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
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.language = &a.language
	c.appearance = &a.appearance
	c.tooltips = &a.tooltips
	c.chatWindow = &a.chatWindow
	c.show = func() showAppParams { return a.show }
	c.imageFor = a.imageFor
	c.imageForSize = a.imageForSize
	c.imageForViewport = a.imageForViewport
	c.releaseViewportImage = a.releaseViewportImage
	c.imageErrorFor = a.imageErrorFor
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
}
