package launcher

import (
	"fmt"
	"runtime"
	"time"

	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

const nativeFilePreviewAutoLoadDelay = 180 * time.Millisecond

// buildNativeFilePreview reserves the preview rectangle for a platform file handler.
func (a *App) buildNativeFilePreview(path string, autoLoad bool, palette uiPalette, width, height float32) woxwidget.Widget {
	theme := palette.componentTheme()
	if runtime.GOOS != "windows" {
		return previewview.WebViewPreviewMessage("High-fidelity Office preview is currently available on Windows only.", theme.PreviewText, theme, width, height)
	}
	active := a.nativeFilePreviewPath == path
	if a.nativeFilePreviewError != "" && active {
		return previewview.WebViewPreviewMessage(a.nativeFilePreviewError, theme.ErrorText, theme, width, height)
	}
	if !autoLoad && !active && a.nativeFilePreviewPendingPath != path && a.nativeFilePreviewManualPath != path {
		return a.buildManualNativeFilePreview(path, theme, width, height)
	}
	if !active {
		return previewview.WebViewPreviewLoading(theme, width, height)
	}
	generation := a.nativeFilePreviewGeneration
	return previewview.WebViewPreview(previewview.WebViewPreviewProps{
		Width: width, Height: height, Theme: theme,
		OnBounds: func(bounds woxui.Rect) {
			a.requestNativeFilePreviewBounds(path, generation, bounds)
		},
	})
}

// buildManualNativeFilePreview keeps large Office documents from starting a shell handler until requested.
func (a *App) buildManualNativeFilePreview(path string, theme woxcomponent.Theme, width, height float32) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: theme.QueryBackground, Child: woxwidget.Align{
		Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "Large Office files are loaded on demand.", Style: woxui.TextStyle{Size: 13}, Color: theme.PreviewText},
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "native-file-preview-load", Label: "Load full preview", Variant: woxcomponent.ButtonSecondary, Theme: theme, OnTap: func() {
				a.requestManualNativeFilePreview(path)
			}}),
		}},
	}}
}

// requestManualNativeFilePreview records an explicit large-file request before scheduling the handler.
func (a *App) requestManualNativeFilePreview(path string) {
	a.nativeFilePreviewManualPath = path
	if a.scheduleNativeFilePreview(path) && a.window != nil {
		_ = a.window.Invalidate()
	}
}

// scheduleNativeFilePreview delays shell-handler construction so rapid result changes cancel obsolete work.
func (a *App) scheduleNativeFilePreview(path string) bool {
	if path == "" || a.nativeFilePreviewPath == path || a.nativeFilePreviewPendingPath == path {
		return false
	}
	a.stopNativeFilePreviewTimers()
	a.nativeFilePreviewPendingPath = path
	a.nativeFilePreviewError = ""
	a.nativeFilePreviewGeneration++
	generation := a.nativeFilePreviewGeneration
	a.nativeFilePreviewTimer = time.AfterFunc(nativeFilePreviewAutoLoadDelay, func() {
		if err := a.runOnUI("activate native file preview", func() {
			a.activateScheduledNativeFilePreview(path, generation)
		}); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, "activate native file preview: "+err.Error())
		}
	})
	return true
}

// activateScheduledNativeFilePreview makes a delayed request current only when no later selection replaced it.
func (a *App) activateScheduledNativeFilePreview(path string, generation uint64) {
	if a.destroyed.Load() || a.nativeFilePreviewGeneration != generation || a.nativeFilePreviewPendingPath != path {
		return
	}
	a.nativeFilePreviewTimer = nil
	a.nativeFilePreviewPendingPath = ""
	a.nativeFilePreviewPath = path
	a.clearNativeFilePreviewBounds()
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

// requestNativeFilePreviewBounds coalesces paint-time geometry into one deferred native-window update.
func (a *App) requestNativeFilePreviewBounds(path string, generation uint64, bounds woxui.Rect) {
	if a.nativeFilePreviewPath != path || a.nativeFilePreviewGeneration != generation || a.nativeFilePreviewError != "" {
		return
	}
	if a.nativeFilePreviewHasReportedBounds && a.nativeFilePreviewReportedBoundsPath == path && a.nativeFilePreviewReportedBoundsGeneration == generation && a.nativeFilePreviewReportedBounds == bounds {
		return
	}
	a.nativeFilePreviewBounds = bounds
	a.nativeFilePreviewBoundsPath = path
	a.nativeFilePreviewBoundsGeneration = generation
	if a.nativeFilePreviewBoundsTimer != nil {
		return
	}
	a.nativeFilePreviewBoundsTimer = time.AfterFunc(time.Millisecond, func() {
		if err := a.runOnUI("update native file preview bounds", a.flushNativeFilePreviewBounds); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, "update native file preview bounds: "+err.Error())
		}
	})
}

