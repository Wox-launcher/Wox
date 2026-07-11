package dictationoverlay

import "wox/util/overlay"

func newDictationOverlayRenderer(id string, closable bool) (*dictationOverlayRenderer, bool) {
	_, _ = id, closable
	return nil, false
}

func (renderer *dictationOverlayRenderer) nativeAttachment() overlay.NativeAttachment {
	return overlay.NativeAttachment{}
}

func (renderer *dictationOverlayRenderer) setActive(active bool) {
}

func (renderer *dictationOverlayRenderer) destroy() {
}
