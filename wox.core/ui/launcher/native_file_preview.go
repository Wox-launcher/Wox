package launcher

import (
	"fmt"
	"runtime"

	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// buildNativeFilePreview reserves the preview rectangle for a platform file handler.
func (a *App) buildNativeFilePreview(path string, palette uiPalette, width, height float32) woxwidget.Widget {
	theme := palette.componentTheme()
	if runtime.GOOS != "windows" {
		return previewview.WebViewPreviewMessage("High-fidelity Office preview is currently available on Windows only.", theme.PreviewText, theme, width, height)
	}
	active := a.nativeFilePreviewPath == path
	if !active {
		return previewview.WebViewPreviewMessage("Loading native file preview…", theme.PreviewText, theme, width, height)
	}
	if a.nativeFilePreviewError != "" {
		return previewview.WebViewPreviewMessage(a.nativeFilePreviewError, theme.ErrorText, theme, width, height)
	}
	return previewview.WebViewPreview(previewview.WebViewPreviewProps{
		Width: width, Height: height, Theme: theme,
		OnBounds: func(bounds woxui.Rect) {
			if a.nativeFilePreviewPath != path || a.nativeFilePreviewError != "" {
				return
			}
			if err := a.window.ShowNativeFilePreview(path, bounds); err != nil {
				a.setNativeFilePreviewError(err)
			}
		},
	})
}

// activateNativeFilePreview makes one native handler the owner of the preview rectangle.
func (a *App) activateNativeFilePreview(path string) bool {
	changed := a.nativeFilePreviewPath != path
	if changed {
		a.nativeFilePreviewPath = path
		a.nativeFilePreviewError = ""
	}
	return changed
}

// deactivateNativeFilePreview releases the platform child window and clears its controller state.
func (a *App) deactivateNativeFilePreview() {
	a.closeNativePreviewCloseOverlay()
	wasActive := a.nativeFilePreviewPath != "" || a.nativeFilePreviewError != ""
	a.nativeFilePreviewPath = ""
	a.nativeFilePreviewError = ""
	if !wasActive || a.window == nil {
		return
	}
	if err := woxui.Call(func() { _ = a.window.HideNativeFilePreview() }); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, "hide native file preview: "+err.Error())
	}
}

func (a *App) setNativeFilePreviewError(err error) {
	if err == nil {
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
	a.closeNativePreviewCloseOverlay()
	if a.window != nil {
		if err := woxui.Call(func() { _ = a.window.HideNativeFilePreview() }); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, "hide native file preview after error: "+err.Error())
		}
	}
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}