// flushNativeFilePreviewBounds applies only the latest valid paint geometry outside the render callback.
func (a *App) flushNativeFilePreviewBounds() {
	a.nativeFilePreviewBoundsTimer = nil
	path := a.nativeFilePreviewBoundsPath
	generation := a.nativeFilePreviewBoundsGeneration
	bounds := a.nativeFilePreviewBounds
	if path == "" || a.nativeFilePreviewPath != path || a.nativeFilePreviewGeneration != generation || a.nativeFilePreviewError != "" || a.window == nil {
		return
	}
	if a.nativeFilePreviewHasReportedBounds && a.nativeFilePreviewReportedBoundsPath == path && a.nativeFilePreviewReportedBoundsGeneration == generation && a.nativeFilePreviewReportedBounds == bounds {
		return
	}
	if err := a.window.ShowNativeFilePreview(path, bounds, generation); err != nil {
		a.setNativeFilePreviewError(generation, err)
		return
	}
	a.nativeFilePreviewReportedBounds = bounds
	a.nativeFilePreviewReportedBoundsPath = path
	a.nativeFilePreviewReportedBoundsGeneration = generation
	a.nativeFilePreviewHasReportedBounds = true
}

// activateNativeFilePreview makes one native handler the owner of the preview rectangle.
func (a *App) activateNativeFilePreview(path string) bool {
	changed := a.nativeFilePreviewPath != path
	if changed {
		a.stopNativeFilePreviewTimers()
		a.nativeFilePreviewPath = path
		a.nativeFilePreviewPendingPath = ""
		a.nativeFilePreviewError = ""
		a.nativeFilePreviewGeneration++
		a.clearNativeFilePreviewBounds()
	}
	return changed
}

// nativeFilePreviewTargetPath returns the current or delayed handler owner.
func (a *App) nativeFilePreviewTargetPath() string {
	if a.nativeFilePreviewPath != "" {
		return a.nativeFilePreviewPath
	}
	return a.nativeFilePreviewPendingPath
}

// deactivateNativeFilePreview releases the platform child window and clears its controller state.
func (a *App) deactivateNativeFilePreview() {
	hadPreview := a.nativeFilePreviewPath != "" || a.nativeFilePreviewPendingPath != "" || a.nativeFilePreviewTimer != nil || a.nativeFilePreviewBoundsTimer != nil
	a.stopNativeFilePreviewTimers()
	a.nativeFilePreviewPath = ""
	a.nativeFilePreviewPendingPath = ""
	a.nativeFilePreviewManualPath = ""
	a.nativeFilePreviewError = ""
	a.clearNativeFilePreviewBounds()
	if !hadPreview {
		return
	}
	a.nativeFilePreviewGeneration++
	if a.window == nil {
		return
	}
	// The child window can outlive controller state while a retained preview tree is being replaced.
	generation := a.nativeFilePreviewGeneration
	if err := woxui.Call(func() { _ = a.window.HideNativeFilePreview(generation) }); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, "hide native file preview: "+err.Error())
	}
}

// stopNativeFilePreviewTimers cancels work that was scheduled by an obsolete selected result.
func (a *App) stopNativeFilePreviewTimers() {
	if a.nativeFilePreviewTimer != nil {
		a.nativeFilePreviewTimer.Stop()
		a.nativeFilePreviewTimer = nil
	}
	if a.nativeFilePreviewBoundsTimer != nil {
		a.nativeFilePreviewBoundsTimer.Stop()
		a.nativeFilePreviewBoundsTimer = nil
	}
}

// clearNativeFilePreviewBounds forgets geometry tied to a previous handler generation.
func (a *App) clearNativeFilePreviewBounds() {
	a.nativeFilePreviewBounds = woxui.Rect{}
	a.nativeFilePreviewBoundsPath = ""
	a.nativeFilePreviewBoundsGeneration = 0
	a.nativeFilePreviewReportedBounds = woxui.Rect{}
	a.nativeFilePreviewReportedBoundsPath = ""
	a.nativeFilePreviewReportedBoundsGeneration = 0
	a.nativeFilePreviewHasReportedBounds = false
}

func (a *App) setNativeFilePreviewError(generation uint64, err error) {
	if err == nil || a.nativeFilePreviewGeneration != generation {
		return
	}
	detail := a.translate("i18n:ui_file_preview_office_preview_handler_unavailable")
	if detail == "" || detail == "i18n:ui_file_preview_office_preview_handler_unavailable" {
		detail = "Install Microsoft Office, WPS, or another app that registers a Windows preview handler for this file type."
	}
	message := fmt.Sprintf("Office preview unavailable:\n%s\n\n%v", detail, err)
	if a.nativeFilePreviewError == message {
		return
	}
	a.nativeFilePreviewError = message
	a.nativeFilePreviewGeneration++
	a.stopNativeFilePreviewTimers()
	a.clearNativeFilePreviewBounds()
	if a.window != nil {
		generation := a.nativeFilePreviewGeneration
		if err := woxui.Call(func() { _ = a.window.HideNativeFilePreview(generation) }); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, "hide native file preview after error: "+err.Error())
		}
		_ = a.window.Invalidate()
	}
}
