package textoverlay

import "wox/util/overlay"

func newTextRenderer(opts Options) (*textRenderer, bool) {
	_ = opts
	return nil, false
}

func (renderer *textRenderer) nativeAttachment() overlay.NativeAttachment {
	return overlay.NativeAttachment{}
}

func (renderer *textRenderer) destroy() {}
