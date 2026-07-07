package imageoverlay

import "wox/util/overlay"

func newImageRenderer(id string, source overlayImage, width float64, height float64, cornerRadius float64, closable bool) (*imageRenderer, bool) {
	_, _, _, _, _, _ = id, source, width, height, cornerRadius, closable
	return nil, false
}

func (renderer *imageRenderer) nativeAttachment() overlay.NativeAttachment {
	return overlay.NativeAttachment{}
}

func (renderer *imageRenderer) destroy() {}
