package iconoverlay

import "wox/util/overlay"

func newIconRenderer(opts Options) (*iconRenderer, bool) {
	_ = opts
	return nil, false
}

func (renderer *iconRenderer) nativeAttachment() overlay.NativeAttachment {
	return overlay.NativeAttachment{}
}

func (renderer *iconRenderer) destroy() {}
