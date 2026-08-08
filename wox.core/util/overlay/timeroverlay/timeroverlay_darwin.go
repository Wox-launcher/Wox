package timeroverlay

import "wox/util/overlay"

// Darwin timer overlay is stubbed for now; Windows owns the specialized countdown UI.
func newTimerRenderer(opts Options) (*timerRenderer, bool) {
	_ = opts
	return nil, false
}

func (renderer *timerRenderer) nativeAttachment() overlay.NativeAttachment {
	return overlay.NativeAttachment{}
}

func (renderer *timerRenderer) destroy() {}
