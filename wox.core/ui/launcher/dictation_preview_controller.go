package launcher

import (
	"context"

	woxui "wox/ui/runtime"
)

// dictationPreviewController owns playback and offline comparison sessions.
// Audio completions are admitted only while the owning session is current.
type dictationPreviewController struct {
	lifecycleCtx     context.Context
	runOnUI          func(string, func()) error
	uiCall           func(func()) error
	window           func() *woxui.Window
	dictationAudio   *dictationPreviewAudioState
	dictationCompare *dictationModelCompareState
}

// bindDictationPreview supplies UI dispatch without retaining the application.
func (a *App) bindDictationPreview() {
	a.dictation.lifecycleCtx = a.lifecycleCtx
	a.dictation.runOnUI = a.runOnUI
	a.dictation.uiCall = func(fn func()) error { return a.runOnUI("update dictation preview", fn) }
	a.dictation.window = func() *woxui.Window { return a.window }
}

// dispose invalidates pending audio and model-comparison completions before teardown.
func (a *dictationPreviewController) dispose() {
	a.deactivateDictationAudio()
	if a.dictationCompare != nil && a.dictationCompare.cancel != nil {
		a.dictationCompare.cancel()
	}
	a.dictationCompare = nil
}
