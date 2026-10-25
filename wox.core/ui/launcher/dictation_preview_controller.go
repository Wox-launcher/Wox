package launcher

import (
	"context"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// dictationPreviewController owns the dictation history preview with its playback and
// offline comparison sessions. Audio completions are admitted only while the owning session is current.
type dictationPreviewController struct {
	lifecycleCtx     context.Context
	runOnUI          func(string, func()) error
	uiCall           func(func()) error
	window           func() *woxui.Window
	language         *languageCatalog
	appearance       *appearanceController
	isDev            bool
	textLayout       func(window *woxui.Window, scrollKey, value string, style woxui.TextStyle, width, lineHeight float32) woxwidget.TextBlockLayout
	dictationAudio   *dictationPreviewAudioState
	dictationCompare *dictationModelCompareState
}

// bindDictationPreview supplies UI dispatch without retaining the application.
func (a *App) bindDictationPreview() {
	a.dictation.lifecycleCtx = a.lifecycleCtx
	a.dictation.runOnUI = a.runOnUI
	a.dictation.uiCall = func(fn func()) error { return a.runOnUI("update dictation preview", fn) }
	a.dictation.window = func() *woxui.Window { return a.window }
	a.dictation.language = &a.language
	a.dictation.appearance = &a.appearance
	a.dictation.isDev = a.isDev
	a.dictation.textLayout = a.previews.previewTextLayout
}

// dispose invalidates pending audio and model-comparison completions before teardown.
func (a *dictationPreviewController) dispose() {
	a.deactivateDictationAudio()
	if a.dictationCompare != nil && a.dictationCompare.cancel != nil {
		a.dictationCompare.cancel()
	}
	a.dictationCompare = nil
}
