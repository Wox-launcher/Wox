package launcher

import (
	"context"

	"wox/plugin"
)

type remotePreviewServices interface {
	ResultPreview(context.Context, string, string, string, string) (plugin.WoxPreview, error)
}

// remotePreviewController owns deferred preview resolution and its bounded cache.
type remotePreviewController struct {
	isDestroyed     func() bool
	services        remotePreviewServices
	sessionID       string
	lifecycleCtx    context.Context
	runOnUI         func(string, func()) error
	onReady         func()
	remotePreviews  map[string]queryPreview
	previewRequests map[string]bool
}

// bindRemotePreview connects resolved content to the selected-preview lifecycle.
func (a *App) bindRemotePreview() {
	a.remotePreview.isDestroyed = a.isDestroyed
	a.remotePreview.services = a.services
	a.remotePreview.sessionID = a.sessionID
	a.remotePreview.lifecycleCtx = a.lifecycleCtx
	a.remotePreview.runOnUI = a.runOnUI
	a.remotePreview.onReady = func() {
		a.previews.reconcileSelectedPreview()
		if a.window != nil {
			_ = a.window.Invalidate()
		}
	}
}
