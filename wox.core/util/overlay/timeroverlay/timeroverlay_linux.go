package timeroverlay

import "wox/util/overlay"

func newTimerRenderer(opts Options) (*timerRenderer, bool) {
	_ = opts
	return nil, false
}

func (renderer *timerRenderer) nativeAttachment() overlay.NativeAttachment {
	return overlay.NativeAttachment{}
}

func (renderer *timerRenderer) destroy() {}
