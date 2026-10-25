package launcher

import (
	"context"
	"strings"
	"time"
)

// filePreviewController owns file inspection, deferred reads, and manual-load overrides.
// The selected target is supplied by the coordinator only when a delayed read is ready.
type filePreviewController struct {
	lifecycleCtx              context.Context
	runOnUI                   func(string, func()) error
	isDestroyed               func() bool
	isSelected                func(string) bool
	invalidate                func()
	onReady                   func()
	filePreviews              map[string]filePreviewContent
	fileRequests              map[string]bool
	filePreviewManualPaths    map[string]bool
	filePreviewTimer          *time.Timer
	filePreviewPendingPath    string
	filePreviewLoadGeneration uint64
}

// bindFilePreview keeps query selection and window invalidation outside the file reader.
func (a *App) bindFilePreview() {
	a.filePreview.lifecycleCtx = a.lifecycleCtx
	a.filePreview.runOnUI = a.runOnUI
	a.filePreview.isDestroyed = a.isDestroyed
	a.filePreview.isSelected = func(path string) bool {
		_, preview, visible := a.previews.selectedPreviewForLifecycle()
		preview = a.remotePreview.resolvePreview(preview)
		return visible && preview.PreviewType == "file" && strings.TrimSpace(preview.PreviewData) == path
	}
	a.filePreview.invalidate = func() {
		if a.window != nil {
			_ = a.window.Invalidate()
		}
	}
	a.filePreview.onReady = func() {
		a.previews.reconcileSelectedPreviewOnUI()
		a.filePreview.invalidate()
	}
}
