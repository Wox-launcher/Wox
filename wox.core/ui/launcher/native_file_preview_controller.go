package launcher

import (
	"context"
	"time"

	woxui "wox/ui/runtime"
)

// nativeFilePreviewController owns shell-handler generations and deferred geometry updates.
type nativeFilePreviewController struct {
	lifecycleCtx context.Context
	runOnUI      func(string, func()) error
	isDestroyed  func() bool
	window       func() *woxui.Window
	translate    func(string) string
	// Native Office preview state separates selected, delayed, and reported handler generations.
	nativeFilePreviewPath                     string
	nativeFilePreviewPendingPath              string
	nativeFilePreviewManualPath               string
	nativeFilePreviewError                    string
	nativeFilePreviewGeneration               uint64
	nativeFilePreviewTimer                    *time.Timer
	nativeFilePreviewBoundsTimer              *time.Timer
	nativeFilePreviewBounds                   woxui.Rect
	nativeFilePreviewBoundsPath               string
	nativeFilePreviewBoundsGeneration         uint64
	nativeFilePreviewReportedBounds           woxui.Rect
	nativeFilePreviewReportedBoundsPath       string
	nativeFilePreviewReportedBoundsGeneration uint64
	nativeFilePreviewHasReportedBounds        bool
	nativeFilePreviewOcclusionTimer           *time.Timer
	nativeFilePreviewOcclusion                woxui.Rect
	nativeFilePreviewReportedOcclusion        woxui.Rect
}

// bindNativeFilePreview supplies the window capability without exposing query or result state.
func (a *App) bindNativeFilePreview() {
	a.nativeFile.lifecycleCtx = a.lifecycleCtx
	a.nativeFile.runOnUI = a.runOnUI
	a.nativeFile.isDestroyed = a.isDestroyed
	a.nativeFile.window = func() *woxui.Window { return a.window }
	a.nativeFile.translate = a.language.translate
}
